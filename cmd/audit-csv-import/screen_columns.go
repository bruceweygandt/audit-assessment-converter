package main

import (
	"strings"
)

// Helpers for the fields that stack several columns (plan and result), each
// column under a bold heading named after it.

// columnsFor returns the columns mapped to t, in CSV order.
func (m *Model) columnsFor(t MappingTarget) []int {
	var cols []int
	for i, mt := range m.columnMappings {
		if mt == t {
			cols = append(cols, i)
		}
	}
	return cols
}

// rowsCovered counts the CSV rows with text in at least one of cols.
func (m *Model) rowsCovered(cols []int) int {
	filled := 0
	for _, row := range m.csvRows {
		for _, c := range cols {
			if c < len(row) && strings.TrimSpace(row[c]) != "" {
				filled++
				break
			}
		}
	}
	return filled
}

// stackOrderNote shows the order stacked columns appear in (the CSV's).
func (m *Model) stackOrderNote(t MappingTarget) string {
	cols := m.columnsFor(t)
	if len(cols) < 2 {
		return ""
	}
	var names []string
	for _, c := range cols {
		names = append(names, truncate(m.csvHeaders[c], 20))
	}
	return dimStyle.Render("  Order in Paramify (the CSV's column order): " + strings.Join(names, " → "))
}
