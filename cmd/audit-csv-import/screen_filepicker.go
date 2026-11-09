package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
)

type csvParsedMsg struct {
	data *CSVData
	err  error
}

type jsonLoadedMsg struct {
	data []Assessment
	err  error
}

func newFilePicker(allowedTypes []string) filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = allowedTypes
	fp.ShowHidden = false
	fp.AutoHeight = false
	fp.Height = 15
	fp.CurrentDirectory = startDirectory()
	return fp
}

// startDirectory is where the file pickers open: the repo's example_files
// folder (where your own files go), else the demo folder, else the current
// folder, else the home folder.
func startDirectory() string {
	if cwd, err := os.Getwd(); err == nil {
		for _, dir := range []string{"example_files", "demo"} {
			if info, err := os.Stat(filepath.Join(cwd, dir)); err == nil && info.IsDir() {
				return filepath.Join(cwd, dir)
			}
		}
		return cwd
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

func parseCSVCmd(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := ParseCSV(path)
		return csvParsedMsg{data: data, err: err}
	}
}

func loadJSONCmd(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := LoadAssessments(path)
		return jsonLoadedMsg{data: data, err: err}
	}
}

func (m *Model) updateCSVFilePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// esc goes back a step; ←/backspace go up a folder (handled by the picker).
		// Only tab continues: on a file it moves on, on a folder it opens it.
		// enter opens folders but never leaves the step.
		m.pickerHint = ""
		fromTab := msg.String() == "tab"
		switch msg.String() {
		case "esc":
			m.step = StepWelcome
			return m, nil
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		var cmd tea.Cmd
		m.csvFilePicker, cmd = m.csvFilePicker.Update(msg)
		if didSelect, path := m.csvFilePicker.DidSelectFile(msg); didSelect {
			if !fromTab {
				m.pickerHint = "Press tab to continue with " + filepath.Base(path) + "."
				return m, cmd
			}
			m.csvPath = path
			return m, parseCSVCmd(path)
		}
		return m, cmd
	case csvParsedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.errReturn = StepCSVFilePicker
			m.step = StepError
			return m, nil
		}
		m.csvHeaders = msg.data.Headers
		m.csvRows = msg.data.Rows
		m.columnMappings = make([]MappingTarget, len(m.csvHeaders))
		// The download is usually kept next to the CSV
		m.jsonFilePicker.CurrentDirectory = filepath.Dir(m.csvPath)
		m.step = StepExistingJSONFilePicker
		return m, m.jsonFilePicker.Init()
	}

	var cmd tea.Cmd
	m.csvFilePicker, cmd = m.csvFilePicker.Update(msg)

	return m, cmd
}

func (m *Model) viewCSVFilePicker() string {
	var b strings.Builder
	b.WriteString(m.header("Select the Auditor's CSV"))
	b.WriteString(guide(
		"Pick the spreadsheet with the auditor's notes, one row per requirement.",
		"Only .csv files can be selected. Excel files show greyed out: open them",
		"in Excel and use File → Save As → CSV UTF-8, then come back here.",
	))
	b.WriteString(dimStyle.Render("  Folder: "+m.csvFilePicker.CurrentDirectory) + "\n")
	b.WriteString(m.csvFilePicker.View())
	b.WriteString(m.pickerHintView())
	b.WriteString(helpStyle.Render("\n  ↑/↓ move • tab continue with the highlighted file (or open a folder) • ← up a folder • esc previous step • ctrl+c quit"))
	return b.String()
}

func (m *Model) updateJSONFilePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.pickerHint = ""
		fromTab := msg.String() == "tab"
		switch msg.String() {
		case "esc":
			m.step = StepCSVFilePicker
			return m, m.csvFilePicker.Init()
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		}
		var cmd tea.Cmd
		m.jsonFilePicker, cmd = m.jsonFilePicker.Update(msg)
		if didSelect, path := m.jsonFilePicker.DidSelectFile(msg); didSelect {
			if !fromTab {
				m.pickerHint = "Press tab to continue with " + filepath.Base(path) + "."
				return m, cmd
			}
			m.jsonPath = path
			return m, loadJSONCmd(path)
		}
		return m, cmd
	case jsonLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.errReturn = StepExistingJSONFilePicker
			m.step = StepError
			return m, nil
		}
		m.existingData = msg.data
		m.assessmentItems = make([]string, len(msg.data))
		for i, a := range msg.data {
			m.assessmentItems[i] = fmt.Sprintf("%s (%s)", a.Name, a.Program)
		}
		m.step = StepAssessmentSelect
		return m, nil
	}

	var cmd tea.Cmd
	m.jsonFilePicker, cmd = m.jsonFilePicker.Update(msg)

	return m, cmd
}

func (m *Model) viewJSONFilePicker() string {
	var b strings.Builder
	b.WriteString(m.header("Select Your Paramify Download"))
	b.WriteString(guide(
		"Pick the audit assessments file downloaded from Paramify",
		"(Audit Assessments → Download audit assessments). It's usually",
		"named audit-assessments.json. The tool won't change this file; the one",
		"assessment you update is saved as a new file at the end.",
	))
	b.WriteString(dimStyle.Render("  Folder: "+m.jsonFilePicker.CurrentDirectory) + "\n")
	b.WriteString(m.jsonFilePicker.View())
	b.WriteString(m.pickerHintView())
	b.WriteString(helpStyle.Render("\n  ↑/↓ move • tab continue with the highlighted file (or open a folder) • ← up a folder • esc previous step • ctrl+c quit"))
	return b.String()
}

// pickerHintView reminds the user that only tab continues, after enter on a file.
func (m *Model) pickerHintView() string {
	if m.pickerHint == "" {
		return ""
	}
	return "\n  " + warningStyle.Render(m.pickerHint)
}
