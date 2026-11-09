package main

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newMappingModel returns a model on the column mapping screen with column 0 as the ID.
func newMappingModel(headers ...string) *Model {
	m := NewModel()
	m.csvHeaders = headers
	m.csvRows = [][]string{make([]string, len(headers))}
	m.idColumnIdx = 0
	m.columnMappings = make([]MappingTarget, len(headers))
	m.columnMappings[0] = MapID
	for i := 1; i < len(headers); i++ {
		m.mappingColumns = append(m.mappingColumns, i)
	}
	m.step = StepColumnMapping
	return m
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		}
		m.Update(msg)
	}
}

// pick moves the target cursor to t and confirms it.
func pick(m *Model, t MappingTarget) {
	for m.mappingTargetCursor > 0 {
		press(m, "up")
	}
	for i := 0; i < targetIndex(t); i++ {
		press(m, "down")
	}
	press(m, "enter")
}

func TestColumnMapping_GoingBackKeepsLaterChoices(t *testing.T) {
	m := newMappingModel("ID", "Requirement", "Overview", "Gaps")
	pick(m, MapSkip)       // Requirement
	pick(m, MapResult)     // Overview
	press(m, "esc", "esc") // back to Requirement
	pick(m, MapSkip)       // re-confirm Requirement

	// Now on Overview: the cursor should be on its saved choice, not skip
	if m.mappingCursor != 1 || m.mappingTargetCursor != targetIndex(MapResult) {
		t.Fatalf("expected cursor on saved 'result' for Overview, got column %d target %d", m.mappingCursor, m.mappingTargetCursor)
	}
	press(m, "enter") // accept it unchanged
	if m.columnMappings[2] != MapResult {
		t.Errorf("Overview should still map to result, got %v", m.columnMappings[2])
	}
}

func TestColumnMapping_BackFromMetadataRestoresCursor(t *testing.T) {
	m := newMappingModel("ID", "Overview", "Gaps")
	pick(m, MapResult)
	pick(m, MapRemediation)
	press(m, "tab")
	if m.step != StepMetadata {
		t.Fatalf("expected metadata step, got %v", m.step)
	}
	press(m, "esc")
	if m.step != StepColumnMapping || m.mappingTargetCursor != targetIndex(MapRemediation) {
		t.Errorf("expected cursor on saved 'remediation', got step %v target %d", m.step, m.mappingTargetCursor)
	}
}

func TestColumnMapping_PlanStacksButRemediationIsSingle(t *testing.T) {
	m := newMappingModel("ID", "Plan A", "Plan B", "Gaps 1", "Gaps 2", "Notes 1", "Notes 2")
	pick(m, MapPlan)
	pick(m, MapPlan) // a second plan column stacks, no warning
	pick(m, MapRemediation)
	// Moving the cursor onto remediation for the second column warns which column will be replaced
	for i := 0; i < targetIndex(MapRemediation); i++ {
		press(m, "down")
	}
	if view := m.viewColumnMapping(); !strings.Contains(view, `replaces "Gaps 1"`) {
		t.Errorf("expected replacement warning in view:\n%s", view)
	}
	press(m, "enter")
	pick(m, MapResult)
	pick(m, MapResult)

	want := []MappingTarget{MapID, MapPlan, MapPlan, MapSkip, MapRemediation, MapResult, MapResult}
	for i, w := range want {
		if m.columnMappings[i] != w {
			t.Errorf("column %q: expected %v, got %v", m.csvHeaders[i], w, m.columnMappings[i])
		}
	}
}

func TestColumnMapping_TabWarnsOnceAboutUnreviewedColumns(t *testing.T) {
	m := newMappingModel("ID", "Notes", "Gaps", "More")
	press(m, "tab") // two columns after this one have no checked suggestion
	if m.step != StepColumnMapping || !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "haven't been reviewed") {
		t.Fatal("first tab should warn and stay")
	}
	press(m, "tab")
	if m.step != StepMetadata {
		t.Error("second tab should continue")
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestColumnMapping_DescriptionsAligned(t *testing.T) {
	m := newMappingModel("ID", "Notes")
	for cursor := range AllMappingTargets() {
		m.mappingTargetCursor = cursor
		view := ansi.ReplaceAllString(m.viewColumnMapping(), "")
		col := -1
		for _, line := range strings.Split(view, "\n") {
			for _, desc := range []string{"Ignore this column", "Assessment plan field", "Result field", "Remediation field"} {
				if i := strings.Index(line, desc); i >= 0 {
					if col == -1 {
						col = i
					} else if i != col {
						t.Errorf("cursor %d: %q starts at column %d, expected %d\n%s", cursor, desc, i, col, view)
					}
				}
			}
		}
	}
}
