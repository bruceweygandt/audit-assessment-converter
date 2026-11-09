package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestIntegration_GenerateFromExampleCSV(t *testing.T) {
	csvPath := "../../example_files/20x Phase Two Report - Paramify - 01.26.2026 - Human-Readable (1).csv"
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Skip("example CSV file not found")
	}

	csvData, err := ParseCSV(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	// Map columns as: ID=0, Skip=1, Skip=2, Result=3(Automation), Result=4(Assessor), Remediation=5(Gaps), Skip=6
	config := GenerateConfig{
		Name:           "Integration Test Assessment",
		Target:         "20x Moderate - Phase Two Pilot",
		Program:        "20x Moderate - Phase Two Pilot",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapSkip, MapSkip, MapResult, MapResult, MapRemediation, MapSkip},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "EXAMINE",
		AssignedEmail:  "assessor@example.com",
		OwnerEmail:     "owner@example.com",
	}

	assessment := GenerateAssessment(csvData, config)

	// Verify correct number of requirements
	if len(assessment.ProgramRequirementAssessments) != len(csvData.Rows) {
		t.Errorf("expected %d PRAs, got %d", len(csvData.Rows), len(assessment.ProgramRequirementAssessments))
	}

	// Find KSI-CMT-04 and verify structure
	var found bool
	for _, pra := range assessment.ProgramRequirementAssessments {
		if pra.ProgramRequirement == "KSI-CMT-04" {
			found = true

			if len(pra.AssuranceSteps) != 1 {
				t.Fatalf("expected 1 assurance step, got %d", len(pra.AssuranceSteps))
			}

			step := pra.AssuranceSteps[0]

			// Verify result has bold headers
			if len(step.Result) < 4 {
				t.Fatalf("expected at least 4 result nodes, got %d", len(step.Result))
			}

			// First node should be bold "Automation Capability"
			if !step.Result[0].Children[0].Bold {
				t.Error("expected first result node to be bold")
			}
			if step.Result[0].Children[0].Text != "Automation Capability" {
				t.Errorf("expected bold header 'Automation Capability', got %q", step.Result[0].Children[0].Text)
			}

			// Should have an "Assessor Overview" bold header somewhere
			foundOverview := false
			for _, node := range step.Result {
				if len(node.Children) > 0 && node.Children[0].Bold && node.Children[0].Text == "Assessor Overview" {
					foundOverview = true
					break
				}
			}
			if !foundOverview {
				t.Error("expected to find bold 'Assessor Overview' header in result")
			}

			// Remediation should not be null (CMT-04 has gaps)
			if string(step.Remediation) == "null" {
				t.Error("expected non-null remediation for KSI-CMT-04")
			}

			break
		}
	}
	if !found {
		t.Error("KSI-CMT-04 not found in generated assessment")
	}

	// Verify full output is valid JSON
	output, err := json.MarshalIndent([]Assessment{*assessment}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var reparsed []Assessment
	if err := json.Unmarshal(output, &reparsed); err != nil {
		t.Fatalf("generated JSON is not valid: %v", err)
	}

	// Verify a row with empty automation capability and gaps has proper structure
	for _, pra := range assessment.ProgramRequirementAssessments {
		if pra.ProgramRequirement == "FRR-MAS-02" {
			step := pra.AssuranceSteps[0]
			// MAS-02 has empty automation capability, so result should skip that header
			// and should have "Assessor Overview" as the first bold header
			if len(step.Result) < 1 {
				t.Fatal("expected non-empty result for FRR-MAS-02")
			}
			firstBold := step.Result[0]
			if firstBold.Children[0].Text == "Automation Capability" {
				// Automation Capability should be empty for this row, so it should be skipped
				// But let's verify the actual CSV data
				for _, row := range csvData.Rows {
					if row[0] == "FRR-MAS-02" {
						if row[3] == "" {
							t.Error("FRR-MAS-02 has empty automation but bold header was generated")
						}
						break
					}
				}
			}
			break
		}
	}
}

func TestIntegration_GenerateWithPrefixStrip(t *testing.T) {
	csvPath := "../../example_files/20x Phase Two Report - Paramify - 01.26.2026 - Human-Readable (1).csv"
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Skip("example CSV file not found")
	}

	csvData, err := ParseCSV(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	config := GenerateConfig{
		Name:           "FR 20x Mod",
		Target:         "FR 20x Mod",
		Program:        "FR 20x Mod",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapSkip, MapSkip, MapResult, MapResult, MapResult, MapResult, MapSkip},
		DefaultStatus:  "SUBMITTED",
		DefaultMethod:  "EXAMINE",
		PlanText:       "The assessor reviewed the FedRAMP 20x compliance evidence.",
		IDPrefixes:     []string{"FRR-", "KSI-"},
	}

	// Ensure mappings cover all columns (pad if needed)
	for len(config.ColumnMappings) < len(csvData.Headers) {
		config.ColumnMappings = append(config.ColumnMappings, MapSkip)
	}

	assessment := GenerateAssessment(csvData, config)

	if len(assessment.ProgramRequirementAssessments) != len(csvData.Rows) {
		t.Errorf("expected %d PRAs, got %d", len(csvData.Rows), len(assessment.ProgramRequirementAssessments))
	}

	// Verify prefix stripping worked — no IDs should start with FRR- anymore.
	// Note: IDs like FRR-KSI-01 become KSI-01 after stripping FRR-, which is a
	// valid control ID (KSI is a control family). So we only check for FRR- prefix.
	for i, pra := range assessment.ProgramRequirementAssessments {
		id := pra.ProgramRequirement
		if len(id) >= 4 && id[:4] == "FRR-" {
			t.Errorf("PRA[%d] ID %q still has FRR- prefix after stripping", i, id)
		}
		// IDs that were originally KSI-xxx should have KSI- stripped
		origID := csvData.Rows[i][0]
		if len(origID) >= 4 && origID[:4] == "KSI-" {
			if id == origID {
				t.Errorf("PRA[%d] ID %q was not stripped (original: %q)", i, id, origID)
			}
		}
	}

	// Write to temp file and verify
	tmpFile, err := os.CreateTemp("", "test-assessment-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := WriteAssessments(tmpFile.Name(), []Assessment{*assessment}); err != nil {
		t.Fatalf("WriteAssessments failed: %v", err)
	}

	// Read back and verify
	written, err := LoadAssessments(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read back written file: %v", err)
	}

	if len(written) != 1 {
		t.Fatalf("expected 1 assessment, got %d", len(written))
	}
	if written[0].Name != "FR 20x Mod" {
		t.Errorf("expected name 'FR 20x Mod', got %q", written[0].Name)
	}
	if len(written[0].ProgramRequirementAssessments) != len(csvData.Rows) {
		t.Errorf("expected %d PRAs in written file, got %d",
			len(csvData.Rows), len(written[0].ProgramRequirementAssessments))
	}

	// Verify first PRA has stripped ID
	firstPRA := written[0].ProgramRequirementAssessments[0]
	if firstPRA.ProgramRequirement == csvData.Rows[0][0] {
		t.Errorf("first PRA ID %q was not stripped", firstPRA.ProgramRequirement)
	}

	t.Logf("Generated %d PRAs, wrote %d bytes to %s",
		len(assessment.ProgramRequirementAssessments),
		func() int64 { fi, _ := os.Stat(tmpFile.Name()); return fi.Size() }(),
		tmpFile.Name())
}

func TestIntegration_MergeWithExampleJSON(t *testing.T) {
	jsonPath := "../../example_files/assessments.json"
	csvPath := "../../example_files/20x Phase Two Report - Paramify - 01.26.2026 - Human-Readable (1).csv"

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Skip("example files not found")
	}

	// Load existing assessments
	assessments, err := LoadAssessments(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	originalAssessmentCount := len(assessments)
	originalPRACount := len(assessments[2].ProgramRequirementAssessments) // Phase Two

	// Parse CSV
	csvData, err := ParseCSV(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	config := GenerateConfig{
		Name:           "Test",
		Target:         "Test",
		Program:        "Test",
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapSkip, MapSkip, MapResult, MapResult, MapRemediation, MapSkip},
		DefaultStatus:  "SUBMITTED",
		DefaultMethod:  "EXAMINE",
	}

	// Merge into the third assessment (Phase Two)
	MergeIntoAssessment(&assessments[2], csvData, config)

	// Should not change number of assessments
	if len(assessments) != originalAssessmentCount {
		t.Errorf("expected %d assessments, got %d", originalAssessmentCount, len(assessments))
	}

	// Phase Two should have at least as many PRAs as before (potentially more from new IDs)
	if len(assessments[2].ProgramRequirementAssessments) < originalPRACount {
		t.Errorf("expected at least %d PRAs after merge, got %d",
			originalPRACount, len(assessments[2].ProgramRequirementAssessments))
	}

	// Other assessments should be unchanged
	if len(assessments[0].ProgramRequirementAssessments) != 837 {
		t.Errorf("expected first assessment to still have 837 PRAs, got %d",
			len(assessments[0].ProgramRequirementAssessments))
	}
}
