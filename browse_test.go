package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBrowseCmdNoData(t *testing.T) {
	code, _, stderr := execRun(t, "browse", "-o", t.TempDir())
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "no SBOM JSON files") {
		t.Fatalf("stderr = %s", stderr)
	}
}

func TestBrowseCmdLaunchesWithLoadedRows(t *testing.T) {
	dir := writeSBOMDir(t, map[string]string{"acme/app.json": goodSBOM})

	var gotRows []row
	old := runBrowseProgram
	runBrowseProgram = func(rows []row, opts ...tea.ProgramOption) error {
		gotRows = rows
		return nil
	}
	defer func() { runBrowseProgram = old }()

	code, _, stderr := execRun(t, "browse", "-o", dir)
	if code != 0 {
		t.Fatalf("code = %d, stderr:\n%s", code, stderr)
	}
	if len(gotRows) == 0 {
		t.Fatal("runBrowseProgram was not called with any rows")
	}
}

func TestBrowseCmdProgramErrorPropagates(t *testing.T) {
	dir := writeSBOMDir(t, map[string]string{"acme/app.json": goodSBOM})

	old := runBrowseProgram
	runBrowseProgram = func(rows []row, opts ...tea.ProgramOption) error { return errors.New("boom") }
	defer func() { runBrowseProgram = old }()

	code, _, stderr := execRun(t, "browse", "-o", dir)
	if code != 1 || !strings.Contains(stderr, "boom") {
		t.Fatalf("code = %d, stderr = %s", code, stderr)
	}
}

func TestBrowseCmdDefaultOutputFlag(t *testing.T) {
	cmd := newBrowseCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil || f.DefValue != "sboms" {
		t.Fatalf("output flag = %+v, want default \"sboms\"", f)
	}
}

func TestBrowseCmdRejectsArgs(t *testing.T) {
	code, _, stderr := execRun(t, "browse", "extra-arg")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown command") && !strings.Contains(stderr, "arg") {
		t.Fatalf("stderr = %s, want an args-rejection message", stderr)
	}
}

// The two tests below call the real (unswapped) runBrowseProgram directly,
// with safe non-terminal I/O, so its actual body is genuinely exercised
// rather than only ever stubbed. See the task's coverage note above for why
// these specific I/O choices are the ones that work.

func TestRunBrowseProgramQuits(t *testing.T) {
	rows := []row{{"acme/app", "npm", "lodash", "4.17.21"}}
	err := runBrowseProgram(rows, tea.WithInput(strings.NewReader("q")), tea.WithOutput(io.Discard))
	if err != nil {
		t.Fatalf("runBrowseProgram: %v", err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestRunBrowseProgramInputError(t *testing.T) {
	rows := []row{{"acme/app", "npm", "lodash", "4.17.21"}}
	err := runBrowseProgram(rows, tea.WithInput(errReader{}), tea.WithOutput(io.Discard))
	if err == nil || !strings.Contains(err.Error(), "browse:") {
		t.Fatalf("err = %v, want wrapped browse error", err)
	}
}
