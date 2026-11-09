package main

// Step represents a screen in the TUI flow.
type Step int

const (
	StepWelcome Step = iota
	StepCSVFilePicker
	StepExistingJSONFilePicker
	StepAssessmentSelect
	StepCSVPreview
	StepIDColumnSelect
	StepIDPrefixStrip
	StepColumnMapping
	StepMetadata
	StepOutputPath
	StepReview
	StepGenerating
	StepDone
	StepError
)

// MappingTarget represents what a CSV column maps to.
type MappingTarget int

const (
	MapSkip MappingTarget = iota
	MapID
	MapPlan
	MapResult
	MapRemediation
	MapMethod     // per-row assurance step method (TEST / EXAMINE / INTERVIEW / UNKNOWN)
	MapStepStatus // per-row assurance step status (OPEN / PASS / FAIL / PARTIAL)
	MapAssigned   // per-row assessor assigned to the step
	MapOwner      // per-row assurance owner of the requirement
	MapReviewer   // per-row requirement reviewer
	MapStatus     // per-row review status of the requirement (OPEN / SUBMITTED / ACCEPTED / CHANGE_REQUESTED)
)

func (m MappingTarget) String() string {
	switch m {
	case MapSkip:
		return "skip"
	case MapID:
		return "id"
	case MapPlan:
		return "plan"
	case MapResult:
		return "result"
	case MapRemediation:
		return "remediation"
	case MapMethod:
		return "method"
	case MapStepStatus:
		return "step status"
	case MapAssigned:
		return "assigned"
	case MapOwner:
		return "assurance owner"
	case MapReviewer:
		return "reviewer"
	case MapStatus:
		return "review status"
	default:
		return "unknown"
	}
}

// AllMappingTargets returns the non-ID mapping targets for column assignment.
func AllMappingTargets() []MappingTarget {
	return []MappingTarget{MapSkip, MapPlan, MapResult, MapRemediation,
		MapStatus, MapMethod, MapStepStatus, MapAssigned, MapOwner, MapReviewer}
}

// MultiColumn reports whether several CSV columns can share a target.
// Plan and result stack columns (each under a bold heading named after the
// column); every other target reads a single column.
func (m MappingTarget) MultiColumn() bool {
	return m == MapResult || m == MapPlan || m == MapSkip
}
