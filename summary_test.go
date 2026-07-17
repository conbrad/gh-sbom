package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSummarize(t *testing.T) {
	rows := []row{
		{"r1", "npm", "lodash", "4.17.21"},
		{"r2", "npm", "lodash", "4.17.20"},
		{"r2", "npm", "lodash", "4.17.21"}, // same repo+pkg twice: counts once
		{"r1", "npm", "axios", "1.0.0"},
		{"r2", "npm", "chalk", "5.0.0"},
	}
	var stdout bytes.Buffer
	summarize(rows, &options{tsvFile: "combined.tsv", top: 2}, &stdout)

	out := stdout.String()
	if !strings.Contains(out, "5 dependency entries, 3 unique packages across 2 repos") {
		t.Fatalf("summary line missing in:\n%s", out)
	}
	// lodash spans 2 repos; axios and chalk tie at 1 and sort alphabetically,
	// truncated to top 2.
	if !strings.Contains(out, "2  lodash") || !strings.Contains(out, "1  axios") ||
		strings.Contains(out, "chalk") {
		t.Fatalf("rollup wrong:\n%s", out)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	var stdout bytes.Buffer
	summarize(nil, &options{tsvFile: "x.tsv", top: 20}, &stdout)
	if !strings.Contains(stdout.String(), "0 dependency entries") {
		t.Fatalf("out = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "Top") {
		t.Fatalf("rollup printed for empty rows:\n%s", stdout.String())
	}
}
