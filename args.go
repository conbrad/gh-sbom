package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// Sentinel results from parseArgs: the message has already been printed and
// only the exit code remains to be decided.
var (
	errHelp  = errors.New("help requested")
	errUsage = errors.New("usage shown")
)

type options struct {
	target          string
	outDir          string
	tsvFile         string
	limit           int
	top             int
	includeArchived bool
	skipFetch       bool
}

func parseArgs(args []string, stdout, stderr io.Writer) (*options, error) {
	opts := &options{outDir: "sboms", tsvFile: "combined.tsv", limit: 1000, top: 20}

	intVal := func(name, v string) (int, error) {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
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
			fmt.Fprint(stdout, usage)
			return nil, errHelp
		case "-v", "--version":
			fmt.Fprintln(stdout, "gh-sbom "+version)
			return nil, errHelp
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
		fmt.Fprint(stderr, usage)
		return nil, errUsage
	}
	return opts, nil
}
