package main

import (
	"bytes"
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
	for _, want := range []string{"Usage:", "sbom <org> | <owner>/<repo>", "--skip-fetch", "--include-archived", "Examples:"} {
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
