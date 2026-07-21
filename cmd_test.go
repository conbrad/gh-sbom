package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

func execRun(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	factory := func() (*api.RESTClient, error) { return handlerClient(t, orgMux(t)), nil }
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, factory)
	return code, out.String(), errOut.String()
}

func TestCmdHelp(t *testing.T) {
	code, stdout, _ := execRun(t, "--help")
	if code != 0 {
		t.Fatalf("help code = %d", code)
	}
	for _, want := range []string{"Usage:", "sbom <org> | <owner>/<repo>", "--skip-fetch", "--include-archived", "Examples:", "html"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q:\n%s", want, stdout)
		}
	}
}

func TestCmdVersion(t *testing.T) {
	code, stdout, _ := execRun(t, "--version")
	if code != 0 || stdout != "gh-sbom "+version+"\n" {
		t.Fatalf("code = %d, stdout = %q", code, stdout)
	}
}

func TestCmdArgValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no target", nil, "a target <org> or <owner>/<repo> is required"},
		{"too many args", []string{"acme", "other"}, "unexpected argument: other"},
		{"unknown flag", []string{"acme", "--bogus"}, "unknown flag: --bogus"},
		{"non-numeric limit", []string{"acme", "--limit", "abc"}, "invalid argument"},
		{"negative limit", []string{"acme", "--limit=-1"}, "--limit must be non-negative, got -1"},
		{"negative top", []string{"acme", "--top=-2"}, "--top must be non-negative, got -2"},
		{"invalid format", []string{"acme", "--format", "yaml"}, `invalid format "yaml" (valid: tsv, csv, json, html, parquet)`},
		{"empty out", []string{"acme", "--out", ""}, "--out requires a non-empty path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := execRun(t, tc.args...)
			if code != 1 {
				t.Fatalf("code = %d, want 1", code)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("stderr missing %q:\n%s", tc.wantErr, stderr)
			}
		})
	}
}

func TestCmdDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	code, stdout, _ := execRun(t, "acme")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	// Default output locations are relative to the working directory.
	if !strings.Contains(stdout, "Wrote combined.tsv:") {
		t.Fatalf("default tsv path not used:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Top 20 packages") {
		t.Fatalf("default top not used:\n%s", stdout)
	}
}

func TestCmdFormatDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	code, stdout, _ := execRun(t, "acme", "-f", "json")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	// Default out path follows the format.
	if !strings.Contains(stdout, "Wrote combined.json:") {
		t.Fatalf("default out path wrong:\n%s", stdout)
	}
	if _, err := os.Stat("combined.json"); err != nil {
		t.Fatalf("combined.json not written: %v", err)
	}
}

func TestCmdHTMLFormat(t *testing.T) {
	t.Chdir(t.TempDir())
	code, stdout, _ := execRun(t, "acme", "-f", "html")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stdout, "Wrote combined.html:") {
		t.Fatalf("default html out path wrong:\n%s", stdout)
	}
	data, err := os.ReadFile("combined.html")
	if err != nil {
		t.Fatalf("combined.html not written: %v", err)
	}
	if !strings.Contains(string(data), "<!DOCTYPE html>") || !strings.Contains(string(data), "<table>") {
		t.Fatalf("combined.html missing expected markup: %q", data)
	}
}

func TestCmdFormatCaseInsensitive(t *testing.T) {
	t.Chdir(t.TempDir())
	code, stdout, _ := execRun(t, "acme", "--format", "JSON")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stdout, "Wrote combined.json:") {
		t.Fatalf("uppercase format not normalized:\n%s", stdout)
	}
}

func TestCmdReaggregateIgnoresOwnOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	out := filepath.Join("sboms", "combined.json")
	// First run writes its JSON table inside the SBOM output dir...
	if code, _, stderr := execRun(t, "acme", "-o", "sboms", "-f", "json", "--out", out); code != 0 {
		t.Fatalf("first run code = %d, stderr:\n%s", code, stderr)
	}
	// ...and re-aggregation must skip that file rather than abort.
	code, stdout, stderr := execRun(t, "--skip-fetch", "-o", "sboms", "-f", "json", "--out", out)
	if code != 0 {
		t.Fatalf("second run code = %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "warning: skipping") {
		t.Fatalf("expected skip warning:\n%s", stderr)
	}
	if !strings.Contains(stdout, "Wrote "+out+":") {
		t.Fatalf("stdout = %s", stdout)
	}
}

func TestCmdExplicitOut(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "deps.csv")
	code, stdout, _ := execRun(t, "acme", "-o", filepath.Join(dir, "sboms"), "--format", "csv", "--out", out)
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(stdout, "Wrote "+out+":") {
		t.Fatalf("explicit out path not reported:\n%s", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "repo,ecosystem,package,version\n") {
		t.Fatalf("csv header missing: %q", data)
	}
}
