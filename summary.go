package main

import (
	"fmt"
	"io"
	"sort"
)

func summarize(rows []row, opts *options, stdout io.Writer) {
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

	fmt.Fprintf(stdout, "\nWrote %s: %d dependency entries, %d unique packages across %d repos\n",
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
	fmt.Fprintf(stdout, "\nTop %d packages by repo count:\n", opts.top)
	for _, c := range counts {
		fmt.Fprintf(stdout, "  %4d  %s\n", c.n, c.pkg)
	}
}
