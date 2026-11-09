package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// topLevel returns an assessment's own properties (everything except the
// requirements) as ordered key/value pairs, decoded generically.
func topLevel(t *testing.T, raw json.RawMessage) [][2]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	dec.Token() // {
	var out [][2]any
	for dec.More() {
		k, _ := dec.Token()
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		if k != requirementsKey {
			out = append(out, [2]any{k, v})
		}
	}
	return out
}

// TestMerge_CopiesAssessmentPropertiesExactly: whatever the download holds at
// the assessment level, the output carries it unchanged, for whichever
// assessment is picked. Only the requirements change.
func TestMerge_CopiesAssessmentPropertiesExactly(t *testing.T) {
	dir := t.TempDir()
	download := filepath.Join(dir, "audit-assessments.json")
	os.WriteFile(download, []byte(`[
	  {"name":"First","targetType":"PROGRAM","target":"P1","program":"P1","mechanism":"","cycleFrequency":"",
	   "programRequirementAssessments":[]},
	  {"id":"4f1c9a2e-7d3b-4c8e-9a10-0b2c3d4e5f60",
	   "programRequirementAssessments":[{"programRequirement":"A-1","status":"OPEN","owner":"","reviewer":"","remarks":null,"assuranceSteps":[]}],
	   "name":"R&D <Pilot> \"Q4\" – Évaluation","targetType":"PROGRAM","target":"SOC 2 AICPA","program":"SOC 2 AICPA",
	   "mechanism":null,"cycleFrequency":"ANNUALLY","schedule":{"every":3,"unit":"MONTHS"},"tags":["a","b"]},
	  {"name":"Third","targetType":"PROGRAM","target":"P3","program":"P3","mechanism":"","cycleFrequency":"",
	   "programRequirementAssessments":[]}
	]`), 0644)

	var src []json.RawMessage
	data, _ := os.ReadFile(download)
	json.Unmarshal(data, &src)

	m := NewModel()
	m.jsonPath = download
	var err error
	if m.existingData, err = LoadAssessments(download); err != nil {
		t.Fatalf("a download with null/object/extra properties should load: %v", err)
	}
	m.selectedAssessmentIdx = 1
	m.csvHeaders = []string{"ID", "Notes"}
	m.csvRows = [][]string{{"A-1", "merged"}}
	m.columnMappings = []MappingTarget{MapID, MapResult}
	m.initMetadata()
	m.outputFilePath = filepath.Join(dir, "out.json")
	if msg := m.generateCmd()().(generateDoneMsg); msg.err != nil {
		t.Fatal(msg.err)
	}

	var out []json.RawMessage
	data, _ = os.ReadFile(m.outputFilePath)
	if err := json.Unmarshal(data, &out); err != nil || len(out) != 1 {
		t.Fatalf("output should hold exactly one assessment: %v (%d)", err, len(out))
	}
	if got, want := topLevel(t, out[0]), topLevel(t, src[1]); !reflect.DeepEqual(got, want) {
		t.Errorf("assessment properties changed:\n got  %v\n want %v", got, want)
	}
	// Key order is kept too, requirements included in their original position
	var keys []string
	dec := json.NewDecoder(bytes.NewReader(out[0]))
	dec.Token()
	for dec.More() {
		k, _ := dec.Token()
		keys = append(keys, k.(string))
		var skip json.RawMessage
		dec.Decode(&skip)
	}
	want := []string{"id", requirementsKey, "name", "targetType", "target", "program", "mechanism", "cycleFrequency", "schedule", "tags"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("key order = %v, want %v", keys, want)
	}
	// Text is written as read: no &-style escaping of & < >
	if !bytes.Contains(data, []byte(`R&D <Pilot> \"Q4\" – Évaluation`)) {
		t.Errorf("name should be written verbatim:\n%s", data)
	}
	// And the requirement was actually merged
	written, _ := LoadAssessments(m.outputFilePath)
	if r := plainRich(written[0].ProgramRequirementAssessments[0].AssuranceSteps[0].Result); r != "Notes / merged" {
		t.Errorf("merged result = %q", r)
	}
}
