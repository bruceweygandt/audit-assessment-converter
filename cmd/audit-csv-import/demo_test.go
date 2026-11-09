package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDemoFiles merges the demo CSV into the demo assessment with the
// mapping a user would pick, and checks the result. It skips when the demo
// folder isn't present (it's only on the demo branch).
func TestDemoFiles(t *testing.T) {
	csvPath, jsonPath := "../../demo/demo-assessment-report.csv", "../../demo/demo-audit-assessments.json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Skip("demo files not found")
	}
	csv, err := ParseCSV(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	assessments, err := LoadAssessments(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	target := &assessments[1] // Demo - FedRAMP 20x Phase One Pilot Assessment

	// The suggestions a user would see
	idCol := SuggestIDColumn(csv.Headers, csv.Rows, func() map[string]bool {
		ids := map[string]bool{}
		for _, p := range target.ProgramRequirementAssessments {
			ids[p.ProgramRequirement] = true
		}
		return ids
	}())
	if csv.Headers[idCol] != "Indicator ID" {
		t.Fatalf("ID column suggested as %q", csv.Headers[idCol])
	}
	mappings := make([]MappingTarget, len(csv.Headers))
	for i, sg := range SuggestMappings(csv.Headers, csv.Rows, idCol) {
		mappings[i] = sg.Target
	}
	// Plus the free-text columns a user maps by hand
	set := func(header string, t MappingTarget) {
		for i, h := range csv.Headers {
			if h == header {
				mappings[i] = t
			}
		}
	}
	set("Automation Capability", MapResult)
	set("Assessor Overview", MapResult)
	set("Gaps", MapRemediation)
	set("Recommendation", MapPlan)
	set("Assessor", MapAssigned)

	updated, added := MergeIntoAssessment(target, csv, GenerateConfig{
		IDColumnIdx: idCol, ColumnMappings: mappings, IDPrefixes: []string{"KSI-", "FRR-"},
		DefaultStatus: "OPEN", DefaultMethod: "EXAMINE", DefaultStepStatus: "OPEN",
	})
	if len(updated) != 14 || len(added) != 1 || added[0] != "XYZ-01" {
		t.Errorf("expected 14 updated and XYZ-01 added, got %d updated, added %v", len(updated), added)
	}
	out := filepath.Join(t.TempDir(), "out.json")
	if err := WriteAssessments(out, []Assessment{*target}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAssessments(out); err != nil {
		t.Errorf("written file doesn't load back: %v", err)
	}
}
