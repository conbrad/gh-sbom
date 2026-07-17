package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "sboms")
	tsv := filepath.Join(dir, "combined.tsv")
	factory := func() (*api.RESTClient, error) { return handlerClient(t, orgMux(t)), nil }

	var stdout, stderr bytes.Buffer
	code := run([]string{"acme", "-o", outDir, "-t", tsv, "--top", "2"}, &stdout, &stderr, factory)
	if code != 0 {
		t.Fatalf("code = %d, stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Top 2 packages by repo count:") {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if _, err := os.Stat(tsv); err != nil {
		t.Fatalf("tsv not written: %v", err)
	}

	// Re-aggregate offline from the files the first run produced.
	stdout.Reset()
	code = run([]string{"--skip-fetch", "-o", outDir, "-t", tsv}, &stdout, &stderr, nil)
	if code != 0 || !strings.Contains(stdout.String(), "unique packages") {
		t.Fatalf("skip-fetch: code = %d, stdout = %s", code, stdout.String())
	}
}

func TestRunFailures(t *testing.T) {
	var stdout, stderr bytes.Buffer
	okFactory := func() (*api.RESTClient, error) { return handlerClient(t, orgMux(t)), nil }

	if code := run([]string{"--help"}, &stdout, &stderr, nil); code != 0 {
		t.Fatalf("help code = %d", code)
	}
	if code := run(nil, &stdout, &stderr, nil); code != 1 {
		t.Fatalf("usage code = %d", code)
	}
	if code := run([]string{"--bogus"}, &stdout, &stderr, nil); code != 1 {
		t.Fatalf("bad flag code = %d", code)
	}
	if code := run([]string{"acme"}, &stdout, &stderr, func() (*api.RESTClient, error) {
		return nil, errors.New("no auth")
	}); code != 1 || !strings.Contains(stderr.String(), "gh auth login") {
		t.Fatalf("client error not reported: %s", stderr.String())
	}
	if code := run([]string{"ghost", "-o", t.TempDir()}, &stdout, &stderr, func() (*api.RESTClient, error) {
		return errClient(t), nil
	}); code != 1 {
		t.Fatal("fetch failure should exit 1")
	}
	if code := run([]string{"--skip-fetch", "-o", t.TempDir()}, &stdout, &stderr, okFactory); code != 1 {
		t.Fatal("aggregate failure should exit 1")
	}
}

func TestMainFunc(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	defer func() { os.Args, exit = oldArgs, oldExit }()

	code := -1
	exit = func(c int) { code = c }
	os.Args = []string{"gh-sbom", "--version"}
	main()
	if code != 0 {
		t.Fatalf("main exit code = %d", code)
	}
}

func TestDefaultClient(t *testing.T) {
	// Exercises the constructor; success depends on the environment's gh auth,
	// so only panics/hangs would fail this.
	_, _ = defaultClient()
}
