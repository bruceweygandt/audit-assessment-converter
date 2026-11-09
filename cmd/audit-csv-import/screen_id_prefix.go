package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) initIDPrefixStrip() {
	ti := textinput.New()
	ti.Placeholder = "e.g. FRR-,KSI- (leave empty to skip)"
	ti.CharLimit = 100
	ti.Width = 50
	ti.Focus()

	// Preserve any existing prefix setting when navigating back; otherwise
	// pre-fill the prefixes that make CSV IDs match the assessment's IDs.
	m.suggestedPrefixes = SuggestPrefixes(columnValues(m.csvRows, m.idColumnIdx), m.existingRequirementIDs())
	if len(m.idPrefixes) > 0 {
		ti.SetValue(strings.Join(m.idPrefixes, ","))
	} else if len(m.suggestedPrefixes) > 0 {
		ti.SetValue(strings.Join(m.suggestedPrefixes, ","))
	}

	m.idPrefixInput = ti
}

// parsePrefixes splits a comma-separated input into trimmed, non-empty prefix strings.
func parsePrefixes(input string) []string {
	var prefixes []string
	for _, p := range strings.Split(input, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

// stripIDPrefix removes the first matching prefix from an ID string.
func stripIDPrefix(id string, prefixes []string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(id, prefix) {
			return id[len(prefix):]
		}
	}
	return id
}

// matchSummary shows how many CSV IDs, after stripping, match the assessment.
func (m *Model) matchSummary(prefixes []string, existing map[string]bool) string {
	ids := columnValues(m.csvRows, m.idColumnIdx)
	var unmatched []string
	for _, id := range ids {
		if !existing[stripIDPrefix(id, prefixes)] {
			unmatched = append(unmatched, id)
		}
	}
	matched := len(ids) - len(unmatched)
	var b strings.Builder
	summary := fmt.Sprintf("%d of %d CSV IDs match a requirement in this assessment.", matched, len(ids))
	if len(unmatched) == 0 {
		b.WriteString("  " + successStyle.Render("✓ "+summary) + "\n")
	} else {
		b.WriteString("  " + warningStyle.Render("! "+summary) + "\n")
		shown := unmatched
		if len(shown) > 5 {
			shown = shown[:5]
		}
		more := ""
		if len(unmatched) > len(shown) {
			more = fmt.Sprintf(", +%d more", len(unmatched)-len(shown))
		}
		b.WriteString(dimStyle.Render("    Not matched (will be added as new requirements): "+strings.Join(shown, ", ")+more) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (m *Model) updateIDPrefixStrip(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.step = StepIDColumnSelect
			return m, nil
		case "tab":
			m.idPrefixes = parsePrefixes(m.idPrefixInput.Value())
			// Preview a row that matches an existing requirement, so there's a before/after to compare
			if existing := m.existingRequirementIDs(); existing != nil {
				for i, row := range m.csvRows {
					if m.idColumnIdx < len(row) && existing[stripIDPrefix(row[m.idColumnIdx], m.idPrefixes)] {
						m.previewRow = i
						break
					}
				}
			}
			m.step = StepColumnMapping
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.idPrefixInput, cmd = m.idPrefixInput.Update(msg)
	return m, cmd
}

func (m *Model) viewIDPrefixStrip() string {
	var b strings.Builder
	b.WriteString(m.header("Match IDs to Paramify"))
	b.WriteString(guide(
		"Auditors often add a prefix Paramify doesn't use: the CSV says KSI-CNA-03",
		"but Paramify says CNA-03. List prefixes to remove, separated by commas",
		"(e.g. FRR-,KSI-). Leave it empty if the IDs already match.",
	))
	if len(m.suggestedPrefixes) > 0 {
		b.WriteString(tip("filled in from comparing your CSV with the assessment. Press tab to accept."))
		b.WriteString("\n")
	}

	b.WriteString("  Prefixes: ")
	b.WriteString(m.idPrefixInput.View())
	b.WriteString("\n")

	// Show a preview of the first few IDs with/without stripping
	prefixes := parsePrefixes(m.idPrefixInput.Value())
	b.WriteString("\n")
	if existing := m.existingRequirementIDs(); existing != nil {
		b.WriteString(m.matchSummary(prefixes, existing))
	}
	b.WriteString("  " + activeFieldStyle.Render("ID Preview:"))
	b.WriteString("\n")

	previewCount := 8
	if len(m.csvRows) < previewCount {
		previewCount = len(m.csvRows)
	}

	for i := 0; i < previewCount; i++ {
		if m.idColumnIdx >= len(m.csvRows[i]) {
			continue
		}
		original := m.csvRows[i][m.idColumnIdx]
		stripped := stripIDPrefix(original, prefixes)

		if original != stripped {
			b.WriteString(fmt.Sprintf("    %s → %s\n",
				dimStyle.Render(truncate(original, 30)),
				selectedStyle.Render(stripped),
			))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", truncate(original, 40)))
		}
	}
	if len(m.csvRows) > previewCount {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ... and %d more\n", len(m.csvRows)-previewCount)))
	}

	b.WriteString(helpStyle.Render("\n  tab continue • esc back"))
	return b.String()
}
