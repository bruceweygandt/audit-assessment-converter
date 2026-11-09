package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type generateDoneMsg struct {
	path    string
	updated []string // requirement IDs that matched the existing assessment
	added   []string // requirement IDs not found and appended as new
	err     error
}

// generateConfig collects the user's choices into a GenerateConfig.
func (m *Model) generateConfig() GenerateConfig {
	return GenerateConfig{
		IDColumnIdx:       m.idColumnIdx,
		ColumnMappings:    m.columnMappings,
		DefaultStatus:     m.defaultStatus,
		DefaultMethod:     m.defaultMethod,
		AssignedEmail:     m.assignedEmail,
		OwnerEmail:        m.ownerEmail,
		ReviewerEmail:     m.reviewerEmail,
		PlanText:          m.planText,
		DefaultStepStatus: m.defaultStepStatus,
		IDPrefixes:        m.idPrefixes,
	}
}

func (m *Model) generateCmd() tea.Cmd {
	return func() tea.Msg {
		config := m.generateConfig()

		csvData := &CSVData{
			Headers: m.csvHeaders,
			Rows:    m.csvRows,
		}

		// Write only the merged assessment: importing a file that also carried
		// the untouched ones would duplicate them in Paramify. Its name,
		// target and program come from the download unchanged.
		merged := m.existingData[m.selectedAssessmentIdx]
		updated, added := MergeIntoAssessment(&merged, csvData, config)
		if err := checkImportable(merged); err != nil {
			return generateDoneMsg{err: err}
		}
		if err := WriteAssessments(m.outputFilePath, []Assessment{merged}); err != nil {
			return generateDoneMsg{err: err}
		}
		return generateDoneMsg{path: m.outputFilePath, updated: updated, added: added}
	}
}

func (m *Model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.step = StepOutputPath
			return m, nil
		case "tab":
			m.step = StepGenerating
			return m, m.generateCmd()
		}
	case generateDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.errReturn = StepReview
			m.step = StepError
			return m, nil
		}
		m.generatedPath = msg.path
		m.mergeUpdated = msg.updated
		m.mergeAdded = msg.added
		m.step = StepDone
		return m, nil
	}
	return m, nil
}

func (m *Model) viewReview() string {
	var b strings.Builder
	b.WriteString(m.header("Review & Create the File"))
	b.WriteString(guide(
		"Check everything below. Pressing tab writes the output file: it holds",
		"only the assessment you're updating, so importing it won't duplicate the",
		"others. Nothing is sent to Paramify. Press esc to go back and change anything.",
	))

	// CSV file
	fileName := m.csvPath
	if idx := strings.LastIndex(fileName, "/"); idx >= 0 {
		fileName = fileName[idx+1:]
	}
	b.WriteString(fmt.Sprintf("  %s %s (%d rows)\n", labelStyle.Render("CSV:"), truncate(fileName, 50), len(m.csvRows)))

	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Base JSON:"), filepath.Base(m.jsonPath)))
	if m.selectedAssessmentIdx < len(m.existingData) {
		a := m.existingData[m.selectedAssessmentIdx]
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Assessment:"), a.Name))
		b.WriteString(fmt.Sprintf("  %s %s  %s\n", labelStyle.Render("Program:"), a.Program, dimStyle.Render("(both from your download)")))
	}
	// Per-row fields: where each row's value comes from, column first
	for _, f := range []int{metaStatus, metaMethod, metaStepStatus, metaAssigned, metaOwner, metaReviewer} {
		src := metaSource[metaSourceOf[f]]
		srcNote, valueNote := m.fieldCountNotes(f)
		line := valueNote
		if col := m.mappedColumn(src.target); col != "" {
			line = fmt.Sprintf("from %q: %s · %s", col, srcNote, valueNote)
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render(metaLabels[f]), line))
		if w := m.columnWarning(src.target, src.parse); w != "" {
			b.WriteString("  " + labelStyle.Render("") + " " + warningStyle.Render("! "+w) + "\n")
		}
	}

	srcNote, valueNote := m.planCountNotes()
	line := valueNote
	if cols := m.columnsFor(MapPlan); len(cols) > 0 {
		line = srcNote + " · " + valueNote
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Plan:"), line))

	// ID column
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("ID Column:"), m.csvHeaders[m.idColumnIdx]))

	// ID prefix strip
	if len(m.idPrefixes) > 0 {
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Strip Prefix:"), strings.Join(m.idPrefixes, ", ")))
	}

	// Mappings
	b.WriteString("\n")
	b.WriteString("  " + activeFieldStyle.Render("Column Mappings:"))
	b.WriteString("\n")
	for i, colIdx := range m.mappingColumns {
		_ = i
		header := m.csvHeaders[colIdx]
		mapping := m.columnMappings[colIdx]
		b.WriteString(fmt.Sprintf("    %-30s → %s\n", truncate(header, 30), mappingColor(mapping).Render(mapping.String())))
	}

	// Flag per-row values that will fall back to the defaults
	checks := []struct {
		target   MappingTarget
		parse    func(string) string
		fallback string
	}{
		{MapStatus, ParseRequirementStatus, "the Status " + m.defaultStatus},
		{MapMethod, ParseMethod, "the Method " + m.defaultMethod},
		{MapStepStatus, ParseStepStatus, "the Step Status " + m.defaultStepStatus},
	}
	for _, c := range checks {
		for i, t := range m.columnMappings {
			if t != c.target {
				continue
			}
			if bad := Unrecognized(m.csvRows, i, c.parse); len(bad) > 0 {
				shown := bad
				if len(shown) > 4 {
					shown = shown[:4]
				}
				b.WriteString("\n  " + warningStyle.Render(fmt.Sprintf("! %q has values the tool doesn't recognize: %s", m.csvHeaders[i], strings.Join(shown, ", "))))
				b.WriteString("\n    " + dimStyle.Render("Those rows use "+c.fallback+" instead."))
				b.WriteString("\n")
			}
		}
	}

	// Output path
	b.WriteString(fmt.Sprintf("\n  %s %s\n", labelStyle.Render("Output:"), m.outputFilePath))

	b.WriteString(helpStyle.Render("\n  tab create file • esc back • ctrl+c quit without creating"))
	return b.String()
}

func (m *Model) viewGenerating() string {
	return titleStyle.Render("Generating...") + "\n\n  Please wait..."
}

func (m *Model) viewDone() string {
	var b strings.Builder
	b.WriteString(successStyle.Render("Done! Your file is ready to import."))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Assessment file written to:\n  %s\n", m.generatedPath))
	{
		b.WriteString(fmt.Sprintf("\n  %d requirements updated, %d added as new.\n", len(m.mergeUpdated), len(m.mergeAdded)))
		if len(m.mergeAdded) > 0 {
			b.WriteString(dimStyle.Render("  Not found in the existing assessment (check ID prefixes if these should have matched):"))
			b.WriteString("\n")
			shown := m.mergeAdded
			if len(shown) > 15 {
				shown = shown[:15]
			}
			b.WriteString("    " + strings.Join(shown, ", "))
			if len(m.mergeAdded) > len(shown) {
				b.WriteString(fmt.Sprintf(", ... and %d more", len(m.mergeAdded)-len(shown)))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n  " + activeFieldStyle.Render("Next steps") + "\n")
	b.WriteString("  1. In Paramify, open Audit Assessments and import the file above.\n")
	b.WriteString("  2. Open a few updated requirements and check the result, remediation and status.\n")
	b.WriteString("  3. Keep your original download until you're happy with the import.\n")
	b.WriteString(helpStyle.Render("\n  tab or q quit"))
	return b.String()
}

func (m *Model) viewError() string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("Error"))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(fmt.Sprintf("  %s\n", m.err.Error()))
	}
	b.WriteString(helpStyle.Render("\n  esc go back and fix it • q quit"))
	return b.String()
}

// checkImportable refuses to write an assessment Paramify's import would
// reject: it needs a name and the program it belongs to.
func checkImportable(a Assessment) error {
	var missing []string
	if strings.TrimSpace(a.Name) == "" {
		missing = append(missing, "name")
	}
	if strings.TrimSpace(a.Program) == "" {
		missing = append(missing, "program")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("the assessment has no %s, so Paramify would reject it (\"Program \"\" not found\"). Nothing was written.\nGo back to the Assessment Details step and fill in the Name and Target (the program name exactly as in Paramify)", strings.Join(missing, " or "))
}
