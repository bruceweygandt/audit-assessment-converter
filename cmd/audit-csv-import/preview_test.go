package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var stripANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// newPreviewModel is a merge-mode model on the mapping screen with a wide terminal.
func newPreviewModel() *Model {
	m := newMappingModel("ID", "Notes", "Gaps")
	m.csvRows = [][]string{{"KSI-A-1", "Looks good", "Fix it"}}
	m.idPrefixes = []string{"KSI-"}
	m.existingData = []Assessment{{Name: "Test", ProgramRequirementAssessments: []ProgramRequirementAssessment{{
		ProgramRequirement: "A-1", Status: "OPEN", Owner: "owner@x.com",
		AssuranceSteps: []AssuranceStep{{Plan: TextToParagraphs("Old plan"), Result: TextToParagraphs("Old result"), Remediation: []byte("null")}},
	}}}}
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 60})
	return m
}

func TestPreview_FollowsHighlightedChoice(t *testing.T) {
	m := newPreviewModel()
	for i := 0; i < targetIndex(MapResult); i++ {
		press(m, "down") // hover "result" for the Notes column, without confirming
	}
	view := stripANSI.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"Live Preview",
		"CSV · ", "1 rows × 3 columns", // the grid
		"→ result",                     // the mapping row under the header
		"▶2",                           // spreadsheet row number of the previewed row
		"JSON · ", "A-1 · changes vs.", // the JSON entry, diffed against the download
		`- `, `"text": "Old result"`, // removed line
		`+ `, `"text": "Looks good"`, // added line
		`▶ +`, // the result lines are lit up
	} {
		if !strings.Contains(view, want) {
			t.Errorf("preview missing %q:\n%s", want, view)
		}
	}
	if m.columnMappings[1] != MapSkip {
		t.Error("hovering must not change the saved mapping")
	}
}

func TestRequirementJSON_TagsFields(t *testing.T) {
	method := "TEST"
	p := ProgramRequirementAssessment{
		ProgramRequirement: "A-1", Status: "OPEN",
		AssuranceSteps: []AssuranceStep{
			{Method: &method, Status: "PASS", Plan: TextToParagraphs("p"), Result: TextToParagraphs("r"), Remediation: []byte("null")},
			{Status: "FAIL", Remediation: []byte("null")},
		},
	}
	got := map[string]string{}
	for _, l := range requirementJSON(p) {
		got[strings.TrimSpace(l.text)] = l.field
	}
	checks := map[string]string{
		`"status": "OPEN",`: fieldStatus,
		`"status": "PASS",`: fieldStepStatus,
		`"method": "TEST",`: fieldMethod,
		`"text": "r"`:       fieldResult,
		`"status": "FAIL",`: "", // second step: not previewed
	}
	for line, want := range checks {
		if got[line] != want {
			t.Errorf("%s tagged %q, want %q", line, got[line], want)
		}
	}
}

func TestDiffLines(t *testing.T) {
	a := []jsonLine{{"a", ""}, {"b", ""}, {"c", ""}}
	b := []jsonLine{{"a", ""}, {"x", ""}, {"c", ""}}
	var got strings.Builder
	for _, op := range diffLines(a, b) {
		got.WriteString(string(op.op) + op.line.text + ",")
	}
	if got.String() != " a,-b,+x, c," {
		t.Errorf("unexpected diff %q", got.String())
	}
}

func TestPreview_DoesNotChangeTheDownload(t *testing.T) {
	m := newPreviewModel()
	m.columnMappings[1] = MapResult
	m.View()
	got := m.existingData[0].ProgramRequirementAssessments[0].AssuranceSteps[0]
	if plainRich(got.Result) != "Old result" {
		t.Errorf("rendering the preview modified the loaded assessment: %q", plainRich(got.Result))
	}
}

func TestPreview_HiddenWhenNarrowOrToggled(t *testing.T) {
	m := newPreviewModel()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	view := stripANSI.ReplaceAllString(m.View(), "")
	if strings.Contains(view, "Live Preview") || !strings.Contains(view, "Widen the window") {
		t.Error("narrow terminals should hide the preview and say how to see it")
	}
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 60})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "Live Preview") {
		t.Error("ctrl+p should hide the preview")
	}
}

func TestPreview_RowKeysDontStealTypedBrackets(t *testing.T) {
	m := newPreviewModel()
	m.csvRows = append(m.csvRows, []string{"KSI-A-2", "", ""})
	press(m, "enter", "enter", "tab") // map both columns, tab → details
	if m.step != StepMetadata {
		t.Fatalf("expected details step, got %v", m.step)
	}
	m.focusMetadata(metaOwner)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	if m.metadata.inputs[metaOwner].Value() != "]" || m.previewRow != 0 {
		t.Error("] typed in a text field should go into the field, not change the row")
	}
	m.focusMetadata(metaStatus)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	if m.previewRow != 1 {
		t.Error("] on a non-text field should move to the next preview row")
	}
}

func TestAudit_KeysMatchHelp(t *testing.T) {
	// q no longer quits mid-flow (it would lose all work); ctrl+c does
	m := newPreviewModel()
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd != nil {
		t.Error("q on the mapping screen should not quit")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c should quit")
	}

	// Details: tab goes straight to "Where to save" (no separate plan/result steps)
	m = newPreviewModel()
	press(m, "enter", "enter", "tab") // → details
	press(m, "tab")
	if m.step != StepOutputPath {
		t.Fatalf("tab should continue from details to where-to-save, got %v", m.step)
	}
	press(m, "esc")
	if m.step != StepMetadata {
		t.Errorf("esc from where-to-save should return to details, got %v", m.step)
	}
	press(m, "tab")

	// Empty output path: tab stays and explains
	m.setOutputPath("")
	press(m, "tab")
	if m.step != StepOutputPath || !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "Type a file name") {
		t.Error("tab on an empty path should stay and ask for a path")
	}
}

func TestAudit_ErrorReturnsToFailedStep(t *testing.T) {
	m := newPreviewModel()
	m.step = StepReview
	m.Update(generateDoneMsg{err: errTest})
	if m.step != StepError {
		t.Fatal("expected error screen")
	}
	press(m, "esc")
	if m.step != StepReview {
		t.Errorf("esc after a write error should return to Review, got %v", m.step)
	}

	m.step = StepCSVFilePicker
	m.Update(csvParsedMsg{err: errTest})
	press(m, "esc")
	if m.step != StepCSVFilePicker {
		t.Errorf("esc after a CSV error should return to the CSV picker, got %v", m.step)
	}
}

func TestAudit_PreviewHiddenHint(t *testing.T) {
	m := newPreviewModel()
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "ctrl+p shows it") {
		t.Error("hidden preview should say how to show it again")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "Live Preview") {
		t.Error("ctrl+p again should show the preview")
	}
}

var errTest = fmt.Errorf("boom")
