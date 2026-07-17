package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    options
		wantErr string
	}{
		{
			name: "defaults with target",
			args: []string{"acme"},
			want: options{target: "acme", outDir: "sboms", tsvFile: "combined.tsv", limit: 1000, top: 20},
		},
		{
			name: "all flags",
			args: []string{"acme", "-o", "out", "--tsv", "x.tsv", "--limit", "5", "-n", "3", "--include-archived", "--skip-fetch"},
			want: options{target: "acme", outDir: "out", tsvFile: "x.tsv", limit: 5, top: 3, includeArchived: true, skipFetch: true},
		},
		{
			name: "skip-fetch without target",
			args: []string{"--skip-fetch"},
			want: options{outDir: "sboms", tsvFile: "combined.tsv", limit: 1000, top: 20, skipFetch: true},
		},
		{name: "missing output value", args: []string{"-o"}, wantErr: "-o requires a value"},
		{name: "missing tsv value", args: []string{"acme", "-t"}, wantErr: "-t requires a value"},
		{name: "missing limit value", args: []string{"acme", "-l"}, wantErr: "-l requires a value"},
		{name: "missing top value", args: []string{"acme", "-n"}, wantErr: "-n requires a value"},
		{name: "bad limit", args: []string{"acme", "--limit", "abc"}, wantErr: `--limit requires a non-negative number, got "abc"`},
		{name: "negative top", args: []string{"acme", "--top", "-1"}, wantErr: `--top requires a non-negative number, got "-1"`},
		{name: "unknown flag", args: []string{"--bogus"}, wantErr: "unknown flag: --bogus"},
		{name: "extra argument", args: []string{"acme", "other"}, wantErr: "unexpected argument: other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			opts, err := parseArgs(tc.args, &stdout, &stderr)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *opts != tc.want {
				t.Fatalf("opts = %+v, want %+v", *opts, tc.want)
			}
		})
	}
}

func TestParseArgsHelpVersionUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if _, err := parseArgs([]string{"--help"}, &stdout, &stderr); !errors.Is(err, errHelp) {
		t.Fatalf("help err = %v, want errHelp", err)
	}
	if !strings.Contains(stdout.String(), "USAGE") {
		t.Fatalf("help output missing usage: %q", stdout.String())
	}

	stdout.Reset()
	if _, err := parseArgs([]string{"-v"}, &stdout, &stderr); !errors.Is(err, errHelp) {
		t.Fatalf("version err = %v, want errHelp", err)
	}
	if !strings.Contains(stdout.String(), "gh-sbom "+version) {
		t.Fatalf("version output = %q", stdout.String())
	}

	if _, err := parseArgs(nil, &stdout, &stderr); !errors.Is(err, errUsage) {
		t.Fatalf("no-args err = %v, want errUsage", err)
	}
	if !strings.Contains(stderr.String(), "USAGE") {
		t.Fatalf("usage not printed to stderr: %q", stderr.String())
	}
}
