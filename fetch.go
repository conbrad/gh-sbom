package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Swapped out in tests.
var sleepBetween = 100 * time.Millisecond

type sbomDoc struct {
	SBOM struct {
		Packages []struct {
			SPDXID       string `json:"SPDXID"`
			Name         string `json:"name"`
			VersionInfo  string `json:"versionInfo"`
			ExternalRefs []struct {
				ReferenceType    string `json:"referenceType"`
				ReferenceLocator string `json:"referenceLocator"`
			} `json:"externalRefs"`
		} `json:"packages"`
		Relationships []struct {
			RelationshipType   string `json:"relationshipType"`
			RelatedSpdxElement string `json:"relatedSpdxElement"`
		} `json:"relationships"`
	} `json:"sbom"`
}

// listRepos returns repo names for an org or user account, trying the org
// endpoint first. Archived repos are skipped unless included.
func listRepos(client *api.RESTClient, owner string, opts *options) ([]string, error) {
	names, err := listReposFrom(client, "orgs/"+owner+"/repos", opts)
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == 404 {
		// Not an org; fall back to the user endpoint.
		names, err = listReposFrom(client, "users/"+owner+"/repos", opts)
	}
	if err != nil {
		return nil, fmt.Errorf("could not list repos for %q: %w", owner, err)
	}
	return names, nil
}

func listReposFrom(client *api.RESTClient, base string, opts *options) ([]string, error) {
	names := []string{}
	for page := 1; len(names) < opts.limit; page++ {
		var repos []struct {
			Name     string `json:"name"`
			Archived bool   `json:"archived"`
		}
		path := fmt.Sprintf("%s?per_page=100&page=%d", base, page)
		if err := client.Get(path, &repos); err != nil {
			return nil, err
		}
		for _, r := range repos {
			if !opts.includeArchived && r.Archived {
				continue
			}
			if len(names) < opts.limit {
				names = append(names, r.Name)
			}
		}
		if len(repos) < 100 {
			break
		}
	}
	return names, nil
}

func fetchSBOMs(client *api.RESTClient, opts *options, stderr io.Writer) error {
	var owner string
	var repos []string

	if idx := strings.Index(opts.target, "/"); idx >= 0 {
		owner = opts.target[:idx]
		repos = []string{opts.target[idx+1:]}
	} else {
		owner = opts.target
		fmt.Fprintf(stderr, "Listing repos for %s...\n", owner)
		var err error
		if repos, err = listRepos(client, owner, opts); err != nil {
			return err
		}
		if len(repos) == 0 {
			return fmt.Errorf("no repos found for %q", owner)
		}
	}

	var rate struct {
		Resources struct {
			Core struct {
				Remaining int `json:"remaining"`
			} `json:"core"`
		} `json:"resources"`
	}
	if err := client.Get("rate_limit", &rate); err == nil && rate.Resources.Core.Remaining < len(repos) {
		fmt.Fprintf(stderr, "warning: only %d API requests remaining this hour for %d repos\n",
			rate.Resources.Core.Remaining, len(repos))
	}

	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return err
	}

	var ok, empty int
	var failed []string
	for i, repo := range repos {
		fmt.Fprintf(stderr, "[%d/%d] %s/%s\n", i+1, len(repos), owner, repo)

		var doc json.RawMessage
		if err := client.Get("repos/"+owner+"/"+repo+"/dependency-graph/sbom", &doc); err != nil {
			failed = append(failed, repo)
			fmt.Fprintln(stderr, "  failed (dependency graph disabled, or no access)")
			continue
		}
		if err := os.WriteFile(filepath.Join(opts.outDir, repo+".json"), doc, 0o644); err != nil {
			return err
		}
		ok++

		var parsed sbomDoc
		if json.Unmarshal(doc, &parsed) == nil && len(parsed.SBOM.Packages) <= 1 {
			empty++
			fmt.Fprintln(stderr, "  note: no dependencies detected (dependency graph disabled, or no supported manifests)")
		}
		time.Sleep(sleepBetween) // be nice to rate limits on large orgs
	}

	fmt.Fprintf(stderr, "\nFetched %d/%d SBOMs into %s/ (%d with no dependencies)\n",
		ok, len(repos), opts.outDir, empty)
	if len(failed) > 0 {
		fmt.Fprintf(stderr, "Failed (%d): %s\n", len(failed), strings.Join(failed, ", "))
		fmt.Fprintln(stderr, "For private repos, an admin may need to enable the dependency graph under")
		fmt.Fprintln(stderr, "Settings -> Advanced Security -> Dependency graph.")
	}
	return nil
}
