package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Assessment represents a Paramify assessment. When it's read from a
// download, its own properties are kept verbatim and written back unchanged
// (see assessment_json.go); the fields below are read-only views of them.
type Assessment struct {
	Name                          string                         `json:"name"`
	TargetType                    string                         `json:"targetType"`
	Target                        string                         `json:"target"`
	Program                       string                         `json:"program"`
	Mechanism                     string                         `json:"mechanism"`
	CycleFrequency                string                         `json:"cycleFrequency"`
	ProgramRequirementAssessments []ProgramRequirementAssessment `json:"programRequirementAssessments"`

	properties []assessmentProperty // every top-level key from the download, in order
}

// ProgramRequirementAssessment represents a single requirement's assessment.
type ProgramRequirementAssessment struct {
	ProgramRequirement string          `json:"programRequirement"`
	Status             string          `json:"status"`
	Owner              string          `json:"owner"`
	Reviewer           string          `json:"reviewer"`
	Remarks            json.RawMessage `json:"remarks"`
	AssuranceSteps     []AssuranceStep `json:"assuranceSteps"`
}

// AssuranceStep represents one step in the capability assurance section.
type AssuranceStep struct {
	Order       int             `json:"order"`
	Method      *string         `json:"method"`
	Status      string          `json:"status"`
	Assigned    string          `json:"assigned"`
	Plan        []RichTextNode  `json:"plan"`
	Result      []RichTextNode  `json:"result"`
	Remediation json.RawMessage `json:"remediation"`
}

// GenerateConfig holds all settings needed to generate an assessment.
type GenerateConfig struct {
	Name              string
	Target            string
	Program           string
	IDColumnIdx       int
	ColumnMappings    []MappingTarget
	DefaultStatus     string   // Used when no status column is mapped, or its cell is blank/unrecognized
	DefaultMethod     string   // Used when no method column is mapped, or its cell is blank/unrecognized
	AssignedEmail     string   // Fallback for rows whose assigned column is blank
	OwnerEmail        string   // Fallback for rows whose owner column is blank
	ReviewerEmail     string   // Fallback for rows whose reviewer column is blank
	PlanText          string   // Plan for rows whose plan column is blank (or every row when none is mapped)
	DefaultStepStatus string   // Step status for rows whose step status cell is blank or unreadable (OPEN when unset)
	IDPrefixes        []string // Prefixes to strip from CSV IDs (e.g. "FRR-", "KSI-")
}

// GenerateAssessment creates a new Assessment from CSV data and config.
func GenerateAssessment(csvData *CSVData, config GenerateConfig) *Assessment {
	assessment := &Assessment{
		Name:           config.Name,
		TargetType:     "PROGRAM",
		Target:         config.Target,
		Program:        config.Program,
		Mechanism:      "",
		CycleFrequency: "",
	}

	// Identify mapped column indices
	var resultCols, planCols []int
	singleCols := make(map[MappingTarget]int) // remediation, method, ... → column
	for i, m := range config.ColumnMappings {
		switch {
		case m == MapResult:
			resultCols = append(resultCols, i)
		case m == MapPlan:
			planCols = append(planCols, i)
		case m != MapSkip && m != MapID:
			singleCols[m] = i
		}
	}
	remediationColIdx, hasRemediationCol := singleCols[MapRemediation]
	if !hasRemediationCol {
		remediationColIdx = -1
	}

	for _, row := range csvData.Rows {
		if config.IDColumnIdx >= len(row) {
			continue
		}
		// cell returns this row's trimmed value for a mapped target, or "".
		cell := func(t MappingTarget) string {
			if idx, ok := singleCols[t]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		idValue := row[config.IDColumnIdx]
		// Strip configured prefixes from the ID
		for _, prefix := range config.IDPrefixes {
			if len(idValue) > len(prefix) && idValue[:len(prefix)] == prefix {
				idValue = idValue[len(prefix):]
				break
			}
		}

		// columnValues pairs each mapped column's header with this row's cell
		sections := func(cols []int) []ColumnValue {
			var out []ColumnValue
			for _, idx := range cols {
				// Skip any mappings that fall outside the header range to avoid panics
				if idx < 0 || idx >= len(csvData.Headers) {
					continue
				}
				val := ""
				if idx < len(row) {
					val = row[idx]
				}
				out = append(out, ColumnValue{Name: csvData.Headers[idx], Value: val})
			}
			return out
		}

		// Result and plan: each mapped column becomes a bold heading plus its text
		result := BuildSectionedRichText(sections(resultCols))

		var plan []RichTextNode
		if len(planCols) > 0 {
			plan = BuildSectionedRichText(sections(planCols))
			if !resultHasContent(plan) {
				plan = nil // every plan cell blank: fall through to the typed text
			}
		}
		if plan == nil && config.PlanText != "" {
			plan = TextToParagraphs(config.PlanText)
		}
		if plan == nil {
			plan = EmptyParagraph()
		}

		// Build remediation
		var remediationJSON json.RawMessage
		if remediationColIdx >= 0 && remediationColIdx < len(row) {
			nodes := BuildFieldRichText(row[remediationColIdx])
			if nodes != nil {
				data, _ := json.Marshal(nodes)
				remediationJSON = data
			}
		}
		if remediationJSON == nil {
			remediationJSON = json.RawMessage("null")
		}

		// Method: the row's method column first, then the default
		var method *string
		if m := ParseMethod(cell(MapMethod)); m != "" {
			method = &m
		} else if config.DefaultMethod != "" {
			m := config.DefaultMethod
			method = &m
		}

		// Step status: the row's status column first, then the Details value
		stepStatus := ParseStepStatus(cell(MapStepStatus))
		if stepStatus == "" {
			stepStatus = config.DefaultStepStatus
		}
		if stepStatus == "" {
			stepStatus = "OPEN"
		}

		// People: the row's column first, then the value from the details screen
		orDefault := func(v, def string) string {
			if v != "" {
				return v
			}
			return def
		}

		pra := ProgramRequirementAssessment{
			ProgramRequirement: idValue,
			Status:             orDefault(ParseRequirementStatus(cell(MapStatus)), config.DefaultStatus),
			Owner:              orDefault(cell(MapOwner), config.OwnerEmail),
			Reviewer:           orDefault(cell(MapReviewer), config.ReviewerEmail),
			Remarks:            json.RawMessage("null"),
			AssuranceSteps: []AssuranceStep{
				{
					Order:       0,
					Method:      method,
					Status:      stepStatus,
					Assigned:    orDefault(cell(MapAssigned), config.AssignedEmail),
					Plan:        plan,
					Result:      result,
					Remediation: remediationJSON,
				},
			},
		}

		assessment.ProgramRequirementAssessments = append(
			assessment.ProgramRequirementAssessments, pra,
		)
	}

	return assessment
}

// resultHasContent checks if a rich text result field has any real text content
// (not just empty paragraphs).
func resultHasContent(nodes []RichTextNode) bool {
	for _, node := range nodes {
		for _, child := range node.Children {
			if child.Text != "" || child.IsLink {
				return true
			}
		}
	}
	return false
}

// LoadAssessments reads an existing assessments JSON file.
func LoadAssessments(path string) ([]Assessment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	var assessments []Assessment
	if err := json.Unmarshal(data, &assessments); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	// Paramify renamed projectRequirementAssessments to programRequirementAssessments.
	// Older files would otherwise load with zero requirements.
	if bytes.Contains(data, []byte(`"projectRequirementAssessments"`)) {
		return nil, fmt.Errorf("this file uses the old projectRequirementAssessments format; download the audit assessments from Paramify again and use the new file")
	}
	return assessments, nil
}

// WriteAssessments writes assessments to a JSON file with indentation.
func WriteAssessments(path string, assessments []Assessment) error {
	compact, err := marshalNoEscape(assessments)
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	var data bytes.Buffer
	if err := json.Indent(&data, compact, "", "  "); err != nil {
		return fmt.Errorf("formatting JSON: %w", err)
	}
	return os.WriteFile(path, data.Bytes(), 0644)
}
