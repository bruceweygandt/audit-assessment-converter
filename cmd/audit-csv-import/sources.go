package main

import (
	"fmt"
	"strings"
)

// Choosing, from a field's side, which CSV column supplies it per row. Used by
// the details and plan screens so users can pick a column whatever it's named.

// targetColumn returns the column mapped to t, or -1.
func (m *Model) targetColumn(t MappingTarget) int {
	for i, mt := range m.columnMappings {
		if mt == t {
			return i
		}
	}
	return -1
}

// mappedColumn returns the header of the column mapped to t, or "".
func (m *Model) mappedColumn(t MappingTarget) string {
	if i := m.targetColumn(t); i >= 0 && i < len(m.csvHeaders) {
		return m.csvHeaders[i]
	}
	return ""
}

// setTargetColumn makes col the only column mapped to t (col -1 unmaps t).
// It returns what col was mapped to before, so the screen can say what changed.
func (m *Model) setTargetColumn(t MappingTarget, col int) MappingTarget {
	for i, mt := range m.columnMappings {
		if mt == t {
			m.columnMappings[i] = MapSkip
		}
	}
	if col < 0 || col >= len(m.columnMappings) {
		return MapSkip
	}
	prev := m.columnMappings[col]
	m.columnMappings[col] = t
	return prev
}

// cycleTargetColumn moves t to the previous/next column (delta -1/+1), with
// "no column" as one of the stops. Returns a note when another field lost its column.
//
// Browsing is non-destructive: a column t passes over gets its earlier mapping
// back when t moves on (tracked in m.borrowed), so only where t stops changes.
func (m *Model) cycleTargetColumn(t MappingTarget, delta int) string {
	if m.borrowed == nil {
		m.borrowed = make(map[int]MappingTarget)
	}
	current := m.targetColumn(t)
	choices := append([]int{-1}, m.mappingColumns...)
	pos := 0
	for i, c := range choices {
		if c == current {
			pos = i
		}
	}
	col := choices[(pos+delta+len(choices))%len(choices)]

	// Release the column we're leaving, restoring what it held before
	if current >= 0 {
		m.columnMappings[current] = MapSkip
		if prev, ok := m.borrowed[current]; ok {
			m.columnMappings[current] = prev
			delete(m.borrowed, current)
		}
	}
	if col < 0 {
		return ""
	}
	prev := m.columnMappings[col]
	m.columnMappings[col] = t
	if prev != MapSkip && prev != t {
		m.borrowed[col] = prev
		return fmt.Sprintf("%q was mapped to %s; it now feeds %s instead.", m.csvHeaders[col], prev, t)
	}
	return ""
}

// sourceLabel describes where t comes from, for a "from column" row.
func (m *Model) sourceLabel(t MappingTarget) string {
	col := m.targetColumn(t)
	if col < 0 {
		return "none"
	}
	label := fmt.Sprintf("%q", truncate(m.csvHeaders[col], 22))
	if vals := columnValues(m.csvRows, col); len(vals) > 0 {
		label += "  " + dimStyle.Render(fmt.Sprintf("e.g. %q", truncate(vals[0], 14)))
	}
	return label
}

// valuesFound summarizes the distinct values in t's column, marking any the
// tool can't read, so users can tell they picked the right column.
func (m *Model) valuesFound(t MappingTarget, parse func(string) string) string {
	col := m.targetColumn(t)
	if col < 0 {
		return ""
	}
	seen := make(map[string]bool)
	var parts []string
	for _, v := range columnValues(m.csvRows, col) {
		if seen[v] || len(parts) >= 4 {
			continue
		}
		seen[v] = true
		label := truncate(v, 25)
		if parse != nil && parse(v) == "" {
			label += " (not recognized)"
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "This column is empty in every row."
	}
	return "Values in this column: " + strings.Join(parts, ", ")
}

// fieldCounts says, for one per-row field, how many CSV rows get their value
// from the column and how many fall back to the value set on the Details screen.
type fieldCounts struct {
	column     int // the field's column, or -1
	total      int
	fromColumn int // filled cells the field can use
	blank      int // empty cells (or every row, when there's no column)
	unreadable int // filled cells the field can't read (dropdown fields only)
}

// fallback is how many rows use the Details screen value.
func (c fieldCounts) fallback() int { return c.blank + c.unreadable }

// countField counts how each row's value for t will be decided. parse is nil
// for free-text fields (assigned, owner, reviewer), where any filled cell is used.
func (m *Model) countField(t MappingTarget, parse func(string) string) fieldCounts {
	c := fieldCounts{column: m.targetColumn(t), total: len(m.csvRows)}
	if c.column < 0 {
		c.blank = c.total
		return c
	}
	for _, row := range m.csvRows {
		v := ""
		if c.column < len(row) {
			v = strings.TrimSpace(row[c.column])
		}
		switch {
		case v == "":
			c.blank++
		case parse != nil && parse(v) == "":
			c.unreadable++
		default:
			c.fromColumn++
		}
	}
	return c
}

// rowCount formats "1 row" / "3 rows".
func rowCount(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}

// peopleColumnWarning returns a warning when the column picked for a people
// field (assigned, owner, reviewer) holds sentences rather than names or
// emails, since every filled cell would be used as-is.
func (m *Model) peopleColumnWarning(t MappingTarget) string {
	col := m.targetColumn(t)
	if col < 0 {
		return ""
	}
	vals := columnValues(m.csvRows, col)
	if len(vals) == 0 {
		return ""
	}
	notPeople := 0
	example := ""
	for _, v := range vals {
		if !isPersonCell(v) {
			notPeople++
			if example == "" {
				example = v
			}
		}
	}
	if float64(notPeople)/float64(len(vals)) <= 0.2 {
		return ""
	}
	return fmt.Sprintf("%q looks like text, not people (e.g. %q). Each row's %s would be set to that text.",
		m.csvHeaders[col], truncate(example, 40), t)
}

// columnWarning flags a column that's probably the wrong one for its field:
// people text that isn't people, or a dropdown column none of whose values
// can be read.
func (m *Model) columnWarning(t MappingTarget, parse func(string) string) string {
	if parse == nil {
		return m.peopleColumnWarning(t)
	}
	c := m.countField(t, parse)
	if c.column >= 0 && c.fromColumn == 0 && c.unreadable > 0 {
		return fmt.Sprintf("None of %q's values can be read as a %s, so it may be the wrong column. Every row uses the value below.",
			m.csvHeaders[c.column], t)
	}
	return ""
}
