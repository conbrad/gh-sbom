package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type browseView int

const (
	viewList browseView = iota
	viewDetail
)

type sortColumn int

const (
	sortRepo sortColumn = iota
	sortEcosystem
	sortPackage
	sortVersion
)

var sortColumnNames = [...]string{"repo", "ecosystem", "package", "version"}

// sortState is one step in the 8-state cycle: 4 columns x 2 directions.
type sortState struct {
	col sortColumn
	asc bool
}

var sortCycle = []sortState{
	{sortRepo, true}, {sortRepo, false},
	{sortEcosystem, true}, {sortEcosystem, false},
	{sortPackage, true}, {sortPackage, false},
	{sortVersion, true}, {sortVersion, false},
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true)
	footerStyle = lipgloss.NewStyle().Faint(true)
)

type detailData struct {
	pkg, version string
	repos        []string
}

// browseModel implements tea.Model for `gh sbom browse`.
type browseModel struct {
	all       []row // full unfiltered set, loaded once
	filtered  []row // currently visible, after filter+sort; indices match tbl's rows
	filter    string
	filtering bool // true while editing the filter (entered via '/'); single-letter
	// commands (s, q, /) are only live when this is false, so they never
	// collide with filter text being typed.
	sortIdx int
	view    browseView
	tbl     table.Model
	detail  detailData
}

func newBrowseModel(rows []row) browseModel {
	m := browseModel{all: rows}
	m.tbl = table.New(
		table.WithColumns([]table.Column{
			{Title: "Repo", Width: 24},
			{Title: "Ecosystem", Width: 12},
			{Title: "Package", Width: 30},
			{Title: "Version", Width: 14},
		}),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	m.applyFilterAndSort()
	return m
}

func (m browseModel) Init() tea.Cmd { return nil }

func (m browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h := msg.Height - 6
		if h < 1 {
			h = 1
		}
		m.tbl.SetHeight(h)
		return m, nil
	case tea.KeyMsg:
		if m.view == viewDetail {
			switch msg.String() {
			case "esc", "enter":
				m.view = viewList
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.filtering {
			switch msg.String() {
			case "esc", "enter":
				m.filtering = false
			case "backspace":
				if m.filter != "" {
					r := []rune(m.filter)
					m.filter = string(r[:len(r)-1])
					m.applyFilterAndSort()
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.filter += string(msg.Runes)
					m.applyFilterAndSort()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.filtering = true
			return m, nil
		case "s":
			m.sortIdx = (m.sortIdx + 1) % len(sortCycle)
			m.applyFilterAndSort()
			return m, nil
		case "enter":
			m.openDetail()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

func (m *browseModel) applyFilterAndSort() {
	q := strings.ToLower(m.filter)
	filtered := make([]row, 0, len(m.all))
	for _, r := range m.all {
		if q == "" || strings.Contains(strings.ToLower(r.repo+" "+r.ecosystem+" "+r.pkg+" "+r.version), q) {
			filtered = append(filtered, r)
		}
	}
	st := sortCycle[m.sortIdx]
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := sortField(filtered[i], st.col), sortField(filtered[j], st.col)
		if st.asc {
			return a < b
		}
		return a > b
	})
	m.filtered = filtered

	rows := make([]table.Row, len(filtered))
	for i, r := range filtered {
		rows[i] = table.Row{r.repo, r.ecosystem, r.pkg, r.version}
	}
	m.tbl.SetRows(rows)
}

func sortField(r row, c sortColumn) string {
	switch c {
	case sortEcosystem:
		return r.ecosystem
	case sortPackage:
		return r.pkg
	case sortVersion:
		return r.version
	default:
		return r.repo
	}
}

func (m *browseModel) openDetail() {
	c := m.tbl.Cursor()
	if c < 0 || c >= len(m.filtered) {
		return
	}
	sel := m.filtered[c]
	seen := map[string]bool{}
	var repos []string
	for _, r := range m.all {
		if r.pkg == sel.pkg && r.version == sel.version && !seen[r.repo] {
			seen[r.repo] = true
			repos = append(repos, r.repo)
		}
	}
	sort.Strings(repos)
	m.detail = detailData{pkg: sel.pkg, version: sel.version, repos: repos}
	m.view = viewDetail
}

func (m browseModel) View() string {
	if m.view == viewDetail {
		return m.detailView()
	}
	return m.listView()
}

func (m browseModel) listView() string {
	header := headerStyle.Render(fmt.Sprintf("gh-sbom browse — %d dependencies", len(m.filtered)))
	cursor := ""
	if m.filtering {
		cursor = "█"
	}
	filterLine := fmt.Sprintf("Filter: %s%s", m.filter, cursor)
	var footer string
	if m.filtering {
		footer = footerStyle.Render("esc/enter: done filtering")
	} else {
		footer = footerStyle.Render(fmt.Sprintf("sort: %s (s: cycle)   /: filter   enter: repos on this version   q: quit", sortLabel(sortCycle[m.sortIdx])))
	}
	return header + "\n\n" + filterLine + "\n\n" + m.tbl.View() + "\n\n" + footer
}

func sortLabel(s sortState) string {
	dir := "↑"
	if !s.asc {
		dir = "↓"
	}
	return sortColumnNames[s.col] + " " + dir
}

func (m browseModel) detailView() string {
	header := headerStyle.Render(fmt.Sprintf("gh-sbom browse — %s @ %s", m.detail.pkg, m.detail.version))
	var b strings.Builder
	fmt.Fprintf(&b, "%d repo(s) on this exact version:\n\n", len(m.detail.repos))
	for _, r := range m.detail.repos {
		fmt.Fprintf(&b, "  %s\n", r)
	}
	footer := footerStyle.Render("esc/enter: back to list   q: quit")
	return header + "\n\n" + b.String() + "\n" + footer
}
