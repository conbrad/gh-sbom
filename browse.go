package main

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// Swapped out in cobra-level tests (browse_test.go) so RunE doesn't seize a
// real terminal. The variadic opts let a direct test supply safe,
// non-terminal I/O to genuinely execute (not stub) this function's body --
// see TestRunBrowseProgramQuits / TestRunBrowseProgramInputError.
var runBrowseProgram = func(rows []row, opts ...tea.ProgramOption) error {
	_, err := tea.NewProgram(newBrowseModel(rows), opts...).Run()
	if err != nil {
		return fmt.Errorf("browse: %w", err)
	}
	return nil
}

func newBrowseCmd() *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Interactively browse a prior run's aggregated dependency table",
		Long: `Opens a terminal UI over the dependency table from a prior run -- the
same data --skip-fetch re-aggregates, read directly from the SBOM output
directory. No network calls.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := aggregate(&options{outDir: outDir}, io.Discard)
			if err != nil {
				return err
			}
			return runBrowseProgram(rows, tea.WithAltScreen())
		},
	}
	cmd.Flags().StringVarP(&outDir, "output", "o", "sboms", "directory of raw SBOM JSON files to browse")
	return cmd
}
