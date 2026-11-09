package main

import (
	"encoding/json"
	"testing"
)

func TestMergeIntoAssessment_MatchingIDs(t *testing.T) {
	existing := &Assessment{
		Name:       "Existing",
		TargetType: "PROGRAM",
		ProgramRequirementAssessments: []ProgramRequirementAssessment{
			{
				ProgramRequirement: "REQ-01",
				Status:             "OPEN",
				AssuranceSteps: []AssuranceStep{
					{
						Order:       0,
						Status:      "OPEN",
						Plan:        EmptyParagraph(),
						Result:      EmptyParagraph(),
						Remediation: json.RawMessage("null"),
					},
				},
			},
			{
				ProgramRequirement: "REQ-02",
				Status:             "OPEN",
				AssuranceSteps:     []AssuranceStep{},
			},
		},
	}

	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"REQ-01", "Updated notes"},
		},
	}

	config := GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "SUBMITTED",
		DefaultMethod:  "EXAMINE",
	}

	MergeIntoAssessment(existing, csvData, config)

	// REQ-01 should be updated
	pra := existing.ProgramRequirementAssessments[0]
	if pra.Status != "SUBMITTED" {
		t.Errorf("expected status 'SUBMITTED', got %q", pra.Status)
	}
	if len(pra.AssuranceSteps) != 1 {
		t.Fatalf("expected 1 assurance step, got %d", len(pra.AssuranceSteps))
	}
	// Result should have been replaced with new content
	if len(pra.AssuranceSteps[0].Result) < 2 {
		t.Error("expected result to have bold header + content")
	}

	// REQ-02 should be unchanged
	pra2 := existing.ProgramRequirementAssessments[1]
	if pra2.Status != "OPEN" {
		t.Errorf("expected REQ-02 status to remain 'OPEN', got %q", pra2.Status)
	}
}

func TestMergeIntoAssessment_NewIDs(t *testing.T) {
	existing := &Assessment{
		Name:       "Existing",
		TargetType: "PROGRAM",
		ProgramRequirementAssessments: []ProgramRequirementAssessment{
			{ProgramRequirement: "REQ-01", Status: "OPEN"},
		},
	}

	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"REQ-NEW", "New requirement"},
		},
	}

	config := GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "OPEN",
		DefaultMethod:  "TEST",
	}

	MergeIntoAssessment(existing, csvData, config)

	if len(existing.ProgramRequirementAssessments) != 2 {
		t.Fatalf("expected 2 PRAs after merge, got %d", len(existing.ProgramRequirementAssessments))
	}

	// Original should be preserved
	if existing.ProgramRequirementAssessments[0].ProgramRequirement != "REQ-01" {
		t.Error("expected original REQ-01 to be preserved")
	}

	// New one should be appended
	newPRA := existing.ProgramRequirementAssessments[1]
	if newPRA.ProgramRequirement != "REQ-NEW" {
		t.Errorf("expected 'REQ-NEW', got %q", newPRA.ProgramRequirement)
	}
}

func TestMergeIntoAssessment_PrefixStrip(t *testing.T) {
	// Simulates the exact scenario that caused the duplicate PRA bug:
	// Existing assessment has bare IDs like "CNA-01", CSV has prefixed IDs like "FRR-CNA-01"
	existing := &Assessment{
		Name:       "Existing",
		TargetType: "PROGRAM",
		ProgramRequirementAssessments: []ProgramRequirementAssessment{
			{
				ProgramRequirement: "CNA-01",
				Status:             "OPEN",
				AssuranceSteps: []AssuranceStep{
					{
						Order:       0,
						Status:      "OPEN",
						Plan:        EmptyParagraph(),
						Result:      EmptyParagraph(),
						Remediation: json.RawMessage("null"),
					},
				},
			},
			{
				ProgramRequirement: "ADS-01",
				Status:             "OPEN",
				AssuranceSteps: []AssuranceStep{
					{
						Order:       0,
						Status:      "OPEN",
						Plan:        EmptyParagraph(),
						Result:      EmptyParagraph(),
						Remediation: json.RawMessage("null"),
					},
				},
			},
		},
	}

	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"FRR-CNA-01", "Updated CNA notes"},
			{"KSI-ADS-01", "Updated ADS notes"},
			{"NEW-01", "Brand new requirement"},
		},
	}

	config := GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "SUBMITTED",
		DefaultMethod:  "EXAMINE",
		IDPrefixes:     []string{"FRR-", "KSI-"},
	}

	MergeIntoAssessment(existing, csvData, config)

	// Should have 3 PRAs: 2 updated + 1 new (NOT 5 from duplicates)
	if len(existing.ProgramRequirementAssessments) != 3 {
		t.Fatalf("expected 3 PRAs after merge (2 updated + 1 new), got %d", len(existing.ProgramRequirementAssessments))
	}

	// CNA-01 should be updated (matched via FRR-CNA-01 with prefix strip)
	pra0 := existing.ProgramRequirementAssessments[0]
	if pra0.ProgramRequirement != "CNA-01" {
		t.Errorf("expected 'CNA-01', got %q", pra0.ProgramRequirement)
	}
	if pra0.Status != "SUBMITTED" {
		t.Errorf("expected CNA-01 status 'SUBMITTED', got %q", pra0.Status)
	}

	// ADS-01 should be updated (matched via KSI-ADS-01 with prefix strip)
	pra1 := existing.ProgramRequirementAssessments[1]
	if pra1.ProgramRequirement != "ADS-01" {
		t.Errorf("expected 'ADS-01', got %q", pra1.ProgramRequirement)
	}
	if pra1.Status != "SUBMITTED" {
		t.Errorf("expected ADS-01 status 'SUBMITTED', got %q", pra1.Status)
	}

	// NEW-01 should be appended (no matching prefix, no match in existing)
	pra2 := existing.ProgramRequirementAssessments[2]
	if pra2.ProgramRequirement != "NEW-01" {
		t.Errorf("expected 'NEW-01', got %q", pra2.ProgramRequirement)
	}
}

func TestMergeIntoAssessment_EmptyExisting(t *testing.T) {
	existing := &Assessment{
		Name:       "Empty",
		TargetType: "PROGRAM",
	}

	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows: [][]string{
			{"REQ-01", "First"},
			{"REQ-02", "Second"},
		},
	}

	config := GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		DefaultStatus:  "OPEN",
	}

	MergeIntoAssessment(existing, csvData, config)

	if len(existing.ProgramRequirementAssessments) != 2 {
		t.Fatalf("expected 2 PRAs, got %d", len(existing.ProgramRequirementAssessments))
	}
}

func TestMergeIntoAssessment_KeepsAssignedAndReportsIDs(t *testing.T) {
	existing := &Assessment{
		ProgramRequirementAssessments: []ProgramRequirementAssessment{
			{
				ProgramRequirement: "CNA-03",
				AssuranceSteps: []AssuranceStep{
					{Assigned: "assessor@example.com", Remediation: json.RawMessage("null")},
				},
			},
			{
				ProgramRequirement: "CNA-04",
				AssuranceSteps: []AssuranceStep{
					{Assigned: "old@example.com", Remediation: json.RawMessage("null")},
				},
			},
		},
	}
	csvData := &CSVData{
		Headers: []string{"ID", "Notes"},
		Rows:    [][]string{{"KSI-CNA-03", "a"}, {"KSI-ZZZ-99", "b"}},
	}

	updated, added := MergeIntoAssessment(existing, csvData, GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		IDPrefixes:     []string{"KSI-"},
	})

	if len(updated) != 1 || updated[0] != "CNA-03" {
		t.Errorf("expected updated [CNA-03], got %v", updated)
	}
	if len(added) != 1 || added[0] != "ZZZ-99" {
		t.Errorf("expected added [ZZZ-99], got %v", added)
	}
	if got := existing.ProgramRequirementAssessments[0].AssuranceSteps[0].Assigned; got != "assessor@example.com" {
		t.Errorf("blank Assigned should keep the existing assessor, got %q", got)
	}

	// A non-blank Assigned replaces the existing assessor
	csvData.Rows = [][]string{{"KSI-CNA-04", "c"}}
	MergeIntoAssessment(existing, csvData, GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapResult},
		IDPrefixes:     []string{"KSI-"},
		AssignedEmail:  "new@example.com",
	})
	if got := existing.ProgramRequirementAssessments[1].AssuranceSteps[0].Assigned; got != "new@example.com" {
		t.Errorf("expected new assessor, got %q", got)
	}
}

func TestMergeIntoAssessment_PerRowPeopleKeepExistingWhenBlank(t *testing.T) {
	existing := &Assessment{
		ProgramRequirementAssessments: []ProgramRequirementAssessment{
			{ProgramRequirement: "A", Owner: "old-owner", Reviewer: "old-rev",
				AssuranceSteps: []AssuranceStep{{Assigned: "old-assessor", Remediation: json.RawMessage("null")}}},
			{ProgramRequirement: "B", Owner: "old-owner", Reviewer: "old-rev",
				AssuranceSteps: []AssuranceStep{{Assigned: "old-assessor", Remediation: json.RawMessage("null")}}},
		},
	}
	csvData := &CSVData{
		Headers: []string{"ID", "Owner", "Reviewer", "Assigned"},
		Rows:    [][]string{{"A", "new-owner", "new-rev", "new-assessor"}, {"B", "", "", ""}},
	}
	MergeIntoAssessment(existing, csvData, GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapOwner, MapReviewer, MapAssigned},
	})
	a, b := existing.ProgramRequirementAssessments[0], existing.ProgramRequirementAssessments[1]
	if a.Owner != "new-owner" || a.Reviewer != "new-rev" || a.AssuranceSteps[0].Assigned != "new-assessor" {
		t.Errorf("row values should be applied, got %+v", a)
	}
	if b.Owner != "old-owner" || b.Reviewer != "old-rev" || b.AssuranceSteps[0].Assigned != "old-assessor" {
		t.Errorf("blank cells should keep existing people, got %+v", b)
	}
}

func TestMergeIntoAssessment_BlankTextKeepsExisting(t *testing.T) {
	oldRem, _ := json.Marshal(TextToParagraphs("Existing remediation"))
	existing := &Assessment{
		ProgramRequirementAssessments: []ProgramRequirementAssessment{{
			ProgramRequirement: "A",
			AssuranceSteps: []AssuranceStep{{
				Plan:        TextToParagraphs("Existing plan"),
				Result:      TextToParagraphs("Existing result"),
				Remediation: oldRem,
			}},
		}},
	}
	csvData := &CSVData{Headers: []string{"ID", "Gaps"}, Rows: [][]string{{"A", ""}}}
	MergeIntoAssessment(existing, csvData, GenerateConfig{
		IDColumnIdx:    0,
		ColumnMappings: []MappingTarget{MapID, MapRemediation},
	})
	s := existing.ProgramRequirementAssessments[0].AssuranceSteps[0]
	if plainRich(s.Plan) != "Existing plan" || plainRich(s.Result) != "Existing result" || plainRich(rawRich(s.Remediation)) != "Existing remediation" {
		t.Errorf("blank CSV values should keep existing text, got plan=%q result=%q remediation=%s",
			plainRich(s.Plan), plainRich(s.Result), s.Remediation)
	}
}

func TestMergeIntoAssessment_StepStatusColumnThenDefault(t *testing.T) {
	existing := &Assessment{
		ProgramRequirementAssessments: []ProgramRequirementAssessment{{
			ProgramRequirement: "A",
			AssuranceSteps:     []AssuranceStep{{Status: "OPEN", Result: TextToParagraphs("Existing result"), Remediation: json.RawMessage("null")}},
		}},
	}
	csvData := &CSVData{Headers: []string{"ID", "Notes", "Outcome"}, Rows: [][]string{{"A", "", ""}}}
	cfg := GenerateConfig{IDColumnIdx: 0, ColumnMappings: []MappingTarget{MapID, MapResult, MapStepStatus}, DefaultStepStatus: "PARTIAL"}

	// Blank status cell: the Details value applies
	MergeIntoAssessment(existing, csvData, cfg)
	if got := existing.ProgramRequirementAssessments[0].AssuranceSteps[0].Status; got != "PARTIAL" {
		t.Errorf("blank status cell should use the Details value, got %s", got)
	}

	// An explicit status in the CSV wins
	csvData.Rows = [][]string{{"A", "", "Pass"}}
	MergeIntoAssessment(existing, csvData, cfg)
	if got := existing.ProgramRequirementAssessments[0].AssuranceSteps[0].Status; got != "PASS" {
		t.Errorf("explicit CSV status should win, got %s", got)
	}
}
