package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEcosystemOf(t *testing.T) {
	cases := map[string]string{
		"pkg:npm/lodash@4.17.21":    "npm",
		"pkg:golang/github.com/x/y": "golang",
		"pkg:oddball":               "oddball",
		"not-a-purl":                "unknown",
		"":                          "unknown",
	}
	for purl, want := range cases {
		if got := ecosystemOf(purl); got != want {
			t.Errorf("ecosystemOf(%q) = %q, want %q", purl, got, want)
		}
	}
}

func TestAggregate(t *testing.T) {
	dir := writeSBOMDir(t, map[string]string{"app.json": goodSBOM, "empty.json": emptySBOM})

	rows, err := aggregate(&options{outDir: dir})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	// goodSBOM: root and nameless packages excluded, lodash + left-pad kept;
	// emptySBOM contributes nothing.
	want := []row{
		{"app", "npm", "lodash", "4.17.21"},
		{"app", "unknown", "left-pad", "unknown"},
	}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestAggregateErrors(t *testing.T) {
	// Malformed glob pattern.
	if _, err := aggregate(&options{outDir: "["}); !errors.Is(err, filepath.ErrBadPattern) {
		t.Fatalf("err = %v, want ErrBadPattern", err)
	}
	// No JSON files.
	if _, err := aggregate(&options{outDir: t.TempDir()}); err == nil ||
		!strings.Contains(err.Error(), "no SBOM JSON files") {
		t.Fatalf("err = %v", err)
	}
	// Unreadable file.
	dir := writeSBOMDir(t, map[string]string{"app.json": goodSBOM})
	if err := os.Chmod(filepath.Join(dir, "app.json"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := aggregate(&options{outDir: dir}); err == nil {
		t.Fatal("expected read error")
	}
	// Invalid JSON.
	dir = writeSBOMDir(t, map[string]string{"app.json": "{"})
	if _, err := aggregate(&options{outDir: dir}); err == nil ||
		!strings.Contains(err.Error(), "app.json") {
		t.Fatalf("err = %v", err)
	}
}
