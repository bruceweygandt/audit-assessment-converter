package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateIDColumnSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.step = StepCSVPreview
			return m, nil
		case "up", "k":
			if m.idColumnCursor > 0 {
				m.idColumnCursor--
			}
		case "down", "j":
			if m.idColumnCursor < len(m.csvHeaders)-1 {
				m.idColumnCursor++
			}
		case "tab":
			m.idColumnIdx = m.idColumnCursor
			// Reset mappings: set selected column as ID, others as skip
			// Start every other column on its suggested mapping
			m.suggestedMappings = SuggestMappings(m.csvHeaders, m.csvRows, m.idColumnIdx)
			m.columnMappings = make([]MappingTarget, len(m.csvHeaders))
			for i, sg := range m.suggestedMappings {
				m.columnMappings[i] = sg.Target
			}
			// Initialize column mapping screen
			m.mappingColumns = nil
			for i := range m.csvHeaders {
				if i != m.idColumnIdx {
					m.mappingColumns = append(m.mappingColumns, i)
				}
			}
			m.mappingCursor = 0
			m.mappingTargetCursor = 0
			if len(m.mappingColumns) > 0 {
				m.mappingTargetCursor = targetIndex(m.columnMappings[m.mappingColumns[0]])
			}
			// Go to prefix strip screen
			m.initIDPrefixStrip()
			m.step = StepIDPrefixStrip
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewIDColumnSelect() string {
	var b strings.Builder
	b.WriteString(m.header("Pick the Requirement ID Column"))
	b.WriteString(guide(
		"Which column says which requirement each row is about (e.g. KSI-CNA-03,",
		"AC-2)? The tool uses it to match rows to requirements in Paramify.",
		"The most likely column is already highlighted; press tab to use it.",
	))

	listH := m.height - 14
	if listH < 5 {
		listH = 5
	}
	start, end := visibleWindow(m.idColumnCursor, len(m.csvHeaders), listH)
	if start > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ↑ %d more columns", start)) + "\n")
	}
	for i := start; i < end; i++ {
		header := m.csvHeaders[i]
		sample := ""
		if len(m.csvRows) > 0 && i < len(m.csvRows[0]) {
			sample = truncate(m.csvRows[0][i], 40)
		}

		tag := ""
		if i == m.idColumnSuggested {
			tag = " " + suggestedTag()
		}
		name := fmt.Sprintf("%-30s", truncate(header, 30))
		if i == m.idColumnCursor {
			b.WriteString(fmt.Sprintf("  %s %s %s%s\n",
				cursorStyle.Render(">"),
				selectedStyle.Render(name),
				dimStyle.Render(fmt.Sprintf("e.g. %q", sample)),
				tag,
			))
		} else {
			b.WriteString(fmt.Sprintf("    %s %s%s\n",
				name,
				dimStyle.Render(fmt.Sprintf("e.g. %q", sample)),
				tag,
			))
		}
	}

	if end < len(m.csvHeaders) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ↓ %d more columns", len(m.csvHeaders)-end)) + "\n")
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d columns in this CSV", len(m.csvHeaders))) + "\n")
	b.WriteString(helpStyle.Render("\n  ↑/↓ choose • tab continue • esc back"))
	return b.String()
}
