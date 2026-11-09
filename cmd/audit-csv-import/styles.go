package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	activeFieldStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7C3AED")).
				Bold(true)

	labelStyle = lipgloss.NewStyle().
			Width(17).
			Foreground(lipgloss.Color("#D1D5DB"))

	mappingTagStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)
)

func mappingColor(m MappingTarget) lipgloss.Style {
	switch m {
	case MapPlan:
		return mappingTagStyle.Foreground(lipgloss.Color("#3B82F6"))
	case MapResult:
		return mappingTagStyle.Foreground(lipgloss.Color("#10B981"))
	case MapRemediation:
		return mappingTagStyle.Foreground(lipgloss.Color("#F59E0B"))
	case MapStatus, MapMethod, MapStepStatus:
		return mappingTagStyle.Foreground(lipgloss.Color("#A78BFA"))
	case MapAssigned, MapOwner, MapReviewer:
		return mappingTagStyle.Foreground(lipgloss.Color("#22D3EE"))
	default:
		return mappingTagStyle.Foreground(lipgloss.Color("#6B7280"))
	}
}
