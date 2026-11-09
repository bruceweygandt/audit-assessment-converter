package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateAssessmentSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.step = StepExistingJSONFilePicker
			return m, m.jsonFilePicker.Init()
		case "up", "k":
			if m.assessmentCursor > 0 {
				m.assessmentCursor--
			}
		case "down", "j":
			if m.assessmentCursor < len(m.assessmentItems)-1 {
				m.assessmentCursor++
			}
		case "tab":
			m.selectedAssessmentIdx = m.assessmentCursor
			m.step = StepCSVPreview
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewAssessmentSelect() string {
	var b strings.Builder
	b.WriteString(m.header("Select the Assessment to Update"))
	b.WriteString(guide(
		"Your download holds every audit assessment in the workspace. Pick the",
		"one this CSV belongs to. The output file will contain only that",
		"assessment, so importing it doesn't duplicate the others.",
	))

	for i, item := range m.assessmentItems {
		praCount := len(m.existingData[i].ProgramRequirementAssessments)
		if i == m.assessmentCursor {
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				cursorStyle.Render(">"),
				selectedStyle.Render(item),
				dimStyle.Render(fmt.Sprintf("(%d requirements)", praCount)),
			))
			if praCount == 0 {
				b.WriteString("      " + warningStyle.Render("No requirements: every CSV row would be added as new.") + "\n")
			}
		} else {
			b.WriteString(fmt.Sprintf("    %s %s\n",
				item,
				dimStyle.Render(fmt.Sprintf("(%d requirements)", praCount)),
			))
		}
	}

	b.WriteString(helpStyle.Render("\n  ↑/↓ choose • tab continue • esc back"))
	return b.String()
}
