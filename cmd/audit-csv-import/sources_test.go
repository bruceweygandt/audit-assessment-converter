package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newDetailsModel returns a model on the details screen for a CSV whose
// column names don't hint at what they hold.
func newDetailsModel() *Model {
	m := newMappingModel("ID", "Col A", "Col B", "Col C")
	m.csvRows = [][]string{{"R-1", "Interview", "jane@x.com", "Approach text"}}
	m.columnMappings[3] = MapResult
	m.initMetadata()
	m.step = StepMetadata
	return m
}

func TestDetails_PickColumnForField(t *testing.T) {
	m := newDetailsModel()
	m.focusMetadata(metaMethodSrc)

	press(m, "right") // none → Col A
	if m.targetColumn(MapMethod) != 1 {
		t.Fatalf("expected Col A to feed method, got column %d", m.targetColumn(MapMethod))
	}
	view := m.viewMetadata()
	if !strings.Contains(view, `"Col A"`) || !strings.Contains(view, "Values in this column: Interview") {
		t.Errorf("view should show the picked column and its values:\n%s", view)
	}

	press(m, "left") // back to none
	if m.targetColumn(MapMethod) != -1 {
		t.Errorf("expected method to have no column, got %d", m.targetColumn(MapMethod))
	}
}

func TestDetails_PickingAColumnUsedElsewhereSwitchesIt(t *testing.T) {
	m := newDetailsModel()
	m.focusMetadata(metaOwnerSrc)
	press(m, "left") // none → last column (Col C, currently result)
	if m.columnMappings[3] != MapOwner {
		t.Fatalf("Col C should now feed owner, got %v", m.columnMappings[3])
	}
	if !strings.Contains(m.metadata.sourceNote, `"Col C" was mapped to result`) {
		t.Errorf("expected a note about the switch, got %q", m.metadata.sourceNote)
	}
}

func TestDetails_EnterKeepsTypedValuesWhenComingBack(t *testing.T) {
	m := newDetailsModel()
	m.focusMetadata(metaOwner)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("team-a")})
	press(m, "tab")
	if m.ownerEmail != "team-a" || m.step != StepOutputPath {
		t.Fatalf("expected owner saved and where-to-save step, got %q / %v", m.ownerEmail, m.step)
	}
	press(m, "esc")
	m.initMetadata()
	if got := m.metadata.inputs[metaOwner].Value(); got != "team-a" {
		t.Errorf("owner should survive going back, got %q", got)
	}
}

func TestDetails_PlanGroupUsesStep8ColumnsThenTypedText(t *testing.T) {
	m := newMappingModel("ID", "Approach", "Procedure", "Notes")
	m.csvRows = [][]string{
		{"A-1", "Reviewed configs", "Ran validator", "x"},
		{"A-2", "", "", "y"},
	}
	m.columnMappings[1] = MapPlan // both mapped to plan on step 8
	m.columnMappings[2] = MapPlan
	m.initMetadata()
	m.step = StepMetadata

	view := stripANSI.ReplaceAllString(m.viewMetadata(), "")
	for _, want := range []string{
		"From columns:", `"Approach" + "Procedure"`, "1 row read from these columns",
		"If blank:", "1 row keeps its current plan",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("details view missing %q:\n%s", want, view)
		}
	}

	// Type a fallback plan and continue
	m.focusMetadata(metaPlanText)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Examined evidence")})
	if !strings.Contains(stripANSI.ReplaceAllString(m.viewMetadata(), ""), "1 row uses this text") {
		t.Error("typing a plan should update the count")
	}
	press(m, "tab")
	if m.planText != "Examined evidence" || m.step != StepOutputPath {
		t.Fatalf("expected plan text saved and where-to-save step, got %q / %v", m.planText, m.step)
	}

	// The written plan: stacked columns, then the typed text for the blank row
	pras := GenerateAssessment(&CSVData{Headers: m.csvHeaders, Rows: m.csvRows}, m.generateConfig()).ProgramRequirementAssessments
	if got := plainRich(pras[0].AssuranceSteps[0].Plan); got != "Approach / Reviewed configs / Procedure / Ran validator" {
		t.Errorf("row 1 plan = %q", got)
	}
	if got := plainRich(pras[1].AssuranceSteps[0].Plan); got != "Examined evidence" {
		t.Errorf("row 2 plan = %q", got)
	}

	// Coming back keeps the typed text
	press(m, "esc")
	m.initMetadata()
	if got := m.metadata.inputs[metaPlanText].Value(); got != "Examined evidence" {
		t.Errorf("plan text should survive going back, got %q", got)
	}
}

func TestDetails_BrowsingPastColumnsDoesNotChangeThem(t *testing.T) {
	m := newDetailsModel()
	m.columnMappings[1] = MapMethod // Col A feeds method
	// Col C feeds result (from newDetailsModel)
	m.focusMetadata(metaOwnerSrc)

	press(m, "right", "right") // none → Col A (borrowed from method) → Col B
	if m.columnMappings[1] != MapMethod {
		t.Errorf("Col A should be back on method after browsing past it, got %v", m.columnMappings[1])
	}
	if m.columnMappings[2] != MapOwner {
		t.Fatalf("Col B should feed owner, got %v", m.columnMappings[2])
	}
	press(m, "right", "right") // → Col C (borrowed from result) → none
	if m.columnMappings[3] != MapResult {
		t.Errorf("Col C should be back on result, got %v", m.columnMappings[3])
	}
	if m.targetColumn(MapOwner) != -1 {
		t.Errorf("owner should have no column, got %d", m.targetColumn(MapOwner))
	}
}

func TestDetails_NameAndProgramComeFromTheDownload(t *testing.T) {
	m := newDetailsModel()
	m.existingData = []Assessment{{Name: "SOC 2 Audit", Target: "SOC 2 AICPA", Program: "SOC 2 AICPA"}}
	m.initMetadata()
	view := stripANSI.ReplaceAllString(m.viewMetadata(), "")
	if !strings.Contains(view, "Assessment:") || !strings.Contains(view, "SOC 2 Audit") || !strings.Contains(view, "program SOC 2 AICPA (from your download)") {
		t.Errorf("details should show the assessment and program from the download:\n%s", view)
	}
	for _, gone := range []string{"Name:", "Target:", "Required:"} {
		if strings.Contains(view, gone) {
			t.Errorf("details should no longer ask for %q:\n%s", gone, view)
		}
	}
	press(m, "tab")
	if m.step != StepOutputPath {
		t.Errorf("tab should continue without asking for a name or program, got %v", m.step)
	}
}

func TestDetails_CountsShowColumnFirst(t *testing.T) {
	m := newMappingModel("ID", "Review", "Who", "Name")
	m.csvRows = [][]string{
		{"A-1", "Accepted", "Client Success", "Configure all resources to limit traffic"},
		{"A-2", "", "", "Use immutable infrastructure with strict privileges"},
		{"A-3", "weird", "jane@x.com", "Apply zero trust design principles across systems"},
	}
	m.columnMappings[1] = MapStatus
	m.columnMappings[2] = MapOwner
	m.initMetadata()
	m.step = StepMetadata

	c := m.countField(MapStatus, ParseRequirementStatus)
	if c.fromColumn != 1 || c.blank != 1 || c.unreadable != 1 {
		t.Errorf("status counts: %+v", c)
	}
	view := stripANSI.ReplaceAllString(m.viewMetadata(), "")
	for _, want := range []string{
		"From column:", "1 row read from this column", // status: column first
		"If blank:", `2 rows use OPEN (1 blank, 1 not recognized)`,
		"Every row:", "all 3 rows use EXAMINE", // method has no column
		`2 rows read from this column`, "1 row keeps its current assurance owner (1 blank)", // owner, empty value
	} {
		if !strings.Contains(view, want) {
			t.Errorf("details view missing %q:\n%s", want, view)
		}
	}
	// "From column" comes before the value it overrides
	if strings.Index(view, "From column:") > strings.Index(view, "If blank:") {
		t.Error("From column should be shown before If blank")
	}

	// Pointing a people field at prose warns
	if w := m.peopleColumnWarning(MapOwner); w != "" {
		t.Errorf("names shouldn't warn: %s", w)
	}
	m.setTargetColumn(MapOwner, 3)
	if w := m.peopleColumnWarning(MapOwner); !strings.Contains(w, `"Name" looks like text, not people`) {
		t.Errorf("prose column should warn, got %q", w)
	}
	if !strings.Contains(stripANSI.ReplaceAllString(m.viewMetadata(), ""), "looks like text, not people") {
		t.Error("the warning should show on the details screen")
	}
}

func TestResult_ColumnsMappedOnStep8StackInCSVOrder(t *testing.T) {
	// Mapping two columns to result on step 8 is all it takes
	m := newMappingModel("ID", "Overview", "Notes", "Gaps")
	m.csvRows = [][]string{{"A-1", "Looks good", "n", "Fix it"}}
	pick(m, MapResult) // Overview
	pick(m, MapSkip)   // Notes
	pick(m, MapResult) // Gaps
	if m.columnMappings[1] != MapResult || m.columnMappings[3] != MapResult {
		t.Fatalf("both columns should feed result: %v", m.columnMappings)
	}
	pra := GenerateAssessment(&CSVData{Headers: m.csvHeaders, Rows: m.csvRows},
		GenerateConfig{IDColumnIdx: 0, ColumnMappings: m.columnMappings}).ProgramRequirementAssessments[0]
	if got := plainRich(pra.AssuranceSteps[0].Result); got != "Overview / Looks good / Gaps / Fix it" {
		t.Errorf("result = %q", got)
	}
	if note := m.stackOrderNote(MapResult); !strings.Contains(note, "Overview → Gaps") {
		t.Errorf("order note = %q", note)
	}
}
