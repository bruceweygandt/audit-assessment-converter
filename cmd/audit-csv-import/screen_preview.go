package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func truncate(s string, max int) string {
	// Remove newlines for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func (m *Model) updateCSVPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.step = StepAssessmentSelect
			return m, nil
		case "left", "h":
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		case "right", "l":
			if m.previewScroll < len(m.csvHeaders)-1 {
				m.previewScroll++
			}
			return m, nil
		case "tab":
			// Pre-select the most likely ID column so tab usually just works
			m.idColumnSuggested = SuggestIDColumn(m.csvHeaders, m.csvRows, m.existingRequirementIDs())
			m.idColumnCursor = m.idColumnSuggested
			m.step = StepIDColumnSelect
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewCSVPreview() string {
	var b strings.Builder
	fileName := m.csvPath
	if idx := strings.LastIndex(fileName, "/"); idx >= 0 {
		fileName = fileName[idx+1:]
	}
	b.WriteString(m.header("Check the CSV"))
	b.WriteString(guide(
		"Make sure this is the right file and the columns line up. If text looks",
		"garbled or columns are shifted, re-save the spreadsheet as CSV UTF-8.",
		"You'll choose what each column is for in the next steps.",
	))
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("%s • %d rows • %d columns", fileName, len(m.csvRows), len(m.csvHeaders))))
	b.WriteString("\n\n")

	// Determine how many columns fit, starting from the scrolled-to column
	colWidth := 30
	fitCols := len(m.csvHeaders)
	if w := m.contentWidth(); w > 0 {
		if f := (w - 8) / (colWidth + 3); f < fitCols && f > 0 {
			fitCols = f
		}
	}
	if m.previewScroll > len(m.csvHeaders)-fitCols {
		m.previewScroll = len(m.csvHeaders) - fitCols
	}
	if m.previewScroll < 0 {
		m.previewScroll = 0
	}
	first, last := m.previewScroll, m.previewScroll+fitCols
	if last > len(m.csvHeaders) {
		last = len(m.csvHeaders)
	}
	if first > 0 || last < len(m.csvHeaders) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  Columns %d–%d of %d  (←/→ to see the others)", first+1, last, len(m.csvHeaders))) + "\n")
	}

	// Header row
	var headers []string
	for i := first; i < last; i++ {
		headers = append(headers, fmt.Sprintf("%-*s", colWidth, truncate(m.csvHeaders[i], colWidth)))
	}
	b.WriteString("  " + activeFieldStyle.Render(strings.Join(headers, " │ ")))
	b.WriteString("\n")

	// Separator
	sep := strings.Repeat("─", colWidth)
	var seps []string
	for i := first; i < last; i++ {
		seps = append(seps, sep)
	}
	b.WriteString("  " + dimStyle.Render(strings.Join(seps, "─┼─")))
	b.WriteString("\n")

	// Data rows (first 5)
	rows := m.csvRows
	if len(rows) > 5 {
		rows = rows[:5]
	}
	for _, row := range rows {
		var cells []string
		for i := first; i < last; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			cells = append(cells, fmt.Sprintf("%-*s", colWidth, truncate(cell, colWidth)))
		}
		b.WriteString("  " + strings.Join(cells, " │ "))
		b.WriteString("\n")
	}

	if len(m.csvRows) > 5 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("\n  ... and %d more rows", len(m.csvRows)-5)))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n  ←/→ scroll columns • tab continue • esc back"))
	return b.String()
}
