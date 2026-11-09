package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMethod(t *testing.T) {
	cases := map[string]string{
		"Examine": "EXAMINE", "EXAMINED": "EXAMINE", "test": "TEST", "Testing": "TEST",
		"Interview": "INTERVIEW", "Interview, Examine": "INTERVIEW", "Examine / Test": "EXAMINE",
		"Unknown": "UNKNOWN", "unknown": "UNKNOWN",
		"": "", "Observe": "", "Attestation": "",
	}
	for in, want := range cases {
		if got := ParseMethod(in); got != want {
			t.Errorf("ParseMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseStepStatus(t *testing.T) {
	cases := map[string]string{
		"Satisfied": "PASS", "PASS": "PASS", "Met": "PASS", "Yes": "PASS", "Compliant": "PASS",
		"Other Than Satisfied": "FAIL", "Not Satisfied": "FAIL", "Fail": "FAIL", "Not Met": "FAIL",
		"Non-Compliant": "FAIL", "No": "FAIL",
		"Partial": "PARTIAL", "Partially Satisfied": "PARTIAL", "Partially Implemented": "PARTIAL",
		"Open": "OPEN", "In Progress": "OPEN", "Not Tested": "OPEN",
		"": "", "maybe": "",
	}
	for in, want := range cases {
		if got := ParseStepStatus(in); got != want {
			t.Errorf("ParseStepStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnrecognized(t *testing.T) {
	rows := [][]string{{"Examine"}, {"Observe"}, {""}, {"Observe"}, {"Walkthrough"}}
	if got := Unrecognized(rows, 0, ParseMethod); !reflect.DeepEqual(got, []string{"Observe", "Walkthrough"}) {
		t.Errorf("got %v", got)
	}
}

func TestParseRequirementStatus(t *testing.T) {
	cases := map[string]string{
		"Open": "OPEN", "In Progress": "OPEN", "Not Started": "OPEN",
		"Submitted": "SUBMITTED", "In Review": "SUBMITTED", "Ready for review": "SUBMITTED",
		"Accepted": "ACCEPTED", "Approved": "ACCEPTED", "ACCEPTED": "ACCEPTED",
		"Change Requested": "CHANGE_REQUESTED", "CHANGE_REQUESTED": "CHANGE_REQUESTED",
		"Changes requested": "CHANGE_REQUESTED", "Rejected": "CHANGE_REQUESTED", "Not accepted": "CHANGE_REQUESTED",
		"": "", "Incomplete": "", "???": "",
	}
	for in, want := range cases {
		if got := ParseRequirementStatus(in); got != want {
			t.Errorf("ParseRequirementStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOptionSetsMatchParamify(t *testing.T) {
	// The exact options Paramify's step settings offer, in its order
	if !reflect.DeepEqual(MethodOptions, []string{"TEST", "EXAMINE", "INTERVIEW", "UNKNOWN"}) {
		t.Errorf("methods: %v", MethodOptions)
	}
	if !reflect.DeepEqual(StepStatusOptions, []string{"OPEN", "PASS", "FAIL", "PARTIAL"}) {
		t.Errorf("step statuses: %v", StepStatusOptions)
	}
	want := []string{"OPEN", "OVERDUE", "SUBMITTED", "ACCEPTED", "ARCHIVED", "CHANGE_REQUESTED", "QA_REQUESTED", "REVIEWED"}
	if !reflect.DeepEqual(ReviewStatusOptions, want) {
		t.Errorf("review statuses: %v", ReviewStatusOptions)
	}
	for _, v := range []string{"Open", "Overdue", "Submitted", "Accepted", "Archived", "Change Requested", "QA Requested", "Reviewed"} {
		if got := ParseRequirementStatus(v); got == "" || got != strings.ToUpper(strings.ReplaceAll(v, " ", "_")) {
			t.Errorf("ParseRequirementStatus(%q) = %q", v, got)
		}
	}
	for _, v := range []string{"Open", "Pass", "Fail", "Partial"} {
		if got := ParseStepStatus(v); got != strings.ToUpper(v) {
			t.Errorf("ParseStepStatus(%q) = %q", v, got)
		}
	}
	for _, v := range []string{"Test", "Examine", "Interview", "Unknown"} {
		if got := ParseMethod(v); got != strings.ToUpper(v) {
			t.Errorf("ParseMethod(%q) = %q", v, got)
		}
	}
}

func TestParsers_MatchWholeCellsOnly(t *testing.T) {
	// Prose never counts, even when a known word is inside it
	prose := []string{
		"Use secure passwordless methods for user authentication", // contains "met"
		"Track and approve all changes",                           // contains "approv", "change"
		"Test the backup restore procedure quarterly",             // starts with "Test"
		"The control is open to all staff",                        // contains "open"
		"Satisfied with exceptions",                               // not exactly a value
		"Mostly met",
	}
	for _, v := range prose {
		if s := ParseStepStatus(v); s != "" {
			t.Errorf("ParseStepStatus(%q) = %q, want none", v, s)
		}
		if s := ParseRequirementStatus(v); s != "" {
			t.Errorf("ParseRequirementStatus(%q) = %q, want none", v, s)
		}
		if s := ParseMethod(v); s != "" {
			t.Errorf("ParseMethod(%q) = %q, want none", v, s)
		}
	}
	// Formatting differences still match
	for in, want := range map[string]string{"Satisfied.": "PASS", "  PARTIAL ": "PARTIAL", "Non-Compliant": "FAIL", "Other-Than-Satisfied": "FAIL"} {
		if got := ParseStepStatus(in); got != want {
			t.Errorf("ParseStepStatus(%q) = %q, want %q", in, got, want)
		}
	}
	if got := ParseMethod("Examine and Test"); got != "EXAMINE" {
		t.Errorf("ParseMethod(\"Examine and Test\") = %q", got)
	}
}
