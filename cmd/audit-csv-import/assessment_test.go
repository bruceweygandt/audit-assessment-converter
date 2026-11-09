package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateAssessment_Basic(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Title", "Notes", "Gaps"},
		Rows: [][]string{
			{"REQ-01", "First Req", "Some notes", "A gap"},
			{"REQ-02", "Second Req", "More notes", ""},
		},
	}

	config := GenerateConfig{
		Name:           "Test Assessment",
		Target:         "Test Target",
		Program:        "Test Program",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapSkip, MapResult, MapRemediation},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "EXAMINE",
		AssignedEmail:  "test@example.com",
		OwnerEmail:     "owner@example.com",
		ReviewerEmail:  "reviewer@example.com",
	}

	assessment := GenerateAssessment(csvData, config)

	if assessment.Name != "Test Assessment" {
		t.Errorf("expected name 'Test Assessment', got %q", assessment.Name)
	}
	if assessment.TargetType != "PROGRAM" {
		t.Errorf("expected targetType 'PROGRAM', got %q", assessment.TargetType)
	}
	if len(assessment.ProgramRequirementAssessments) != 2 {
		t.Fatalf("expected 2 PRAs, got %d", len(assessment.ProgramRequirementAssessments))
	}

	pra1 := assessment.ProgramRequirementAssessments[0]
	if pra1.ProgramRequirement != "REQ-01" {
		t.Errorf("expected 'REQ-01', got %q", pra1.ProgramRequirement)
	}
	if pra1.Status != "OPEN" {
		t.Errorf("expected status 'OPEN', got %q", pra1.Status)
	}
	if len(pra1.AssuranceSteps) != 1 {
		t.Fatalf("expected 1 assurance step, got %d", len(pra1.AssuranceSteps))
	}

	step := pra1.AssuranceSteps[0]
	if *step.Method != "EXAMINE" {
		t.Errorf("expected method 'EXAMINE', got %q", *step.Method)
	}
	if step.Assigned != "test@example.com" {
		t.Errorf("expected assigned 'test@example.com', got %q", step.Assigned)
	}

	// Result should have bold header "Notes" + content
	if len(step.Result) < 2 {
		t.Fatalf("expected at least 2 result nodes, got %d", len(step.Result))
	}
	if !step.Result[0].Children[0].Bold {
		t.Error("expected first result node to be bold header")
	}

	// Remediation should not be null for REQ-01 (has gap)
	if string(step.Remediation) == "null" {
		t.Error("expected non-null remediation for REQ-01")
	}

	// REQ-02 should have null remediation (empty gaps)
	pra2 := assessment.ProgramRequirementAssessments[1]
	step2 := pra2.AssuranceSteps[0]
	if string(step2.Remediation) != "null" {
		t.Errorf("expected null remediation for REQ-02, got %s", string(step2.Remediation))
	}
}

func TestGenerateAssessment_MultipleResultColumns(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Auto", "Overview"},
		Rows: [][]string{
			{"REQ-01", "Auto text", "Overview text"},
		},
	}

	config := GenerateConfig{
		Name:           "Test",
		Target:         "Target",
		Program:        "Program",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult, MapResult},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "EXAMINE",
	}

	assessment := GenerateAssessment(csvData, config)
	step := assessment.ProgramRequirementAssessments[0].AssuranceSteps[0]

	// Should have: bold "Auto", text "Auto text", bold "Overview", text "Overview text"
	if len(step.Result) != 4 {
		t.Fatalf("expected 4 result nodes, got %d", len(step.Result))
	}
	if step.Result[0].Children[0].Text != "Auto" {
		t.Errorf("expected 'Auto', got %q", step.Result[0].Children[0].Text)
	}
	if step.Result[2].Children[0].Text != "Overview" {
		t.Errorf("expected 'Overview', got %q", step.Result[2].Children[0].Text)
	}
}

func TestGenerateAssessment_ResultColumns_OutOfRangeHeaders(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Result"},
		Rows: [][]string{
			{"REQ-01", "Result text"},
		},
	}

	config := GenerateConfig{
		Name:        "Test",
		Target:      "Target",
		Program:     "Program",
		IDColumnIdx: 0,
		// ColumnMappings is intentionally longer than Headers; the extra MapResult
		// index should be ignored rather than causing a panic.
		ColumnMappings: []MappingTarget{MapID, MapResult, MapResult},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "EXAMINE",
	}

	assessment := GenerateAssessment(csvData, config)
	if len(assessment.ProgramRequirementAssessments) != 1 {
		t.Fatalf("expected 1 PRA, got %d", len(assessment.ProgramRequirementAssessments))
	}

	step := assessment.ProgramRequirementAssessments[0].AssuranceSteps[0]
	if len(step.Result) == 0 {
		t.Fatalf("expected non-empty result rich text, got 0 nodes")
	}
}

func TestGenerateAssessment_StaticPlanText(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"REQ-01", "Some notes"},
			{"REQ-02", "More notes"},
		},
	}

	planText := "The assessor reviewed the FedRAMP 20x compliance evidence. Assessment activities took place between 12/15/2025 and 01/26/2026."

	config := GenerateConfig{
		Name:           "Test",
		Target:         "Target",
		Program:        "Program",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "EXAMINE",
		PlanText:       planText,
	}

	assessment := GenerateAssessment(csvData, config)

	// Both steps should have the same static plan text
	for _, pra := range assessment.ProgramRequirementAssessments {
		step := pra.AssuranceSteps[0]
		if len(step.Plan) == 0 {
			t.Fatal("expected non-empty plan")
		}
		if step.Plan[0].Children[0].Text != planText {
			t.Errorf("expected plan text %q, got %q", planText, step.Plan[0].Children[0].Text)
		}
	}
}

func TestGenerateAssessment_PlanColumnsStackThenPlanText(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Approach", "Procedure", "Notes"},
		Rows: [][]string{
			{"REQ-01", "Reviewed configs", "Ran the validator", "Some notes"},
			{"REQ-02", "", "Checked logs", "Other notes"},
			{"REQ-03", "", "", "More notes"},
		},
	}

	config := GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapPlan, MapPlan, MapResult},
		DefaultStatus:  "OPEN",
		PlanText:       "Typed plan for blanks",
	}

	pras := GenerateAssessment(csvData, config).ProgramRequirementAssessments
	texts := func(nodes []RichTextNode) []string {
		var out []string
		for _, n := range nodes {
			c := n.Children[0]
			if c.Bold {
				out = append(out, "**"+c.Text+"**")
			} else {
				out = append(out, c.Text)
			}
		}
		return out
	}
	want := [][]string{
		{"**Approach**", "Reviewed configs", "**Procedure**", "Ran the validator"}, // both columns, each under its bold heading
		{"**Procedure**", "Checked logs"},                                          // blank column left out
		{"Typed plan for blanks"},                                                  // all blank: typed text, no heading
	}
	for i, w := range want {
		if got := texts(pras[i].AssuranceSteps[0].Plan); !reflect.DeepEqual(got, w) {
			t.Errorf("row %d: plan %v, want %v", i+1, got, w)
		}
	}
}

func TestGenerateAssessment_DefaultStepStatus(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Notes", "Outcome"},
		Rows: [][]string{
			{"REQ-01", "Has content", "Fail"},
			{"REQ-02", "Has content", ""},
			{"REQ-03", "", "who knows"},
		},
	}
	for _, def := range StepStatusOptions {
		config := GenerateConfig{
			IDColumnIdx:       0,
			ColumnMappings:    []MappingTarget{MapID, MapResult, MapStepStatus},
			DefaultStepStatus: def,
		}
		pras := GenerateAssessment(csvData, config).ProgramRequirementAssessments
		// The column wins; blank or unreadable cells use the Details value
		for i, want := range []string{"FAIL", def, def} {
			if got := pras[i].AssuranceSteps[0].Status; got != want {
				t.Errorf("default %s, row %d: step status %q, want %q", def, i+1, got, want)
			}
		}
	}

	// No default set at all falls back to OPEN
	pras := GenerateAssessment(csvData, GenerateConfig{IDColumnIdx: 0, ColumnMappings: []MappingTarget{MapID, MapResult}}).ProgramRequirementAssessments
	if got := pras[0].AssuranceSteps[0].Status; got != "OPEN" {
		t.Errorf("unset default should give OPEN, got %q", got)
	}
}

func TestParsePrefixes(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"FRR-", []string{"FRR-"}},
		{"FRR-,KSI-", []string{"FRR-", "KSI-"}},
		{"FRR- , KSI- ", []string{"FRR-", "KSI-"}},
		{" , , ", nil},
		{"ABC", []string{"ABC"}},
		{"A,,B", []string{"A", "B"}},
	}

	for _, tt := range tests {
		result := parsePrefixes(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parsePrefixes(%q): expected %v, got %v", tt.input, tt.expected, result)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("parsePrefixes(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], result[i])
			}
		}
	}
}

func TestStripIDPrefix(t *testing.T) {
	tests := []struct {
		id       string
		prefixes []string
		expected string
	}{
		{"FRR-CNA-01", []string{"FRR-", "KSI-"}, "CNA-01"},
		{"KSI-ADS-01", []string{"FRR-", "KSI-"}, "ADS-01"},
		{"CNA-01", []string{"FRR-", "KSI-"}, "CNA-01"},
		{"FRR-CNA-01", nil, "FRR-CNA-01"},
		{"FRR-CNA-01", []string{}, "FRR-CNA-01"},
		{"FRR-", []string{"FRR-"}, ""},
		{"", []string{"FRR-"}, ""},
	}

	for _, tt := range tests {
		result := stripIDPrefix(tt.id, tt.prefixes)
		if result != tt.expected {
			t.Errorf("stripIDPrefix(%q, %v): expected %q, got %q", tt.id, tt.prefixes, tt.expected, result)
		}
	}
}

func TestGenerateAssessment_IDPrefixStrip(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"FRR-CNA-01", "Note 1"},
			{"KSI-ADS-01", "Note 2"},
			{"REQ-03", "Note 3"},
		},
	}

	config := GenerateConfig{
		Name:           "Test",
		Target:         "Target",
		Program:        "Program",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "EXAMINE",
		IDPrefixes:     []string{"FRR-", "KSI-"},
	}

	assessment := GenerateAssessment(csvData, config)

	if len(assessment.ProgramRequirementAssessments) != 3 {
		t.Fatalf("expected 3 PRAs, got %d", len(assessment.ProgramRequirementAssessments))
	}

	// FRR-CNA-01 should become CNA-01
	if assessment.ProgramRequirementAssessments[0].ProgramRequirement != "CNA-01" {
		t.Errorf("expected 'CNA-01', got %q", assessment.ProgramRequirementAssessments[0].ProgramRequirement)
	}

	// KSI-ADS-01 should become ADS-01
	if assessment.ProgramRequirementAssessments[1].ProgramRequirement != "ADS-01" {
		t.Errorf("expected 'ADS-01', got %q", assessment.ProgramRequirementAssessments[1].ProgramRequirement)
	}

	// REQ-03 should remain REQ-03 (no matching prefix)
	if assessment.ProgramRequirementAssessments[2].ProgramRequirement != "REQ-03" {
		t.Errorf("expected 'REQ-03', got %q", assessment.ProgramRequirementAssessments[2].ProgramRequirement)
	}
}

func TestGenerateAssessment_NoPrefixStrip(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"FRR-CNA-01", "Note 1"},
		},
	}

	config := GenerateConfig{
		Name:           "Test",
		Target:         "Target",
		Program:        "Program",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "OPEN",
	}

	assessment := GenerateAssessment(csvData, config)

	// Without IDPrefixes, the ID should remain unchanged
	if assessment.ProgramRequirementAssessments[0].ProgramRequirement != "FRR-CNA-01" {
		t.Errorf("expected 'FRR-CNA-01', got %q", assessment.ProgramRequirementAssessments[0].ProgramRequirement)
	}
}

func TestAssessmentJSON_RoundTrip(t *testing.T) {
	path := "../../example_files/assessments.json"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example assessments.json not found")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var assessments []Assessment
	if err := json.Unmarshal(data, &assessments); err != nil {
		t.Fatal(err)
	}

	if len(assessments) != 3 {
		t.Fatalf("expected 3 assessments, got %d", len(assessments))
	}

	// Re-marshal and verify it produces valid JSON
	output, err := json.MarshalIndent(assessments, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var reparsed []Assessment
	if err := json.Unmarshal(output, &reparsed); err != nil {
		t.Fatalf("failed to re-parse marshaled JSON: %v", err)
	}

	if len(reparsed) != len(assessments) {
		t.Errorf("expected %d assessments after round-trip, got %d", len(assessments), len(reparsed))
	}

	// Verify PRA counts are preserved
	for i, a := range assessments {
		if len(reparsed[i].ProgramRequirementAssessments) != len(a.ProgramRequirementAssessments) {
			t.Errorf("assessment %d: expected %d PRAs, got %d",
				i, len(a.ProgramRequirementAssessments), len(reparsed[i].ProgramRequirementAssessments))
		}
	}
}

func TestLoadAssessments_CurrentFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-assessments.json")
	content := `[{"name":"A","targetType":"PROGRAM","target":"P","program":"P","mechanism":"","cycleFrequency":"ANNUALLY",
		"programRequirementAssessments":[{"programRequirement":"CNA-01","status":"OPEN","owner":"Client Success","reviewer":"",
		"remarks":null,"assuranceSteps":[{"order":0,"method":null,"status":"FAIL","assigned":"","plan":[],"result":[],"remediation":null}]}]}]`
	os.WriteFile(path, []byte(content), 0644)

	got, err := LoadAssessments(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].ProgramRequirementAssessments) != 1 || got[0].ProgramRequirementAssessments[0].ProgramRequirement != "CNA-01" {
		t.Errorf("expected CNA-01 to load, got %+v", got[0].ProgramRequirementAssessments)
	}
}

func TestLoadAssessments_RejectsLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.json")
	os.WriteFile(path, []byte(`[{"name":"A","projectRequirementAssessments":[{"projectRequirement":"CNA-01"}]}]`), 0644)

	_, err := LoadAssessments(path)
	if err == nil || !strings.Contains(err.Error(), "download the audit assessments from Paramify again") {
		t.Errorf("expected legacy format error, got %v", err)
	}
}

// TestLoadAssessments_RealExportRoundTrip checks that loading and re-writing a real
// "Download audit assessments" export from Paramify changes nothing. Merge rewrites
// the whole file, so any loss here would corrupt other assessments on import.
func TestLoadAssessments_RealExportRoundTrip(t *testing.T) {
	path := "../../example_files/audit-assessments.json"
	original, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("example_files/audit-assessments.json not found")
	}
	assessments, err := LoadAssessments(path)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "roundtrip.json")
	if err := WriteAssessments(out, assessments); err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(out)

	var want, got any
	json.Unmarshal(original, &want)
	json.Unmarshal(written, &got)
	if !reflect.DeepEqual(want, got) {
		t.Error("round-tripped export differs from the original")
	}
}

func TestGenerateAssessment_PerRowMethodStatusAndPeople(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Method", "Determination", "Assessor", "Owner", "Reviewer", "Notes"},
		Rows: [][]string{
			{"R-1", "Interview", "Satisfied", "a@x.com", "Client Success", "r@x.com", "ok"},
			{"R-2", "Examine, Test", "Other Than Satisfied", "", "", "", "gap"},
			{"R-3", "observe", "", "", "", "", ""},
		},
	}
	config := GenerateConfig{
		IDColumnIdx:       0,
		ColumnMappings:    []MappingTarget{MapID, MapMethod, MapStepStatus, MapAssigned, MapOwner, MapReviewer, MapResult},
		DefaultMethod:     "TEST",
		DefaultStepStatus: "OPEN",
		AssignedEmail:     "default@x.com",
		OwnerEmail:        "default-owner@x.com",
	}
	pras := GenerateAssessment(csvData, config).ProgramRequirementAssessments

	want := []struct{ method, status, assigned, owner, reviewer string }{
		{"INTERVIEW", "PASS", "a@x.com", "Client Success", "r@x.com"},
		{"EXAMINE", "FAIL", "default@x.com", "default-owner@x.com", ""}, // first listed method; blanks use defaults
		{"TEST", "OPEN", "default@x.com", "default-owner@x.com", ""},    // unrecognized method → default; no result → OPEN
	}
	for i, w := range want {
		p, s := pras[i], pras[i].AssuranceSteps[0]
		if *s.Method != w.method || s.Status != w.status || s.Assigned != w.assigned || p.Owner != w.owner || p.Reviewer != w.reviewer {
			t.Errorf("row %d: got method=%s status=%s assigned=%q owner=%q reviewer=%q, want %+v",
				i+1, *s.Method, s.Status, s.Assigned, p.Owner, p.Reviewer, w)
		}
	}
}

func TestGenerateAssessment_PerRowRequirementStatus(t *testing.T) {
	csvData := &CSVData{
		Headers: []string{"ID", "Review"},
		Rows:    [][]string{{"R-1", "Accepted"}, {"R-2", ""}, {"R-3", "who knows"}},
	}
	pras := GenerateAssessment(csvData, GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapStatus},
		DefaultStatus:  "SUBMITTED",
	}).ProgramRequirementAssessments
	for i, want := range []string{"ACCEPTED", "SUBMITTED", "SUBMITTED"} {
		if pras[i].Status != want {
			t.Errorf("row %d: status %q, want %q", i+1, pras[i].Status, want)
		}
	}
}
