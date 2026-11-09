package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "tab" {
		m.step = StepCSVFilePicker
		return m, m.csvFilePicker.Init()
	}
	return m, nil
}

func (m *Model) viewWelcome() string {
	var b strings.Builder

	b.WriteString(m.header("Audit CSV Import"))
	b.WriteString(guide(
		"Turns an auditor's spreadsheet (CSV) into a file you can import into",
		"Paramify's Audit Assessments. Each CSV row fills in one requirement's",
		"capability assurance step: its plan, result, and remediation.",
		"",
		"You'll need:",
		"  • The auditor's spreadsheet saved as CSV (Excel: File → Save As → CSV)",
		"  • Your audit assessments downloaded from Paramify",
		"    (Audit Assessments → Download audit assessments → audit-assessments.json)",
	))

	b.WriteString("  The tool updates one assessment from your download. Its name and program\n")
	b.WriteString("  are kept exactly as Paramify has them, and only that assessment goes into\n")
	b.WriteString("  the output file.\n\n")
	b.WriteString(tip("starting a brand-new assessment? Create it in Paramify first, then download and use it here."))

	b.WriteString(helpStyle.Render("\n  tab start • q quit (ctrl+c quits from any screen)"))

	return b.String()
}
