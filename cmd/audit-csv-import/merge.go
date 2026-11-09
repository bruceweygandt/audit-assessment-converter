package main

// MergeIntoAssessment updates an existing assessment with data from CSV.
// Matching programRequirements get their assurance steps updated.
// Non-matching CSV rows are appended as new entries.
// Returns the IDs that were updated and the IDs that were appended.
func MergeIntoAssessment(existing *Assessment, csvData *CSVData, config GenerateConfig) (updated, added []string) {
	// Build lookup map of existing requirements
	praMap := make(map[string]int)
	for i, pra := range existing.ProgramRequirementAssessments {
		praMap[pra.ProgramRequirement] = i
	}

	// Generate the new assessment data from CSV
	generated := GenerateAssessment(csvData, config)

	for _, newPRA := range generated.ProgramRequirementAssessments {
		if idx, ok := praMap[newPRA.ProgramRequirement]; ok {
			updated = append(updated, newPRA.ProgramRequirement)
			// Found: update assurance steps
			if len(existing.ProgramRequirementAssessments[idx].AssuranceSteps) > 0 {
				step := newPRA.AssuranceSteps[0]
				old := existing.ProgramRequirementAssessments[idx].AssuranceSteps[0]
				// Blank values never erase what's already there: keep the existing
				// assessor, plan, result and remediation when the CSV has none
				if step.Assigned == "" {
					step.Assigned = old.Assigned
				}
				if !resultHasContent(step.Plan) {
					step.Plan = old.Plan
				}
				if !resultHasContent(step.Result) {
					step.Result = old.Result
				}
				if string(step.Remediation) == "null" || len(step.Remediation) == 0 {
					step.Remediation = old.Remediation
				}
				existing.ProgramRequirementAssessments[idx].AssuranceSteps[0] = step
			} else {
				existing.ProgramRequirementAssessments[idx].AssuranceSteps = newPRA.AssuranceSteps
			}
			if newPRA.Status != "" {
				existing.ProgramRequirementAssessments[idx].Status = newPRA.Status
			}
			// Owner/reviewer come from the row's column or the details screen;
			// when both are blank, keep what's already set
			if newPRA.Owner != "" {
				existing.ProgramRequirementAssessments[idx].Owner = newPRA.Owner
			}
			if newPRA.Reviewer != "" {
				existing.ProgramRequirementAssessments[idx].Reviewer = newPRA.Reviewer
			}
		} else {
			// Not found: append as new
			added = append(added, newPRA.ProgramRequirement)
			existing.ProgramRequirementAssessments = append(
				existing.ProgramRequirementAssessments, newPRA,
			)
		}
	}
	return updated, added
}
