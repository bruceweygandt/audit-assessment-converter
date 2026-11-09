package main

import (
	"regexp"
	"strings"
)

// Reading per-row values (status, method, step status) from the auditor's own wording.
// The values are exactly the options Paramify's step settings offer:
// Method: Test / Examine / Interview / Unknown. Status: Open / Pass / Fail / Partial.

var (
	// MethodOptions and StepStatusOptions are the values Paramify accepts, in
	// the order its dropdowns list them.
	MethodOptions     = []string{"TEST", "EXAMINE", "INTERVIEW", "UNKNOWN"}
	StepStatusOptions = []string{"OPEN", "PASS", "FAIL", "PARTIAL"}
	// ReviewStatusOptions are a requirement's Review Status values, in the
	// order Paramify's Review Status dropdown lists them.
	ReviewStatusOptions = []string{"OPEN", "OVERDUE", "SUBMITTED", "ACCEPTED", "ARCHIVED",
		"CHANGE_REQUESTED", "QA_REQUESTED", "REVIEWED"}
)

// Cells are matched as a whole, never by words inside them, so prose such as
// "Use secure passwordless methods" is not read as a status ("met") or method.
// Punctuation, underscores and capitals are ignored ("CHANGE_REQUESTED",
// "Non-Compliant", "Satisfied." all work).

// methodWords are the words a method cell may be made of.
var methodWords = map[string]string{
	"test": "TEST", "tested": "TEST", "testing": "TEST",
	"examine": "EXAMINE", "examined": "EXAMINE", "examination": "EXAMINE",
	"interview": "INTERVIEW", "interviewed": "INTERVIEW", "interviews": "INTERVIEW",
	"unknown": "UNKNOWN",
}

// ParseMethod reads a method from a cell such as "Examine", "TEST" or
// "Examine, Interview". Every word in the cell must be a method; when several
// are listed, the first one is used, since an assurance step has a single
// method. Returns "" when the cell isn't a method.
func ParseMethod(cell string) string {
	words := strings.Fields(normalize(cell))
	first := ""
	for _, w := range words {
		if w == "and" || w == "or" {
			continue
		}
		m, ok := methodWords[w]
		if !ok {
			return ""
		}
		if first == "" {
			first = m
		}
	}
	return first
}

// stepStatusValues are the whole-cell values read as each step status.
var stepStatusValues = map[string]string{
	"open": "OPEN", "in progress": "OPEN", "pending": "OPEN", "not tested": "OPEN",
	"not assessed": "OPEN", "not started": "OPEN", "tbd": "OPEN",

	"pass": "PASS", "passed": "PASS", "satisfied": "PASS", "met": "PASS",
	"compliant": "PASS", "effective": "PASS", "yes": "PASS", "y": "PASS",

	"fail": "FAIL", "failed": "FAIL", "other than satisfied": "FAIL", "not satisfied": "FAIL",
	"unsatisfied": "FAIL", "not met": "FAIL", "non compliant": "FAIL", "noncompliant": "FAIL",
	"not compliant": "FAIL", "not effective": "FAIL", "deficient": "FAIL", "deficiency": "FAIL",
	"no": "FAIL", "n": "FAIL",

	"partial": "PARTIAL", "partially": "PARTIAL", "partially satisfied": "PARTIAL",
	"partially met": "PARTIAL", "partially compliant": "PARTIAL", "partially implemented": "PARTIAL",
}

// ParseStepStatus reads a step status from a cell that is exactly a known
// value, such as "Satisfied", "Pass", "Partial" or "Other Than Satisfied".
// Returns "" when the cell isn't one.
func ParseStepStatus(cell string) string {
	return stepStatusValues[normalize(cell)]
}

// Unrecognized returns the distinct non-blank values in a column that parse
// can't read, so the review screen can flag them before the file is written.
func Unrecognized(rows [][]string, col int, parse func(string) string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range columnValues(rows, col) {
		if parse(v) == "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// requirementStatusValues are the whole-cell values read as each review status.
var requirementStatusValues = map[string]string{
	"open": "OPEN", "in progress": "OPEN", "not started": "OPEN", "draft": "OPEN",
	"to do": "OPEN", "todo": "OPEN",

	"submitted": "SUBMITTED", "in review": "SUBMITTED", "under review": "SUBMITTED",
	"ready for review": "SUBMITTED", "pending review": "SUBMITTED",

	"accepted": "ACCEPTED", "approved": "ACCEPTED",

	"overdue": "OVERDUE", "past due": "OVERDUE",

	"archived": "ARCHIVED",

	"qa requested": "QA_REQUESTED", "qa request": "QA_REQUESTED",

	"reviewed": "REVIEWED",

	"change requested": "CHANGE_REQUESTED", "changes requested": "CHANGE_REQUESTED",
	"change request": "CHANGE_REQUESTED", "rejected": "CHANGE_REQUESTED",
	"not accepted": "CHANGE_REQUESTED", "returned": "CHANGE_REQUESTED",
	"needs changes": "CHANGE_REQUESTED", "rework": "CHANGE_REQUESTED",
}

// ParseRequirementStatus reads a review status from a cell that is
// exactly a known value, such as "Submitted", "Accepted" or "Changes
// Requested". Returns "" when the cell isn't one.
func ParseRequirementStatus(cell string) string {
	return requirementStatusValues[normalize(cell)]
}

var nonWord = regexp.MustCompile(`[^a-z]+`)

// normalize lowercases a cell and turns punctuation, underscores and runs of
// spaces into single spaces.
func normalize(v string) string {
	return strings.TrimSpace(nonWord.ReplaceAllString(strings.ToLower(v), " "))
}
