package main

import (
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the top-level Bubbletea model.
type Model struct {
	// Navigation
	step Step

	// Screen sub-models
	csvFilePicker     filepicker.Model
	jsonFilePicker    filepicker.Model
	metadata          metadataModel
	previewScroll     int                   // first column shown on the CSV preview screen
	mappingWarned     bool                  // tab on the mapping screen already warned about unreviewed columns
	pickerHint        string                // shown when enter is pressed on a file: only tab continues
	borrowed          map[int]MappingTarget // columns a field is browsing over → their earlier mapping
	outputFolderInput textinput.Model
	outputNameInput   textinput.Model
	outputField       int // which of the two fields has focus

	// CSV data
	csvPath    string
	csvHeaders []string
	csvRows    [][]string

	// The Paramify download
	jsonPath              string
	existingData          []Assessment
	assessmentItems       []string
	assessmentCursor      int
	selectedAssessmentIdx int

	// ID column selection
	idColumnIdx       int
	idColumnCursor    int
	idColumnSuggested int
	suggestedPrefixes []string
	idPrefixes        []string // prefixes to strip from IDs (e.g. "FRR-", "KSI-")
	idPrefixInput     textinput.Model

	// Column mapping
	columnMappings      []MappingTarget
	suggestedMappings   []Suggestion
	mappingColumns      []int // indices of non-ID columns
	mappingCursor       int
	mappingTargetCursor int

	// Assessment metadata
	defaultStatus     string
	defaultMethod     string
	assignedEmail     string
	ownerEmail        string
	reviewerEmail     string
	planText          string
	defaultStepStatus string // step status for rows without one

	// Output
	outputFilePath  string
	outputPathEmpty bool // tab was pressed with no file name
	generatedPath   string
	mergeUpdated    []string
	mergeAdded      []string

	// Live preview pane
	previewRow    int  // CSV row shown in the preview
	previewHidden bool // toggled with ctrl+p

	errReturn Step // where esc goes from the error screen

	// Terminal
	width  int
	height int
	err    error
}

// NewModel creates a new Model with default values.
func NewModel() *Model {
	return &Model{
		step:           StepWelcome,
		csvFilePicker:  newFilePicker([]string{".csv"}),
		jsonFilePicker: newFilePicker([]string{".json"}),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keys and window sizing
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update filepicker heights
		h := msg.Height - 16 // room for the step header and guide panel
		if h < 5 {
			h = 5
		}
		m.csvFilePicker.Height = h
		m.jsonFilePicker.Height = h
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+p":
			if m.previewStep() {
				m.previewHidden = !m.previewHidden
				return m, nil
			}
		case "pgdown":
			if m.previewActive() {
				m.movePreviewRow(1)
				return m, nil
			}
		case "pgup":
			if m.previewActive() {
				m.movePreviewRow(-1)
				return m, nil
			}
		case "]", "[":
			if m.previewActive() && !m.typingText() {
				if msg.String() == "]" {
					m.movePreviewRow(1)
				} else {
					m.movePreviewRow(-1)
				}
				return m, nil
			}
		case "q":
			// q quits only where no work is lost; everywhere else it's ctrl+c
			switch m.step {
			case StepWelcome, StepDone, StepError:
				return m, tea.Quit
			}
		}
	// Handle generate done messages globally so they work during StepGenerating
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

	// Route to current step
	switch m.step {
	case StepWelcome:
		return m.updateWelcome(msg)
	case StepCSVFilePicker:
		return m.updateCSVFilePicker(msg)
	case StepExistingJSONFilePicker:
		return m.updateJSONFilePicker(msg)
	case StepAssessmentSelect:
		return m.updateAssessmentSelect(msg)
	case StepCSVPreview:
		return m.updateCSVPreview(msg)
	case StepIDColumnSelect:
		return m.updateIDColumnSelect(msg)
	case StepIDPrefixStrip:
		return m.updateIDPrefixStrip(msg)
	case StepColumnMapping:
		return m.updateColumnMapping(msg)
	case StepMetadata:
		return m.updateMetadata(msg)
	case StepOutputPath:
		return m.updateOutputPath(msg)
	case StepReview:
		return m.updateReview(msg)
	case StepDone:
		return m.updateDone(msg)
	case StepError:
		return m.updateError(msg)
	}
	return m, nil
}

func (m *Model) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "tab", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			m.err = nil
			m.step = m.errReturn
			switch m.step {
			case StepCSVFilePicker:
				return m, m.csvFilePicker.Init()
			case StepExistingJSONFilePicker:
				return m, m.jsonFilePicker.Init()
			}
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	return m.withPreview(m.viewScreen())
}

// viewScreen renders the current step (the left side when the preview is shown).
func (m *Model) viewScreen() string {
	switch m.step {
	case StepWelcome:
		return m.viewWelcome()
	case StepCSVFilePicker:
		return m.viewCSVFilePicker()
	case StepExistingJSONFilePicker:
		return m.viewJSONFilePicker()
	case StepAssessmentSelect:
		return m.viewAssessmentSelect()
	case StepCSVPreview:
		return m.viewCSVPreview()
	case StepIDColumnSelect:
		return m.viewIDColumnSelect()
	case StepIDPrefixStrip:
		return m.viewIDPrefixStrip()
	case StepColumnMapping:
		return m.viewColumnMapping()
	case StepMetadata:
		return m.viewMetadata()
	case StepOutputPath:
		return m.viewOutputPath()
	case StepReview:
		return m.viewReview()
	case StepGenerating:
		return m.viewGenerating()
	case StepDone:
		return m.viewDone()
	case StepError:
		return m.viewError()
	}
	return ""
}
