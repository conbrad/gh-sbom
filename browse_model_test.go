package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testRows() []row {
	return []row{
		{"acme/api", "npm", "lodash", "4.17.21"},
		{"acme/web", "npm", "lodash", "4.17.20"},
		{"other/service", "npm", "lodash", "4.17.21"},
		{"acme/api", "npm", "axios", "1.0.0"},
	}
}

func typeKey(m browseModel, s string) (tea.Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(browseModel)
	for _, r := range s {
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(browseModel)
	}
	return m, cmd
}

func TestNewBrowseModel(t *testing.T) {
	m := newBrowseModel(testRows())
	if m.view != viewList {
		t.Fatalf("view = %v, want viewList", m.view)
	}
	if len(m.filtered) != 4 {
		t.Fatalf("filtered = %d rows, want 4", len(m.filtered))
	}
	// Default sort is repo ascending: acme/api, acme/api, acme/web, other/service.
	want := []string{"acme/api", "acme/api", "acme/web", "other/service"}
	for i, r := range m.filtered {
		if r.repo != want[i] {
			t.Fatalf("filtered[%d].repo = %q, want %q", i, r.repo, want[i])
		}
	}
}

func TestBrowseModelFilter(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, _ := typeKey(m, "axios")
	m = updated.(browseModel)
	if len(m.filtered) != 1 || m.filtered[0].pkg != "axios" {
		t.Fatalf("filtered = %v, want exactly the axios row", m.filtered)
	}

	// Backspace widens the filter back out.
	for range "axios" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(browseModel)
	}
	if m.filter != "" || len(m.filtered) != 4 {
		t.Fatalf("after full backspace: filter=%q filtered=%d, want empty filter and 4 rows", m.filter, len(m.filtered))
	}
}

func TestBrowseModelFilterMatchesAnyColumn(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, _ := typeKey(m, "OTHER") // uppercase: must match case-insensitively against repo
	m = updated.(browseModel)
	if len(m.filtered) != 1 || m.filtered[0].repo != "other/service" {
		t.Fatalf("filtered = %v, want exactly the other/service row", m.filtered)
	}
}

func TestBrowseModelSortCycle(t *testing.T) {
	m := newBrowseModel(testRows())
	if sortCycle[m.sortIdx] != (sortState{sortRepo, true}) {
		t.Fatalf("initial sort = %v, want repo asc", sortCycle[m.sortIdx])
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(browseModel)
	if sortCycle[m.sortIdx] != (sortState{sortRepo, false}) {
		t.Fatalf("after 1 s-press, sort = %v, want repo desc", sortCycle[m.sortIdx])
	}
	// repo desc: other/service, acme/web, acme/api, acme/api
	if m.filtered[0].repo != "other/service" {
		t.Fatalf("filtered[0].repo = %q, want other/service (repo desc)", m.filtered[0].repo)
	}

	for i := 0; i < 3; i++ { // 3 more presses -> 4 total -> package asc (index 4)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		m = updated.(browseModel)
	}
	if sortCycle[m.sortIdx] != (sortState{sortPackage, true}) {
		t.Fatalf("after 4 s-presses, sort = %v, want package asc", sortCycle[m.sortIdx])
	}
	if m.filtered[0].pkg != "axios" { // "axios" < "lodash"
		t.Fatalf("filtered[0].pkg = %q, want axios (package asc)", m.filtered[0].pkg)
	}

	for i := 0; i < 4; i++ { // 4 more presses -> 8 total -> back to repo asc (full cycle)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		m = updated.(browseModel)
	}
	if sortCycle[m.sortIdx] != (sortState{sortRepo, true}) {
		t.Fatalf("after 8 s-presses, sort = %v, want repo asc (full cycle)", sortCycle[m.sortIdx])
	}
}

func TestBrowseModelEnterOpensDetailDedupedAndSorted(t *testing.T) {
	rows := []row{
		{"zeta/z", "npm", "lodash", "4.17.21"},
		{"acme/api", "npm", "lodash", "4.17.21"},
		{"acme/api", "npm", "lodash", "4.17.21"}, // duplicate within the same repo
		{"acme/api", "npm", "lodash", "4.17.20"}, // different version: excluded
	}
	m := newBrowseModel(rows)
	// Default sort (repo asc) puts acme/api first; cursor starts at 0.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(browseModel)

	if m.view != viewDetail {
		t.Fatalf("view = %v, want viewDetail", m.view)
	}
	if m.detail.pkg != "lodash" || m.detail.version != "4.17.21" {
		t.Fatalf("detail = %+v, want lodash@4.17.21", m.detail)
	}
	want := []string{"acme/api", "zeta/z"} // deduped, alphabetical
	if len(m.detail.repos) != len(want) {
		t.Fatalf("detail.repos = %v, want %v", m.detail.repos, want)
	}
	for i, r := range want {
		if m.detail.repos[i] != r {
			t.Fatalf("detail.repos = %v, want %v", m.detail.repos, want)
		}
	}
}

func TestBrowseModelEscReturnsToListPreservingState(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, _ := typeKey(m, "lodash")
	m = updated.(browseModel)
	if !m.filtering {
		t.Fatal("expected filtering mode to still be active after typing")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // exits filtering mode
	m = updated.(browseModel)
	if m.filtering {
		t.Fatal("expected filtering mode to end after enter")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // now opens detail
	m = updated.(browseModel)
	if m.view != viewDetail {
		t.Fatal("expected viewDetail before esc")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(browseModel)
	if m.view != viewList {
		t.Fatalf("view = %v, want viewList after esc", m.view)
	}
	if m.filter != "lodash" {
		t.Fatalf("filter = %q, want preserved %q", m.filter, "lodash")
	}
}

func TestBrowseModelQuitFromListAndDetail(t *testing.T) {
	m := newBrowseModel(testRows())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a quit cmd from list view")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("cmd() = %T, want tea.QuitMsg", cmd())
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(browseModel)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected a quit cmd from detail view")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestBrowseModelWindowSizeMsg(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updated.(browseModel)
	if m.tbl.Height() != 13 { // SetHeight(20-6=14) -> Height() is 14-1=13 (table reserves 1 row for its own header)
		t.Fatalf("table height = %d, want 13", m.tbl.Height())
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 3})
	m = updated.(browseModel)
	if m.tbl.Height() != 0 { // h clamps to 1 (3-6 would be negative), SetHeight(1) -> Height() is 1-1=0
		t.Fatalf("table height = %d, want 0 (clamped h=1, minus the table's own header row)", m.tbl.Height())
	}
}

func TestBrowseModelArrowKeyReachesTable(t *testing.T) {
	m := newBrowseModel(testRows())
	start := m.tbl.Cursor()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(browseModel)
	if m.tbl.Cursor() == start && len(m.filtered) > 1 {
		t.Fatalf("cursor did not move on KeyDown: still %d", m.tbl.Cursor())
	}
}

func TestBrowseModelInitAndView(t *testing.T) {
	m := newBrowseModel(testRows())
	if m.Init() != nil {
		t.Fatal("Init() should return nil (no initial command)")
	}
	if got := m.View(); got == "" {
		t.Fatal("list View() returned empty string")
	}
	// Test View() with descending sort to cover sortLabel's descending branch
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(browseModel)
	if got := m.View(); got == "" {
		t.Fatal("list View() with descending sort returned empty string")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(browseModel)
	if got := m.View(); got == "" {
		t.Fatal("detail View() returned empty string")
	}
}

func TestBrowseModelEnterOutOfBoundsIsNoOp(t *testing.T) {
	m := newBrowseModel(nil) // no rows at all
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(browseModel)
	if m.view != viewList {
		t.Fatalf("view = %v, want viewList (Enter on empty list is a no-op)", m.view)
	}
}

func TestBrowseModelFilterDoesNotCollideWithCommands(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, cmd := typeKey(m, "s")
	m = updated.(browseModel)
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("typing 's' while filtering must not quit")
		}
	}
	if m.filter != "s" {
		t.Fatalf("filter = %q, want %q (typing 's' while filtering must not trigger a sort cycle)", m.filter, "s")
	}
	if sortCycle[m.sortIdx] != (sortState{sortRepo, true}) {
		t.Fatalf("sort state changed to %v after typing 's' while filtering, want unchanged repo asc", sortCycle[m.sortIdx])
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(browseModel)
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("typing 'q' while filtering must not quit")
		}
	}
	if m.filter != "sq" {
		t.Fatalf("filter = %q, want %q", m.filter, "sq")
	}
}

func TestBrowseModelSlashEntersFilteringMode(t *testing.T) {
	m := newBrowseModel(testRows())
	if m.filtering {
		t.Fatal("new model should not start in filtering mode")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(browseModel)
	if !m.filtering {
		t.Fatal("'/' should enter filtering mode")
	}
}

func TestBrowseModelFilterBackspaceHandlesMultibyteRunes(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(browseModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'é'}})
	m = updated.(browseModel)
	if m.filter != "é" {
		t.Fatalf("filter = %q, want %q", m.filter, "é")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(browseModel)
	if m.filter != "" {
		t.Fatalf("filter = %q after backspace on a multi-byte rune, want empty (not corrupted mid-byte)", m.filter)
	}
}

func TestBrowseModelFilteringModeView(t *testing.T) {
	m := newBrowseModel(testRows())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(browseModel)
	if !m.filtering {
		t.Fatal("expected filtering mode after '/'")
	}
	view := m.View()
	if !strings.Contains(view, "done filtering") {
		t.Fatalf("View() while filtering = %q, want it to mention the filtering footer hint", view)
	}
	if !strings.Contains(view, "█") {
		t.Fatalf("View() while filtering = %q, want the cursor block rendered in the filter line", view)
	}
}

func TestBrowseModelSortIsStable(t *testing.T) {
	rows := []row{
		{"acme/z", "npm", "pkg-b", "1.0"},
		{"acme/z", "npm", "pkg-a", "1.0"},
	}
	m := newBrowseModel(rows)
	// Cycle to ecosystem asc (both rows tie on "npm"): a stable sort must
	// preserve the original relative order (pkg-b before pkg-a) instead of
	// reordering ties arbitrarily.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(browseModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(browseModel)
	if sortCycle[m.sortIdx] != (sortState{sortEcosystem, true}) {
		t.Fatalf("sort = %v, want ecosystem asc", sortCycle[m.sortIdx])
	}
	if m.filtered[0].pkg != "pkg-b" || m.filtered[1].pkg != "pkg-a" {
		t.Fatalf("filtered = %v, want original order preserved for tied ecosystem", m.filtered)
	}
}
