package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type row struct {
	repo, ecosystem, pkg, version string
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
// directory. The repo's own root package (the one the SPDX document
// DESCRIBES) is excluded so rollups only count real dependencies.
// JSON files that don't parse as SBOMs — such as the tool's own
// --format json output when --out points inside the output directory —
// are skipped with a warning rather than aborting the run.
func aggregate(opts *options, stderr io.Writer) ([]row, error) {
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
			fmt.Fprintf(stderr, "warning: skipping %s: not a valid SBOM (%v)\n", f, err)
			continue
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

	return rows, nil
}
