package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// An assessment's own properties (name, targetType, target, program,
// mechanism, cycleFrequency, and anything Paramify adds later) are carried
// from the download to the output exactly as they were: same keys, same
// order, same values and types. Only programRequirementAssessments is
// rewritten. The typed fields on Assessment are read-only views of those
// properties for the screens and checks.

// assessmentProperty is one top-level key of an assessment and its raw value.
type assessmentProperty struct {
	key   string
	value json.RawMessage
}

const requirementsKey = "programRequirementAssessments"

func (a *Assessment) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return fmt.Errorf("an assessment must be a JSON object")
	}
	var props []assessmentProperty
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return fmt.Errorf("reading %q: %w", key, err)
		}
		props = append(props, assessmentProperty{key, value})
	}

	*a = Assessment{properties: props}
	text := func(raw json.RawMessage) string {
		var s string
		_ = json.Unmarshal(raw, &s) // null or non-text values read as ""
		return s
	}
	for _, p := range props {
		switch p.key {
		case "name":
			a.Name = text(p.value)
		case "targetType":
			a.TargetType = text(p.value)
		case "target":
			a.Target = text(p.value)
		case "program":
			a.Program = text(p.value)
		case "mechanism":
			a.Mechanism = text(p.value)
		case "cycleFrequency":
			a.CycleFrequency = text(p.value)
		case requirementsKey:
			if err := json.Unmarshal(p.value, &a.ProgramRequirementAssessments); err != nil {
				return fmt.Errorf("reading %s: %w", requirementsKey, err)
			}
		}
	}
	return nil
}

func (a Assessment) MarshalJSON() ([]byte, error) {
	// Built by the tool rather than read from a download: write the fields
	if a.properties == nil {
		type plain Assessment // drops the methods, so this doesn't recurse
		return marshalNoEscape(plain(a))
	}

	reqs, err := marshalNoEscape(a.ProgramRequirementAssessments)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteByte('{')
	wroteReqs := false
	for i, p := range a.properties {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := marshalNoEscape(p.key)
		b.Write(key)
		b.WriteByte(':')
		if p.key == requirementsKey {
			b.Write(reqs)
			wroteReqs = true
		} else {
			b.Write(p.value)
		}
	}
	if !wroteReqs {
		if len(a.properties) > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"` + requirementsKey + `":`)
		b.Write(reqs)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshalNoEscape is json.Marshal without turning &, < and > into &
// etc., so text such as "R&D Program" is written the way it was read.
func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
