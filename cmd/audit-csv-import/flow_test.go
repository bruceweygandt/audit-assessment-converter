package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// runCmd runs a command synchronously and feeds its message back into the model.
func runCmd(m *Model, cmd tea.Cmd) {
	if cmd != nil {
		if msg := cmd(); msg != nil {
			switch msg.(type) {
			case csvParsedMsg, jsonLoadedMsg, generateDoneMsg:
				m.Update(msg)
			}
		}
	}
}

func tab(t *testing.T, m *Model, want Step) {
	t.Helper()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	runCmd(m, cmd)
	if m.step != want {
		t.Fatalf("after tab expected step %v, got %v (err %v)", want, m.step, m.err)
	}
}

// TestTabContinuesEveryScreen walks a whole merge with tab as the only
// "next step" key, on a CSV with 30 columns, and checks the written file.
func TestTabContinuesEveryScreen(t *testing.T) {
	dir := t.TempDir()
	headers := []string{"Indicator ID"}
	row := []string{"KSI-A-1"}
	for i := 1; i < 30; i++ {
		headers = append(headers, "Col "+string(rune('A'+i%26))+strings.Repeat("'", i/26))
		row = append(row, "v")
	}
	row[5], row[6], row[7] = "Looks good", "Examine", "Partial"
	csvPath := filepath.Join(dir, "audit.csv")
	os.WriteFile(csvPath, []byte(strings.Join(headers, ",")+"\n"+strings.Join(row, ",")+"\n"), 0644)
	jsonPath := filepath.Join(dir, "audit-assessments.json")
	os.WriteFile(jsonPath, []byte(`[{"name":"A","targetType":"PROGRAM","target":"P","program":"P","mechanism":"","cycleFrequency":"",
		"programRequirementAssessments":[{"programRequirement":"A-1","status":"OPEN","owner":"o","reviewer":"",
		"remarks":null,"assuranceSteps":[{"order":0,"method":"TEST","status":"OPEN","assigned":"x","plan":[],"result":[],"remediation":null}]}]}]`), 0644)

	m := NewModel()
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	tab(t, m, StepCSVFilePicker)

	// tab on a highlighted file continues straight away (the picker's own
	// file list isn't exposed, so feed the parsed message as the picker would)
	m.csvPath = csvPath
	m.Update(parseCSVCmd(csvPath)())
	if m.step != StepExistingJSONFilePicker {
		t.Fatalf("expected JSON picker, got %v", m.step)
	}
	m.jsonPath = jsonPath
	m.Update(loadJSONCmd(jsonPath)())
	if m.step != StepAssessmentSelect {
		t.Fatalf("expected assessment select, got %v", m.step)
	}
	tab(t, m, StepCSVPreview)
	if len(m.csvHeaders) != 30 {
		t.Fatalf("expected 30 columns, got %d", len(m.csvHeaders))
	}
	tab(t, m, StepIDColumnSelect)
	// Every column is offered, even on a short screen (the list scrolls)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 24})
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		view := stripANSI.ReplaceAllString(m.View(), "")
		for _, h := range headers {
			if strings.Contains(view, h+" ") {
				seen[h] = true
			}
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if len(seen) != 30 {
		t.Errorf("ID column list reached %d of 30 columns", len(seen))
	}
	for i := 0; i < 40; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	tab(t, m, StepIDPrefixStrip)
	if strings.Join(m.suggestedPrefixes, ",") != "KSI-" {
		t.Errorf("expected KSI- suggested, got %v", m.suggestedPrefixes)
	}
	tab(t, m, StepColumnMapping)
	if len(m.mappingColumns) != 29 {
		t.Fatalf("expected all 29 non-ID columns to be mappable, got %d", len(m.mappingColumns))
	}
	// Map column 5 to result by hand, then tab (warns once), tab again
	for i := 0; i < 4; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	for i := 0; i < targetIndex(MapResult); i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tab(t, m, StepColumnMapping) // warning
	tab(t, m, StepMetadata)
	if m.columnMappings[6] != MapMethod || m.columnMappings[7] != MapStepStatus {
		t.Errorf("content-checked suggestions should be kept: %v %v", m.columnMappings[6], m.columnMappings[7])
	}
	// The ↳ column rows reach every column
	m.focusMetadata(metaOwnerSrc)
	visited := map[int]bool{}
	for i := 0; i < 30; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
		visited[m.targetColumn(MapOwner)] = true
	}
	if len(visited) != 30 { // 29 columns + none
		t.Errorf("owner column picker reached %d of 29 columns", len(visited)-1)
	}
	tab(t, m, StepOutputPath)
	m.setOutputPath(filepath.Join(dir, "out.json"))
	tab(t, m, StepReview)
	tab(t, m, StepDone)

	out, err := LoadAssessments(filepath.Join(dir, "out.json"))
	if err != nil {
		t.Fatal(err)
	}
	step := out[0].ProgramRequirementAssessments[0].AssuranceSteps[0]
	if *step.Method != "EXAMINE" || step.Status != "PARTIAL" || plainRich(step.Result) != "Col F / Looks good" {
		t.Errorf("unexpected step written: method=%v status=%s result=%q", *step.Method, step.Status, plainRich(step.Result))
	}
}

// TestPicker_TabContinuesOnHighlightedFile drives the real file picker: tab on
// a highlighted .csv parses it and moves on, with no separate "choose" step.
func TestPicker_TabContinuesOnHighlightedFile(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "audit.csv")
	os.WriteFile(csvPath, []byte("ID,Notes\nA-1,ok\n"), 0644)

	m := NewModel()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.csvFilePicker.CurrentDirectory = dir
	m.step = StepCSVFilePicker
	m.Update(m.csvFilePicker.Init()()) // read the directory, as the real program does

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	runCmd(m, cmd)
	if m.csvPath != csvPath || m.step != StepExistingJSONFilePicker { // the download comes next
		t.Fatalf("tab on the highlighted file should parse it and continue; path=%q step=%v err=%v", m.csvPath, m.step, m.err)
	}
}

// TestOnlyTabAdvances presses enter on every step of a merge and checks that
// none of them moves on; tab alone does.
func TestOnlyTabAdvances(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "audit.csv")
	os.WriteFile(csvPath, []byte("Indicator ID,Notes\nKSI-A-1,ok\n"), 0644)
	jsonPath := filepath.Join(dir, "audit-assessments.json")
	os.WriteFile(jsonPath, []byte(`[{"name":"A","targetType":"PROGRAM","target":"P","program":"P","mechanism":"","cycleFrequency":"",
		"programRequirementAssessments":[{"programRequirement":"A-1","status":"OPEN","owner":"","reviewer":"",
		"remarks":null,"assuranceSteps":[]}]}]`), 0644)

	enter := func(m *Model) {
		t.Helper()
		before := m.step
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			if msg := cmd(); msg != nil {
				switch msg.(type) {
				case csvParsedMsg, jsonLoadedMsg, generateDoneMsg:
					m.Update(msg)
				}
			}
		}
		if m.step != before {
			t.Fatalf("enter moved from step %v to %v; only tab should", before, m.step)
		}
	}

	m := NewModel()
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	enter(m)
	tab(t, m, StepCSVFilePicker)

	// Real picker: enter on the highlighted file stays and hints; tab continues
	m.csvFilePicker.CurrentDirectory = dir
	m.Update(m.csvFilePicker.Init()())
	m.Update(tea.KeyMsg{Type: tea.KeyDown}) // audit-assessments.json sorts first; move to audit.csv
	enter(m)
	if !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "Press tab to continue with audit") {
		t.Error("enter on a file should say that tab continues")
	}
	tab(t, m, StepExistingJSONFilePicker)
	m.jsonFilePicker.CurrentDirectory = dir
	m.Update(m.jsonFilePicker.Init()())
	enter(m)
	tab(t, m, StepAssessmentSelect)

	for _, next := range []Step{StepCSVPreview, StepIDColumnSelect, StepIDPrefixStrip, StepColumnMapping} {
		enter(m)
		tab(t, m, next)
	}
	enter(m) // the only mapping column: confirms it, stays on the step
	if !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "last column. Press tab") {
		t.Error("the last mapping column should say tab continues")
	}
	tab(t, m, StepMetadata)
	for i := 0; i < len(metaOrder)+2; i++ {
		enter(m) // moves between fields, never to the next step
	}
	tab(t, m, StepOutputPath)
	m.setOutputPath(filepath.Join(dir, "out.json"))
	enter(m)
	tab(t, m, StepReview)
	enter(m)
	tab(t, m, StepDone)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("enter on the Done screen should not quit; tab or q does")
	}
}

// TestMergeWritesOnlyTheMergedAssessment: the download holds every
// assessment in the workspace, but the output must hold just the one that
// was updated, or importing it would duplicate the rest in Paramify.
func TestMergeWritesOnlyTheMergedAssessment(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "audit.csv")
	os.WriteFile(csvPath, []byte("ID,Notes\nB-1,updated\n"), 0644)
	jsonPath := filepath.Join(dir, "audit-assessments.json")
	os.WriteFile(jsonPath, []byte(`[
		{"name":"First","targetType":"PROGRAM","target":"P1","program":"P1","mechanism":"","cycleFrequency":"",
		 "programRequirementAssessments":[{"programRequirement":"A-1","status":"OPEN","owner":"","reviewer":"","remarks":null,"assuranceSteps":[]}]},
		{"name":"Second","targetType":"PROGRAM","target":"P2","program":"P2","mechanism":"","cycleFrequency":"",
		 "programRequirementAssessments":[{"programRequirement":"B-1","status":"OPEN","owner":"","reviewer":"","remarks":null,"assuranceSteps":[]}]},
		{"name":"Third","targetType":"PROGRAM","target":"P3","program":"P3","mechanism":"","cycleFrequency":"",
		 "programRequirementAssessments":[]}]`), 0644)

	m := NewModel()
	m.step = StepCSVFilePicker // the parse/load messages are handled on the picker steps
	m.csvPath = csvPath
	m.Update(parseCSVCmd(csvPath)())
	m.jsonPath = jsonPath
	m.Update(loadJSONCmd(jsonPath)())
	if len(m.existingData) != 3 {
		t.Fatalf("download should load 3 assessments, got %d", len(m.existingData))
	}
	m.selectedAssessmentIdx = 1 // "Second"
	m.idColumnIdx = 0
	m.columnMappings = []MappingTarget{MapID, MapResult}
	m.outputFilePath = filepath.Join(dir, "out.json")
	m.Update(m.generateCmd()())
	if m.step != StepDone {
		t.Fatalf("expected done, got %v (err %v)", m.step, m.err)
	}

	out, err := LoadAssessments(m.outputFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Name != "Second" {
		names := []string{}
		for _, a := range out {
			names = append(names, a.Name)
		}
		t.Fatalf("output should hold only the merged assessment, got %v", names)
	}
	if got := plainRich(out[0].ProgramRequirementAssessments[0].AssuranceSteps[0].Result); got != "Notes / updated" {
		t.Errorf("merged result = %q", got)
	}
	// The download itself is untouched
	orig, _ := LoadAssessments(jsonPath)
	if len(orig) != 3 {
		t.Errorf("download should still hold 3 assessments, got %d", len(orig))
	}
}

func TestDefaultOutputPathIsUnique(t *testing.T) {
	at := time.Date(2026, 10, 20, 14, 32, 0, 0, time.Local)
	m := NewModel()
	m.jsonPath = "/downloads/audit-assessments.json"
	if got := m.defaultOutputPath(at); got != "/downloads/audit-assessments-merged-2026-10-20-1432.json" {
		t.Errorf("merge default = %q", got)
	}
	// A later run gets a different name
	if m.defaultOutputPath(at) == m.defaultOutputPath(at.Add(time.Minute)) {
		t.Error("two runs a minute apart should not share a file name")
	}
}

func TestWhereToSave_ShowsFullPathAndSplitsFields(t *testing.T) {
	dir := t.TempDir()
	long := filepath.Join(dir, "a very long folder name that would not fit", "another long folder")
	os.MkdirAll(long, 0755)

	m := NewModel()
	m.Update(tea.WindowSizeMsg{Width: 187, Height: 43})
	m.jsonPath = filepath.Join(long, "client-SOC2-audit-assessments.json")
	m.initOutputPath()
	m.step = StepOutputPath

	if m.outputField != outputFieldName {
		t.Error("the cursor should start on the file name")
	}
	view := stripANSI.ReplaceAllString(m.View(), "")
	name := filepath.Base(m.outputPath())
	if !strings.HasPrefix(name, "client-SOC2-audit-assessments-merged-") {
		t.Fatalf("unexpected default name %q", name)
	}
	if !strings.Contains(view, "File name:") || !strings.Contains(view, name) {
		t.Errorf("the whole file name should be visible:\n%s", view)
	}
	if !strings.Contains(strings.Join(strings.Fields(view), ""), strings.Join(strings.Fields(m.outputPath()), "")) {
		t.Errorf("the full path should be shown (wrapped if needed):\n%s", view)
	}

	// Rename: clear the name and type a new one without an extension
	m.outputNameInput.SetValue("")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("client-final")})
	if got := m.outputPath(); got != filepath.Join(long, "client-final.json") {
		t.Errorf("expected .json added, got %q", got)
	}

	// A folder that doesn't exist stops tab and says why
	press(m, "down")
	if m.outputField != outputFieldFolder {
		t.Fatal("↓ should move to the folder field")
	}
	m.outputFolderInput.SetValue(filepath.Join(dir, "nope"))
	press(m, "tab")
	if m.step != StepOutputPath || !strings.Contains(stripANSI.ReplaceAllString(m.View(), ""), "folder doesn't exist") {
		t.Error("tab with a missing folder should stay and explain")
	}
	m.outputFolderInput.SetValue(long)
	press(m, "tab")
	if m.step != StepReview || m.outputFilePath != filepath.Join(long, "client-final.json") {
		t.Errorf("expected review with the new path, got %v / %q", m.step, m.outputFilePath)
	}
}

func TestWelcome_IsMergeOnly(t *testing.T) {
	m := NewModel()
	view := stripANSI.ReplaceAllString(m.View(), "")
	if strings.Contains(view, "Create new assessment") {
		t.Error("the welcome screen should no longer offer a new-assessment mode")
	}
	press(m, "tab")
	if m.step != StepCSVFilePicker {
		t.Fatalf("tab should start the tool, got %v", m.step)
	}
	if steps := m.flowSteps(); len(steps) != 11 {
		t.Errorf("expected 11 steps, got %d", len(steps))
	}

	// The write-time safety net still refuses an assessment with no program
	if err := checkImportable(Assessment{Name: "x"}); err == nil || !strings.Contains(err.Error(), "no program") {
		t.Errorf("expected an empty program to be refused, got %v", err)
	}
	if err := checkImportable(Assessment{Name: "x", Program: "SOC 2 AICPA"}); err != nil {
		t.Errorf("a complete assessment should pass, got %v", err)
	}
}
