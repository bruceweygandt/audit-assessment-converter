package main

import (
	"reflect"
	"strings"
	"testing"
)

func set(ids ...string) map[string]bool {
	m := make(map[string]bool)
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestSuggestIDColumn_ByHeaderAndShape(t *testing.T) {
	headers := []string{"Requirement", "Indicator ID", "Assessor Overview"}
	rows := [][]string{
		{"Name things", "KSI-CNA-03", "Looks good"},
		{"Name more", "KSI-CNA-04", "Looks good"},
	}
	if got := SuggestIDColumn(headers, rows, nil); got != 1 {
		t.Errorf("expected column 1, got %d", got)
	}
}

func TestSuggestIDColumn_PrefersColumnMatchingAssessment(t *testing.T) {
	// No header hints: the column whose values match existing IDs (after a prefix) wins
	headers := []string{"Ref", "Control"}
	rows := [][]string{{"R-1", "KSI-CNA-03"}, {"R-2", "KSI-CNA-04"}}
	if got := SuggestIDColumn(headers, rows, set("CNA-03", "CNA-04")); got != 1 {
		t.Errorf("expected column 1, got %d", got)
	}
}

func TestSuggestPrefixes(t *testing.T) {
	existing := set("CNA-03", "CNA-04", "MAS-02", "AC-2")
	ids := []string{"KSI-CNA-03", "KSI-CNA-04", "FRR-MAS-02", "AC-2", "KSI-ZZZ-99"}
	got := SuggestPrefixes(ids, existing)
	want := []string{"KSI-", "FRR-"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestSuggestPrefixes_NeverCutsMidWord(t *testing.T) {
	// "XCNA-03" -> "CNA-03" would need the prefix "X", which doesn't end in a separator
	if got := SuggestPrefixes([]string{"XCNA-03"}, set("CNA-03")); len(got) != 0 {
		t.Errorf("expected no prefixes, got %v", got)
	}
	if got := SuggestPrefixes([]string{"CNA-03"}, nil); len(got) != 0 {
		t.Errorf("expected no prefixes without an assessment, got %v", got)
	}
}

// targets extracts the suggested targets.
func targets(sgs []Suggestion) []MappingTarget {
	out := make([]MappingTarget, len(sgs))
	for i, sg := range sgs {
		out[i] = sg.Target
	}
	return out
}

func TestSuggestMappings_AuditorReport(t *testing.T) {
	// Every non-ID column here is free text, so nothing is guessed from names
	headers := []string{"Indicator ID", "Requirement", "Description", "Automation Capability", "Assessor Overview", "Gaps", "Applies To"}
	rows := [][]string{
		{"KSI-CNA-03", "Name", "Desc", "Validator runs daily", "Assessor reviewed config.", "Needs monitoring", "All"},
		{"KSI-CNA-04", "Name", "Desc", "Bash scripts pull rules", "Looks good.", "", "All"},
	}
	got := targets(SuggestMappings(headers, rows, 0))
	want := []MappingTarget{MapID, MapSkip, MapSkip, MapSkip, MapSkip, MapSkip, MapSkip}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestSuggestMappings_ByContentWhateverTheName(t *testing.T) {
	headers := []string{"ID", "Col 1", "Col 2", "Col 3", "Reviewer", "Control Owner", "Free text"}
	rows := [][]string{
		{"A-1", "Examine", "Satisfied", "Accepted", "reviewer@example.com", "Client Success", "Long notes about the control."},
		{"A-2", "Examine, Interview", "Other Than Satisfied", "Submitted", "reviewer@example.com", "jane@x.com", "More notes."},
		{"A-3", "Test", "Not Met", "Change Requested", "", "Client Success", "Even more."},
	}
	sgs := SuggestMappings(headers, rows, 0)
	got := targets(sgs)
	want := []MappingTarget{MapID, MapMethod, MapStepStatus, MapStatus, MapReviewer, MapOwner, MapSkip}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
	if !strings.Contains(sgs[1].Reason, "3 of 3 filled cells") {
		t.Errorf("expected evidence in the reason, got %q", sgs[1].Reason)
	}
}

func TestSuggestMappings_DoesNotGuessFromLooseMatches(t *testing.T) {
	headers := []string{"ID", "Status", "Test Method Description", "Recommendations", "Owner", "Notes"}
	rows := [][]string{
		{"A-1", "Implemented", "The assessor tested the configuration by reviewing logs.", "Use MFA", "See the SSP section 4 for details.", "internal"},
		{"A-2", "Planned", "Interviewed the admin and examined screenshots.", "Rotate keys", "Handled by the platform team per policy.", "internal"},
	}
	got := targets(SuggestMappings(headers, rows, 0))
	want := []MappingTarget{MapID, MapSkip, MapSkip, MapSkip, MapSkip, MapSkip}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected nothing suggested, got %v", got)
	}
}

func TestSuggestMappings_AmbiguousCasesAreNotSuggested(t *testing.T) {
	// "Open" fits both statuses: only a header that says which decides
	rows := [][]string{{"A-1", "Open", "Open", "Open"}, {"A-2", "Open", "Open", "In Progress"}}
	got := targets(SuggestMappings([]string{"ID", "State", "Review Status", "Step Result"}, rows, 0))
	want := []MappingTarget{MapID, MapSkip, MapStatus, MapStepStatus}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}

	// Two method columns: the tool can't tell which is meant, so neither is suggested
	rows = [][]string{{"A-1", "Examine", "Test"}, {"A-2", "Test", "Interview"}}
	if got := targets(SuggestMappings([]string{"ID", "M1", "M2"}, rows, 0)); !reflect.DeepEqual(got, []MappingTarget{MapID, MapSkip, MapSkip}) {
		t.Errorf("expected no suggestion for duplicate method columns, got %v", got)
	}
}

func TestSuggestMappings_Threshold(t *testing.T) {
	// 3 of 5 recognized (60%) is below the 80% bar
	rows := [][]string{{"1", "Examine"}, {"2", "Test"}, {"3", "Interview"}, {"4", "Observe"}, {"5", "Walkthrough"}}
	if got := targets(SuggestMappings([]string{"ID", "How"}, rows, 0)); got[1] != MapSkip {
		t.Errorf("60%% recognized should not be suggested, got %v", got[1])
	}
}
