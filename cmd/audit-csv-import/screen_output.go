package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The "Where to Save" screen splits the output path into a folder and a file
// name, each wide enough to show in full, with the complete path below.

const (
	outputFieldFolder = iota
	outputFieldName
)

func (m *Model) initOutputPath() {
	folder := textinput.New()
	folder.CharLimit = 1024
	folder.Prompt = ""
	name := textinput.New()
	name.CharLimit = 255
	name.Prompt = ""

	m.outputFolderInput, m.outputNameInput = folder, name
	m.setOutputPath(m.defaultOutputPath(time.Now()))
	m.outputPathEmpty = false
	m.focusOutputField(outputFieldName) // the name is what people usually change
}

// setOutputPath fills the folder and file name fields from a full path.
func (m *Model) setOutputPath(path string) {
	if path == "" {
		m.outputFolderInput.SetValue("")
		m.outputNameInput.SetValue("")
		return
	}
	m.outputFolderInput.SetValue(filepath.Dir(path))
	m.outputNameInput.SetValue(filepath.Base(path))
	m.outputNameInput.CursorEnd()
	m.outputFolderInput.CursorEnd()
}

// outputPath joins the two fields, adding ".json" if the name has no extension.
func (m *Model) outputPath() string {
	folder := strings.TrimSpace(m.outputFolderInput.Value())
	name := strings.TrimSpace(m.outputNameInput.Value())
	if name == "" {
		return ""
	}
	if strings.HasPrefix(folder, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			folder = filepath.Join(home, folder[2:])
		}
	}
	if filepath.Ext(name) == "" {
		name += ".json"
	}
	return filepath.Join(folder, name)
}

func (m *Model) focusOutputField(field int) {
	m.outputField = field
	if field == outputFieldFolder {
		m.outputNameInput.Blur()
		m.outputFolderInput.Focus()
	} else {
		m.outputFolderInput.Blur()
		m.outputNameInput.Focus()
	}
}

// defaultOutputPath suggests a file name that can't be mistaken for the
// download or an earlier run: the download's name plus "-merged" and the
// date and time, written next to the download.
func (m *Model) defaultOutputPath(now time.Time) string {
	stamp := now.Format("2006-01-02-1504")
	if m.jsonPath != "" {
		ext := filepath.Ext(m.jsonPath)
		return strings.TrimSuffix(m.jsonPath, ext) + "-merged-" + stamp + ext
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "assessment-"+stamp+".json")
}

func (m *Model) updateOutputPath(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			m.step = StepMetadata
			return m, nil
		case "tab":
			path := m.outputPath()
			if path == "" {
				m.outputPathEmpty = true
				return m, nil
			}
			if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
				return m, nil // the screen already says the folder doesn't exist
			}
			m.outputPathEmpty = false
			m.outputFilePath = path
			m.step = StepReview
			return m, nil
		case "up", "down", "enter":
			// Move between the two fields; only tab leaves the step
			if m.outputField == outputFieldFolder {
				m.focusOutputField(outputFieldName)
			} else {
				m.focusOutputField(outputFieldFolder)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.outputField == outputFieldFolder {
		m.outputFolderInput, cmd = m.outputFolderInput.Update(msg)
	} else {
		m.outputNameInput, cmd = m.outputNameInput.Update(msg)
	}
	return m, cmd
}

func (m *Model) viewOutputPath() string {
	var b strings.Builder
	b.WriteString(m.header("Where to Save"))
	b.WriteString(guide(
		"The merged file is saved as a new file next to your download, named",
		"with today's date and time so it's never confused with the download",
		"or an earlier run. This is the file you'll import into Paramify.",
		"Press tab to accept, or change the file name or folder first.",
	))

	// Make both fields as wide as the screen allows, so nothing scrolls out of view
	w := m.contentWidth() - 22
	if w < 30 {
		w = 30
	}
	m.outputFolderInput.Width = w
	m.outputNameInput.Width = w

	row := func(field int, label string, input textinput.Model) {
		prefix := "  "
		if m.outputField == field {
			prefix = cursorStyle.Render("> ")
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, labelStyle.Render(label), input.View()))
	}
	row(outputFieldName, "File name:", m.outputNameInput)
	row(outputFieldFolder, "Folder:", m.outputFolderInput)

	// The full path, wrapped rather than cut off
	path := m.outputPath()
	b.WriteString("\n  " + dimStyle.Render("Will save to:") + "\n")
	if path == "" {
		b.WriteString("  " + dimStyle.Render("(type a file name)") + "\n")
	} else {
		wrapped := lipgloss.NewStyle().Width(m.contentWidth() - 6).Render(path)
		for _, line := range strings.Split(wrapped, "\n") {
			b.WriteString("    " + selectedStyle.Render(line) + "\n")
		}
	}

	switch {
	case m.outputPathEmpty && path == "":
		b.WriteString("\n  " + warningStyle.Render("Type a file name to continue.") + "\n")
	case path == "":
	case !dirExists(filepath.Dir(path)):
		b.WriteString("\n  " + warningStyle.Render("That folder doesn't exist. Fix the folder to continue.") + "\n")
	case path == m.jsonPath:
		b.WriteString("\n  " + warningStyle.Render("This is your original download. Saving here replaces it, with no backup.") + "\n")
	case fileExists(path):
		b.WriteString("\n  " + warningStyle.Render("A file with this name already exists and will be replaced.") + "\n")
	}
	b.WriteString(helpStyle.Render("\n  ↑/↓ or enter switch between file name and folder • tab continue • esc back"))
	return b.String()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
