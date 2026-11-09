package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// On-screen guidance shared by every step, so users don't need the README.

var (
	stepCounterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9CA3AF"))

	guideStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("#7C3AED")).
			PaddingLeft(1).
			MarginLeft(2)

	tipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA"))

	suggestedTagStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#60A5FA")).
				Italic(true)
)

// flowSteps lists the screens a user moves through, in order.
func (m *Model) flowSteps() []Step {
	return []Step{StepWelcome, StepCSVFilePicker, StepExistingJSONFilePicker, StepAssessmentSelect,
		StepCSVPreview, StepIDColumnSelect, StepIDPrefixStrip, StepColumnMapping, StepMetadata,
		StepOutputPath, StepReview}
}

// header renders the step counter and title shown at the top of each screen.
func (m *Model) header(title string) string {
	steps := m.flowSteps()
	if m.step == StepWelcome {
		// The step count depends on the mode chosen here
		return titleStyle.Render(title) + "\n"
	}
	for i, s := range steps {
		if s == m.step {
			counter := stepCounterStyle.Render(fmt.Sprintf("Step %d of %d", i+1, len(steps)))
			return counter + "  " + titleStyle.Render(title) + "\n"
		}
	}
	return titleStyle.Render(title) + "\n"
}

// guide renders the "what this step does" panel. Each argument is one line.
func guide(lines ...string) string {
	return guideStyle.Render(strings.Join(lines, "\n")) + "\n\n"
}

// tip renders a single hint line.
func tip(text string) string {
	return "  " + tipStyle.Render("Tip: "+text) + "\n"
}

// suggestedTag marks an option the tool picked automatically.
func suggestedTag() string {
	return suggestedTagStyle.Render("(suggested)")
}
