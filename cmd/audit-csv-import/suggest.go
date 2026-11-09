package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Suggestions that pre-fill each step so most users can just press tab.

// existingRequirementIDs returns the requirement IDs of the assessment being
// merged into, or nil before one is picked.
func (m *Model) existingRequirementIDs() map[string]bool {
	if m.selectedAssessmentIdx >= len(m.existingData) {
		return nil
	}
	ids := make(map[string]bool)
	for _, pra := range m.existingData[m.selectedAssessmentIdx].ProgramRequirementAssessments {
		ids[pra.ProgramRequirement] = true
	}
	return ids
}

// columnValues returns the trimmed, non-empty values of one CSV column.
func columnValues(rows [][]string, col int) []string {
	var vals []string
	for _, row := range rows {
		if col < len(row) {
			if v := strings.TrimSpace(row[col]); v != "" {
				vals = append(vals, v)
			}
		}
	}
	return vals
}

// idLike matches values shaped like requirement IDs: KSI-CNA-03, AC-2(1), FRR-MAS-02, CC6.1.
var idLike = regexp.MustCompile(`^[A-Za-z]{1,6}[-_.]?[A-Za-z0-9]*[-_.]?[A-Za-z0-9().\- ]*\d[A-Za-z0-9().\- ]*$`)

// SuggestIDColumn picks the column most likely to hold requirement IDs.
// When existing requirement IDs are known, the column matching the most of
// them wins; otherwise the header name and value shape decide.
func SuggestIDColumn(headers []string, rows [][]string, existing map[string]bool) int {
	best, bestScore := 0, -1.0
	for col, header := range headers {
		vals := columnValues(rows, col)
		if len(vals) == 0 {
			continue
		}
		score := 0.0
		if len(existing) > 0 {
			matched := 0
			for _, v := range vals {
				if existing[v] || matchesWithPrefix(v, existing) {
					matched++
				}
			}
			score += 10 * float64(matched) / float64(len(vals))
		}
		h := strings.ToLower(header)
		for _, word := range strings.FieldsFunc(h, func(r rune) bool { return !unicode.IsLetter(r) }) {
			if word == "id" || word == "identifier" {
				score += 3
			}
		}
		shaped, unique := 0, make(map[string]bool)
		for _, v := range vals {
			if len(v) <= 40 && idLike.MatchString(v) {
				shaped++
			}
			unique[v] = true
		}
		score += 2 * float64(shaped) / float64(len(vals))
		score += float64(len(unique)) / float64(len(vals))
		if score > bestScore {
			best, bestScore = col, score
		}
	}
	return best
}

// matchesWithPrefix reports whether id is some prefix followed by an existing ID.
func matchesWithPrefix(id string, existing map[string]bool) bool {
	return prefixFor(id, existing) != ""
}

// prefixFor returns the shortest prefix that turns id into an existing ID, or "".
// Prefixes must end in a separator (KSI-, FRR-) so partial words are never cut.
func prefixFor(id string, existing map[string]bool) string {
	for i := 1; i < len(id); i++ {
		prefix := id[:i]
		last := rune(prefix[len(prefix)-1])
		if unicode.IsLetter(last) || unicode.IsDigit(last) {
			continue
		}
		if existing[id[i:]] {
			return prefix
		}
	}
	return ""
}

// SuggestPrefixes returns the prefixes that need stripping for CSV IDs to match
// existing requirement IDs, most common first.
func SuggestPrefixes(ids []string, existing map[string]bool) []string {
	counts := make(map[string]int)
	for _, id := range ids {
		if existing[id] {
			continue
		}
		if p := prefixFor(id, existing); p != "" {
			counts[p]++
		}
	}
	var prefixes []string
	for p := range counts {
		prefixes = append(prefixes, p)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		if counts[prefixes[i]] != counts[prefixes[j]] {
			return counts[prefixes[i]] > counts[prefixes[j]]
		}
		return prefixes[i] < prefixes[j]
	})
	return prefixes
}

// Column mapping suggestions are checked against each column's contents, not
// just its name, because every client's spreadsheet is laid out differently.
// Free-text columns (result, plan, remediation) can't be confirmed from their
// contents, so they are never suggested and start on skip.

// suggestThreshold is the share of filled cells that must be a known value.
const suggestThreshold = 0.8

// Columns are confirmed with the same whole-cell readers the import uses
// (ParseMethod, ParseStepStatus, ParseRequirementStatus), so a suggestion
// never claims a column the import would then fail to read.

// isYesNo spots yes/no cells: they're valid step statuses, but a yes/no
// column alone (e.g. "Inherited?") isn't proof of a step status column.
func isYesNo(v string) bool {
	switch normalize(v) {
	case "yes", "no", "y", "n":
		return true
	}
	return false
}

var emailLike = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// isPersonCell accepts an email or a short name ("Jane Doe", "Client Success").
func isPersonCell(v string) bool {
	if emailLike.MatchString(v) {
		return true
	}
	return len(v) <= 40 && len(strings.Fields(v)) <= 4 && !strings.ContainsAny(v, ".:;!?()")
}

// share returns the fraction of vals for which ok is true, and that count.
func share(vals []string, ok func(string) bool) (float64, int) {
	n := 0
	for _, v := range vals {
		if ok(v) {
			n++
		}
	}
	return float64(n) / float64(len(vals)), n
}

// headerHas reports whether a header contains any of the words.
func headerHas(header string, words ...string) bool {
	h := strings.ToLower(header)
	for _, w := range words {
		if strings.Contains(h, w) {
			return true
		}
	}
	return false
}

// Suggestion is a checked mapping suggestion and the evidence for it.
type Suggestion struct {
	Target MappingTarget
	Reason string
}

// suggestColumn checks one column's contents (and, for people, its header).
func suggestColumn(header string, vals []string) Suggestion {
	if len(vals) == 0 {
		return Suggestion{MapSkip, "the column is empty"}
	}
	evidence := func(n int, what string) string {
		return fmt.Sprintf("%d of %d filled cells are %s", n, len(vals), what)
	}

	if f, n := share(vals, func(v string) bool { return ParseMethod(v) != "" }); f >= suggestThreshold {
		return Suggestion{MapMethod, evidence(n, "Test/Examine/Interview/Unknown")}
	}

	stepF, stepN := share(vals, func(v string) bool { return ParseStepStatus(v) != "" && !isYesNo(v) })
	reqF, reqN := share(vals, func(v string) bool { return ParseRequirementStatus(v) != "" })
	stepOK, reqOK := stepF >= suggestThreshold, reqF >= suggestThreshold
	if stepOK && reqOK {
		// Values like "Open" fit both; only the header can tell them apart
		switch {
		case headerHas(header, "requirement", "review", "approval", "workflow"):
			return Suggestion{MapStatus, evidence(reqN, "review statuses") + ", and the header says requirement/review"}
		case headerHas(header, "step", "pass", "fail", "test result", "determination"):
			return Suggestion{MapStepStatus, evidence(stepN, "step statuses") + ", and the header says step/pass/fail"}
		}
		return Suggestion{MapSkip, ""}
	}
	if stepOK {
		return Suggestion{MapStepStatus, evidence(stepN, "Pass/Fail/Partial/Open values")}
	}
	if reqOK {
		return Suggestion{MapStatus, evidence(reqN, "Submitted/Accepted/Change requested values")}
	}

	// People columns need both: a header naming the role and name/email cells
	roles := []struct {
		target MappingTarget
		words  []string
	}{
		{MapReviewer, []string{"reviewer", "reviewed by"}},
		{MapOwner, []string{"owner"}},
		{MapAssigned, []string{"assigned", "assignee", "assessor email", "assessed by"}},
	}
	for _, r := range roles {
		if headerHas(header, r.words...) {
			if f, n := share(vals, isPersonCell); f >= suggestThreshold {
				return Suggestion{r.target, evidence(n, "emails or names") + ", and the header says " + r.target.String()}
			}
		}
	}
	return Suggestion{MapSkip, ""}
}

// SuggestMappings returns a checked suggestion for every column except the ID
// column. When two columns qualify for the same single-column field, neither
// is suggested, since the tool can't tell which one is meant.
func SuggestMappings(headers []string, rows [][]string, idCol int) []Suggestion {
	out := make([]Suggestion, len(headers))
	count := make(map[MappingTarget]int)
	for i, h := range headers {
		if i == idCol {
			out[i] = Suggestion{MapID, ""}
			continue
		}
		out[i] = suggestColumn(h, columnValues(rows, i))
		count[out[i].Target]++
	}
	for i, sg := range out {
		if sg.Target != MapSkip && sg.Target != MapID && count[sg.Target] > 1 {
			out[i] = Suggestion{MapSkip, ""}
		}
	}
	return out
}
