package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var formatRows = []row{
	{"cli", "golang", "github.com/x/y", "v1.0.0"},
	{"web", "npm", "left,pad", "1.0"},
}

func writeAndRead(t *testing.T, format string, rows []row) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out."+format)
	if err := writeRows(path, format, rows); err != nil {
		t.Fatalf("writeRows(%s): %v", format, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWriteRowsTSV(t *testing.T) {
	want := "repo\tecosystem\tpackage\tversion\n" +
		"cli\tgolang\tgithub.com/x/y\tv1.0.0\n" +
		"web\tnpm\tleft,pad\t1.0\n"
	if got := writeAndRead(t, "tsv", formatRows); got != want {
		t.Fatalf("tsv = %q, want %q", got, want)
	}
}

func TestWriteRowsCSV(t *testing.T) {
	// The comma-containing package name must be quoted.
	want := "repo,ecosystem,package,version\n" +
		"cli,golang,github.com/x/y,v1.0.0\n" +
		"web,npm,\"left,pad\",1.0\n"
	if got := writeAndRead(t, "csv", formatRows); got != want {
		t.Fatalf("csv = %q, want %q", got, want)
	}
}

func TestWriteRowsJSON(t *testing.T) {
	want := `[
  {
    "repo": "cli",
    "ecosystem": "golang",
    "package": "github.com/x/y",
    "version": "v1.0.0"
  },
  {
    "repo": "web",
    "ecosystem": "npm",
    "package": "left,pad",
    "version": "1.0"
  }
]
`
	if got := writeAndRead(t, "json", formatRows); got != want {
		t.Fatalf("json = %q, want %q", got, want)
	}
}

func TestWriteRowsJSONEmpty(t *testing.T) {
	// Zero rows must serialize as [], never null.
	if got := writeAndRead(t, "json", nil); got != "[]\n" {
		t.Fatalf("empty json = %q, want %q", got, "[]\n")
	}
}

func TestWriteRowsInvalidFormat(t *testing.T) {
	err := writeRows(filepath.Join(t.TempDir(), "x"), "yaml", formatRows)
	if err == nil || !strings.Contains(err.Error(), `invalid format "yaml" (valid: tsv, csv, json)`) {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteRowsCreateError(t *testing.T) {
	for _, format := range []string{"tsv", "csv", "json"} {
		path := filepath.Join(t.TempDir(), "no", "such", "dir", "out")
		if err := writeRows(path, format, formatRows); err == nil {
			t.Fatalf("%s: expected error for unwritable path", format)
		}
	}
}

func TestIsValidFormat(t *testing.T) {
	for _, f := range validFormats {
		if !isValidFormat(f) {
			t.Errorf("isValidFormat(%q) = false", f)
		}
	}
	if isValidFormat("yaml") {
		t.Error(`isValidFormat("yaml") = true`)
	}
}
