package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

const version = "0.1.0"

const usage = `gh-sbom: export and aggregate SBOMs from GitHub's dependency graph

Uses GitHub's native SBOM endpoint (GET /repos/{owner}/{repo}/dependency-graph/sbom),
so there is no cloning and no local scanning -- one REST call per repo.

USAGE
  gh sbom <org> [flags]           Fetch SBOMs for every repo in an org
  gh sbom <owner>/<repo> [flags]  Fetch the SBOM for a single repo

FLAGS
  -o, --output <dir>     Directory for raw SBOM JSON files (default: sboms)
  -t, --tsv <file>       Combined TSV output path (default: combined.tsv)
  -l, --limit <n>        Max repos to list from the org (default: 1000)
  -n, --top <n>          Rows in the "most common packages" rollup (default: 20)
      --include-archived Include archived repos (skipped by default)
      --skip-fetch       Re-aggregate existing JSON in the output dir; no API calls
  -v, --version          Print version
  -h, --help             Show this help

OUTPUT
  <output>/<repo>.json   Raw SPDX SBOM per repo
  <tsv>                  Columns: repo, ecosystem, package, version

EXAMPLES
  gh sbom my-org
  gh sbom cli/cli
  gh sbom my-org --top 50
`

type options struct {
	target          string
	outDir          string
	tsvFile         string
	limit           int
	top             int
	includeArchived bool
	skipFetch       bool
}

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

type row struct {
	repo, ecosystem, pkg, version string
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !opts.skipFetch {
		client, err := api.DefaultRESTClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v (try `gh auth login`)\n", err)
			os.Exit(1)
		}
		if err := fetchSBOMs(client, opts); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	rows, err := aggregate(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	summarize(rows, opts)
}

func parseArgs(args []string) (*options, error) {
	opts := &options{outDir: "sboms", tsvFile: "combined.tsv", limit: 1000, top: 20}

	intVal := func(name, v string) (int, error) {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n < 0 {
			return 0, fmt.Errorf("%s requires a non-negative number, got %q", name, v)
		}
		return n, nil
	}
	strArg := func(name string, i *int) (string, error) {
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("%s requires a value", name)
		}
		return args[*i], nil
	}

	var err error
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		case "-v", "--version":
			fmt.Println("gh-sbom " + version)
			os.Exit(0)
		case "-o", "--output":
			if opts.outDir, err = strArg(arg, &i); err != nil {
				return nil, err
			}
		case "-t", "--tsv":
			if opts.tsvFile, err = strArg(arg, &i); err != nil {
				return nil, err
			}
		case "-l", "--limit":
			v, err := strArg(arg, &i)
			if err != nil {
				return nil, err
			}
			if opts.limit, err = intVal(arg, v); err != nil {
				return nil, err
			}
		case "-n", "--top":
			v, err := strArg(arg, &i)
			if err != nil {
				return nil, err
			}
			if opts.top, err = intVal(arg, v); err != nil {
				return nil, err
			}
		case "--include-archived":
			opts.includeArchived = true
		case "--skip-fetch":
			opts.skipFetch = true
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown flag: %s (see gh sbom --help)", arg)
			}
			if opts.target != "" {
				return nil, fmt.Errorf("unexpected argument: %s", arg)
			}
			opts.target = arg
		}
	}

	if opts.target == "" && !opts.skipFetch {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
	return opts, nil
}

// listRepos returns repo names for an org or user account, trying the org
// endpoint first. Archived repos are skipped unless included.
func listRepos(client *api.RESTClient, owner string, opts *options) ([]string, error) {
	bases := []string{"orgs/" + owner + "/repos", "users/" + owner + "/repos"}

	for _, base := range bases {
		names := []string{}
		notAnOrg := false
		for page := 1; len(names) < opts.limit; page++ {
			var repos []struct {
				Name     string `json:"name"`
				Archived bool   `json:"archived"`
			}
			path := fmt.Sprintf("%s?per_page=100&page=%d", base, page)
			if err := client.Get(path, &repos); err != nil {
				var httpErr *api.HTTPError
				if errors.As(err, &httpErr) && httpErr.StatusCode == 404 && base == bases[0] {
					notAnOrg = true // fall through to the users endpoint
					break
				}
				return nil, fmt.Errorf("could not list repos for %q: %w", owner, err)
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
		if !notAnOrg {
			return names, nil
		}
	}
	return nil, fmt.Errorf("no repos found for %q", owner)
}

func fetchSBOMs(client *api.RESTClient, opts *options) error {
	var owner string
	var repos []string

	if idx := strings.Index(opts.target, "/"); idx >= 0 {
		owner = opts.target[:idx]
		repos = []string{opts.target[idx+1:]}
	} else {
		owner = opts.target
		fmt.Fprintf(os.Stderr, "Listing repos for %s...\n", owner)
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
		fmt.Fprintf(os.Stderr, "warning: only %d API requests remaining this hour for %d repos\n",
			rate.Resources.Core.Remaining, len(repos))
	}

	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return err
	}

	var ok, empty int
	var failed []string
	for i, repo := range repos {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s/%s\n", i+1, len(repos), owner, repo)

		var doc json.RawMessage
		if err := client.Get("repos/"+owner+"/"+repo+"/dependency-graph/sbom", &doc); err != nil {
			failed = append(failed, repo)
			fmt.Fprintln(os.Stderr, "  failed (dependency graph disabled, or no access)")
			continue
		}
		if err := os.WriteFile(filepath.Join(opts.outDir, repo+".json"), doc, 0o644); err != nil {
			return err
		}
		ok++

		var parsed sbomDoc
		if json.Unmarshal(doc, &parsed) == nil && len(parsed.SBOM.Packages) <= 1 {
			empty++
			fmt.Fprintln(os.Stderr, "  note: no dependencies detected (dependency graph disabled, or no supported manifests)")
		}
		time.Sleep(100 * time.Millisecond) // be nice to rate limits on large orgs
	}

	fmt.Fprintf(os.Stderr, "\nFetched %d/%d SBOMs into %s/ (%d with no dependencies)\n",
		ok, len(repos), opts.outDir, empty)
	if len(failed) > 0 {
		fmt.Fprintf(os.Stderr, "Failed (%d): %s\n", len(failed), strings.Join(failed, ", "))
		fmt.Fprintln(os.Stderr, "For private repos, an admin may need to enable the dependency graph under")
		fmt.Fprintln(os.Stderr, "Settings -> Advanced Security -> Dependency graph.")
	}
	return nil
}

// ecosystemOf derives the ecosystem from a purl, e.g. "pkg:golang/..." -> "golang".
func ecosystemOf(purl string) string {
	rest, found := strings.CutPrefix(purl, "pkg:")
	if !found {
		return "unknown"
	}
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		return rest[:idx]
	}
	return rest
}

// aggregate extracts one row per dependency from every SBOM in the output
// directory and writes the combined TSV. The repo's own root package (the one
// the SPDX document DESCRIBES) is excluded so rollups only count real
// dependencies.
func aggregate(opts *options) ([]row, error) {
	files, err := filepath.Glob(filepath.Join(opts.outDir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no SBOM JSON files in %s/", opts.outDir)
	}
	sort.Strings(files)

	var rows []row
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc sbomDoc
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}

		repo := strings.TrimSuffix(filepath.Base(f), ".json")
		roots := map[string]bool{}
		for _, rel := range doc.SBOM.Relationships {
			if rel.RelationshipType == "DESCRIBES" {
				roots[rel.RelatedSpdxElement] = true
			}
		}
		for _, pkg := range doc.SBOM.Packages {
			if pkg.Name == "" || roots[pkg.SPDXID] {
				continue
			}
			purl := ""
			for _, ref := range pkg.ExternalRefs {
				if ref.ReferenceType == "purl" {
					purl = ref.ReferenceLocator
					break
				}
			}
			ver := pkg.VersionInfo
			if ver == "" {
				ver = "unknown"
			}
			rows = append(rows, row{repo, ecosystemOf(purl), pkg.Name, ver})
		}
	}

	var b strings.Builder
	b.WriteString("repo\tecosystem\tpackage\tversion\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", r.repo, r.ecosystem, r.pkg, r.version)
	}
	if err := os.WriteFile(opts.tsvFile, []byte(b.String()), 0o644); err != nil {
		return nil, err
	}
	return rows, nil
}

func summarize(rows []row, opts *options) {
	repos := map[string]bool{}
	uniquePkgs := map[string]bool{}
	repoPkg := map[string]map[string]bool{}
	for _, r := range rows {
		repos[r.repo] = true
		uniquePkgs[r.pkg] = true
		if repoPkg[r.pkg] == nil {
			repoPkg[r.pkg] = map[string]bool{}
		}
		repoPkg[r.pkg][r.repo] = true
	}

	fmt.Printf("\nWrote %s: %d dependency entries, %d unique packages across %d repos\n",
		opts.tsvFile, len(rows), len(uniquePkgs), len(repos))

	if len(rows) == 0 || opts.top <= 0 {
		return
	}
	type count struct {
		pkg string
		n   int
	}
	counts := make([]count, 0, len(repoPkg))
	for pkg, rs := range repoPkg {
		counts = append(counts, count{pkg, len(rs)})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].n != counts[j].n {
			return counts[i].n > counts[j].n
		}
		return counts[i].pkg < counts[j].pkg
	})
	if len(counts) > opts.top {
		counts = counts[:opts.top]
	}
	fmt.Printf("\nTop %d packages by repo count:\n", opts.top)
	for _, c := range counts {
		fmt.Printf("  %4d  %s\n", c.n, c.pkg)
	}
}
