package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateColumnMapping(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.mappingCursor > 0 {
				m.mappingCursor--
				// Restore target cursor to current mapping
				colIdx := m.mappingColumns[m.mappingCursor]
				m.mappingTargetCursor = targetIndex(m.columnMappings[colIdx])
			} else {
				m.step = StepIDPrefixStrip
			}
			return m, nil
		case "up", "k":
			if m.mappingTargetCursor > 0 {
				m.mappingTargetCursor--
			}
		case "down", "j":
			targets := AllMappingTargets()
			if m.mappingTargetCursor < len(targets)-1 {
				m.mappingTargetCursor++
			}
		case "tab":
			// Confirm this column and continue with the rest as listed. Columns
			// nobody has looked at get one warning first.
			if !m.canAcceptRest() && !m.mappingWarned {
				m.mappingWarned = true
				return m, nil
			}
			m.mappingWarned = false
			m.applyCurrentMapping()
			m.initMetadata()
			m.step = StepMetadata
			return m, nil
		case "enter":
			m.mappingWarned = false
			m.applyCurrentMapping()

			// Move to the next column. On the last one, stay: only tab leaves the step.
			if m.mappingCursor < len(m.mappingColumns)-1 {
				m.mappingCursor++
				// Start on the column's saved choice so revisiting doesn't overwrite it
				m.mappingTargetCursor = targetIndex(m.columnMappings[m.mappingColumns[m.mappingCursor]])
			}
			return m, nil
		}
	}
	return m, nil
}

// canAcceptRest reports whether every column after the current one is either
// empty or has a content-checked suggestion, so tab can't silently skip a
// column nobody has looked at.
func (m *Model) canAcceptRest() bool {
	for _, ci := range m.mappingColumns[m.mappingCursor+1:] {
		if ci >= len(m.suggestedMappings) {
			return false
		}
		if sg := m.suggestedMappings[ci]; sg.Target == MapSkip && sg.Reason == "" {
			return false
		}
	}
	return true
}

// applyCurrentMapping saves the highlighted target for the current column.
// Every target except result takes a single column: picking one here moves it
// off whichever column had it before.
func (m *Model) applyCurrentMapping() {
	colIdx := m.mappingColumns[m.mappingCursor]
	target := AllMappingTargets()[m.mappingTargetCursor]
	if prev := m.singleTargetColumn(target, colIdx); prev >= 0 {
		m.columnMappings[prev] = MapSkip
	}
	m.columnMappings[colIdx] = target
}

// singleTargetColumn returns the index of another column already mapped to
// target when target allows only one column (everything except result), or -1.
func (m *Model) singleTargetColumn(target MappingTarget, exclude int) int {
	if target.MultiColumn() {
		return -1
	}
	for _, ci := range m.mappingColumns {
		if ci != exclude && m.columnMappings[ci] == target {
			return ci
		}
	}
	return -1
}

// mappingDetails explains what each choice does in Paramify.
var mappingDetails = map[MappingTarget]string{
	MapSkip:        "Not imported. Use for background columns Paramify already has (requirement text, descriptions, scope).",
	MapPlan:        "Adds to the step's Plan under a bold heading with this column's name. Map several columns to plan to stack them, in the CSV's column order; rows where they're all blank can use plan text typed on the Details step.",
	MapResult:      "Adds to the step's Result under a bold heading with this column's name. Map several columns to result to stack them, in the CSV's column order; empty cells are left out.",
	MapRemediation: "Fills the step's Remediation: gaps and what needs fixing. One column only; empty cells leave it blank (or keep the existing text when merging).",
	MapStatus:      "Sets each requirement's Review Status, one of Paramify's eight: Open, Overdue, Submitted, Accepted, Archived, Change Requested, QA Requested, Reviewed (also In review → Submitted, Approved → Accepted, Rejected → Change Requested). Blank cells use the Review Status from the details screen.",
	MapMethod:      "Sets each step's Method from Test, Examine, Interview or Unknown. Blank or unrecognized cells use the Method chosen on the details screen.",
	MapStepStatus:  "Sets each step's status: Pass/Satisfied/Met → PASS, Fail/Other than satisfied/Not met → FAIL, Partial → PARTIAL, Open/Pending → OPEN. Blank cells use the Step Status from the details screen.",
	MapAssigned:    "Sets the assessor (email) assigned to each step. Blank cells use the Assigned value from the details screen, or keep who's already set.",
	MapOwner:       "Sets each requirement's Assurance Owner (email or team name). Blank cells use the Assurance Owner from the details screen, or keep who's already set.",
	MapReviewer:    "Sets each requirement's reviewer (email). Blank cells use the Reviewer from the details screen, or keep who's already set.",
}

func targetIndex(t MappingTarget) int {
	for i, target := range AllMappingTargets() {
		if target == t {
			return i
		}
	}
	return 0
}

func (m *Model) viewColumnMapping() string {
	var b strings.Builder
	colIdx := m.mappingColumns[m.mappingCursor]
	header := m.csvHeaders[colIdx]

	b.WriteString(m.header(fmt.Sprintf("Map Columns · %d of %d", m.mappingCursor+1, len(m.mappingColumns))))
	b.WriteString(guide(
		"Choose where this column's text goes on each requirement's capability",
		"assurance step in Paramify. A column is pre-selected only when its contents",
		"prove it (e.g. every cell is Examine/Test/Interview). Text columns start on",
		"skip: choose result, plan or remediation for the ones you want imported.",
		"Every column in the CSV is listed, whatever the spreadsheet looks like.",
	))

	b.WriteString(fmt.Sprintf("  Column: %s\n", activeFieldStyle.Render(header)))

	// Show a few distinct values so the user can judge what the column holds
	vals := columnValues(m.csvRows, colIdx)
	seen := make(map[string]bool)
	var samples []string
	for _, v := range vals {
		if !seen[v] && len(samples) < 3 {
			seen[v] = true
			samples = append(samples, fmt.Sprintf("%q", truncate(v, 50)))
		}
	}
	if len(samples) == 0 {
		b.WriteString("  " + dimStyle.Render("Sample: (every cell is empty)") + "\n")
	} else {
		b.WriteString(fmt.Sprintf("  Sample: %s\n", dimStyle.Render(strings.Join(samples, ", "))))
		b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(fmt.Sprintf("%d of %d rows filled", len(vals), len(m.csvRows)))))
	}
	var sg Suggestion
	if colIdx < len(m.suggestedMappings) {
		sg = m.suggestedMappings[colIdx]
	}
	if sg.Target != MapSkip && sg.Reason != "" {
		b.WriteString("  " + suggestedTagStyle.Render(fmt.Sprintf("Suggested %s: %s.", sg.Target, sg.Reason)) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  Map to:\n")
	targets := AllMappingTargets()
	descriptions := map[MappingTarget]string{
		MapSkip:        "Ignore this column",
		MapPlan:        "Assessment plan field",
		MapResult:      "Result field (with bold header)",
		MapRemediation: "Remediation field",
		MapStatus:      "Review status per row (Open / Submitted / Accepted / Change Requested / ...)",
		MapMethod:      "Method per row (Test / Examine / Interview / Unknown)",
		MapStepStatus:  "Step status per row (Open / Pass / Fail / Partial)",
		MapAssigned:    "Assessor assigned to the step, per row",
		MapOwner:       "Assurance owner of the requirement, per row",
		MapReviewer:    "Requirement reviewer, per row",
	}

	for i, t := range targets {
		desc := descriptions[t]
		if t == sg.Target && t != MapSkip && sg.Reason != "" {
			desc += " " + suggestedTag()
		}
		// Pad before styling so ANSI codes don't throw off the column alignment
		name := fmt.Sprintf("%-16s", t.String())
		if i == m.mappingTargetCursor {
			line := fmt.Sprintf("    %s %s  %s",
				cursorStyle.Render(">"),
				selectedStyle.Render(name),
				dimStyle.Render(desc),
			)
			if prev := m.singleTargetColumn(t, colIdx); prev >= 0 {
				line += "  " + warningStyle.Render(fmt.Sprintf("(replaces %q, which becomes skip)", truncate(m.csvHeaders[prev], 30)))
			}
			b.WriteString(line + "\n")
		} else {
			b.WriteString(fmt.Sprintf("      %s  %s\n",
				name,
				dimStyle.Render(desc),
			))
		}
	}

	// Explain the highlighted choice in more detail
	b.WriteString("\n" + tipStyle.PaddingLeft(2).Width(m.contentWidth()-2).Render(mappingDetails[targets[m.mappingTargetCursor]]) + "\n")

	// Show summary of mappings, windowed around the current column
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  ── Mappings (%d columns) ──", len(m.mappingColumns))))
	b.WriteString("\n")
	listH := m.height - 30
	if listH < 5 {
		listH = 5
	}
	start, end := visibleWindow(m.mappingCursor, len(m.mappingColumns), listH)
	if start > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ↑ %d more", start)) + "\n")
	}
	for i := start; i < end; i++ {
		ci := m.mappingColumns[i]
		h := m.csvHeaders[ci]
		mapping := m.columnMappings[ci]
		marker := "  "
		if i == m.mappingCursor {
			marker = cursorStyle.Render("→ ")
		}
		if i == m.mappingCursor {
			// The current column shows what's highlighted, not what was saved
			mapping = targets[m.mappingTargetCursor]
		}
		label := mappingColor(mapping).Render(mapping.String())
		if i > m.mappingCursor {
			label += " " + dimStyle.Render("(not reviewed yet)")
		}
		b.WriteString(fmt.Sprintf("  %s%-28s %s\n", marker, truncate(h, 28), label))
	}
	if end < len(m.mappingColumns) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ↓ %d more", len(m.mappingColumns)-end)) + "\n")
	}

	if m.mappingWarned {
		unreviewed := len(m.mappingColumns) - m.mappingCursor - 1
		msg := fmt.Sprintf("%d columns after this one haven't been reviewed and would be imported as listed above. Press tab again to continue anyway, or enter to review them.", unreviewed)
		b.WriteString("\n" + warningStyle.PaddingLeft(2).Width(m.contentWidth()-2).Render(msg) + "\n")
	}

	back := "esc previous column"
	if m.mappingCursor == 0 {
		back = "esc back to prefixes"
	}
	if m.mappingCursor == len(m.mappingColumns)-1 {
		b.WriteString("\n  " + tipStyle.Render("This is the last column. Press tab to continue to the details step.") + "\n")
		b.WriteString(helpStyle.Render("\n  ↑/↓ select target • enter confirm • tab continue to details • " + back))
		return b.String()
	}
	b.WriteString(helpStyle.Render("\n  ↑/↓ select target • enter confirm & next column • tab continue to details • " + back))
	return b.String()
}
