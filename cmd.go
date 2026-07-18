package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

const longHelp = `Export and aggregate SBOMs from GitHub's dependency graph.

Uses GitHub's native SBOM endpoint (GET /repos/{owner}/{repo}/dependency-graph/sbom),
so there is no cloning and no local scanning -- one REST call per repo.

Pass an org (or user) to fetch SBOMs for every repo, or <owner>/<repo> for a
single repo. Raw SPDX JSON is saved per repo in the output directory, and a
combined TSV (columns: repo, ecosystem, package, version) is written alongside
a "most common packages" rollup.`

const example = `  gh sbom my-org
  gh sbom cli/cli
  gh sbom my-org --top 50
  gh sbom --skip-fetch  # re-aggregate previously downloaded SBOMs`

type options struct {
	target          string
	outDir          string
	outFile         string
	format          string
	limit           int
	top             int
	includeArchived bool
	skipFetch       bool
}

func newRootCmd(newClient clientFactory) *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:          "sbom <org> | <owner>/<repo>",
		Short:        "Export and aggregate SBOMs from GitHub's dependency graph",
		Long:         longHelp,
		Example:      example,
		Version:      version,
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("unexpected argument: %s", args[1])
			}
			if len(args) == 0 && !opts.skipFetch {
				return errors.New("a target <org> or <owner>/<repo> is required unless --skip-fetch is set")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isValidFormat(opts.format) {
				return invalidFormatErr(opts.format)
			}
			if opts.outFile == "" {
				opts.outFile = "combined." + opts.format
			}
			if opts.limit < 0 {
				return fmt.Errorf("--limit must be non-negative, got %d", opts.limit)
			}
			if opts.top < 0 {
				return fmt.Errorf("--top must be non-negative, got %d", opts.top)
			}
			if len(args) == 1 {
				opts.target = args[0]
			}

			if !opts.skipFetch {
				client, err := newClient()
				if err != nil {
					return fmt.Errorf("%w (try `gh auth login`)", err)
				}
				if err := fetchSBOMs(client, opts, cmd.ErrOrStderr()); err != nil {
					return err
				}
			}

			rows, err := aggregate(opts)
			if err != nil {
				return err
			}
			if err := writeRows(opts.outFile, opts.format, rows); err != nil {
				return err
			}
			summarize(rows, opts, cmd.OutOrStdout())
			return nil
		},
	}
	cmd.SetVersionTemplate("gh-sbom {{.Version}}\n")

	f := cmd.Flags()
	f.StringVarP(&opts.outDir, "output", "o", "sboms", "directory for raw SBOM JSON files")
	f.StringVarP(&opts.format, "format", "f", "tsv", "output format for the combined table: tsv, csv, or json")
	f.StringVar(&opts.outFile, "out", "", `combined table output path (default "combined.<format>")`)
	f.IntVarP(&opts.limit, "limit", "l", 1000, "max repos to list from the org")
	f.IntVarP(&opts.top, "top", "n", 20, `rows in the "most common packages" rollup`)
	f.BoolVar(&opts.includeArchived, "include-archived", false, "include archived repos (skipped by default)")
	f.BoolVar(&opts.skipFetch, "skip-fetch", false, "re-aggregate existing JSON in the output dir; no API calls")
	return cmd
}
