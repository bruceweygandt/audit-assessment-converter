package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// The live preview pane: the CSV as a spreadsheet grid on top, and below it the
// JSON entry the previewed row becomes, recomputed on every keypress with the
// same generate/merge code that writes the file.

const minPreviewWidth = 130 // narrower terminals show the steps only

var (
	previewPaneStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderLeft(true).
				BorderForeground(lipgloss.Color("#4B5563")).
				PaddingLeft(1)

	previewSectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A78BFA")).
				Bold(true)

	previewHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FBBF24")).
				Bold(true)
)

// Paramify fields shown in the preview, and the column type that fills each.
const (
	fieldID          = "id"
	fieldStatus      = "status"
	fieldOwner       = "owner"
	fieldReviewer    = "reviewer"
	fieldMethod      = "method"
	fieldStepStatus  = "step status"
	fieldAssigned    = "assigned"
	fieldPlan        = "plan"
	fieldResult      = "result"
	fieldRemediation = "remediation"
)

var targetField = map[MappingTarget]string{
	MapID: fieldID, MapStatus: fieldStatus, MapOwner: fieldOwner, MapReviewer: fieldReviewer,
	MapMethod: fieldMethod, MapStepStatus: fieldStepStatus, MapAssigned: fieldAssigned,
	MapPlan: fieldPlan, MapResult: fieldResult, MapRemediation: fieldRemediation,
}

// detailsField maps details-screen fields to preview fields.
var detailsField = map[int]string{
	metaStatus: fieldStatus, metaMethod: fieldMethod, metaStepStatus: fieldStepStatus,
	metaAssigned: fieldAssigned, metaOwner: fieldOwner, metaReviewer: fieldReviewer,
	metaPlanText: fieldPlan,
}

// previewFocus is what the current screen is pointing at.
type previewFocus struct {
	column int    // CSV column to highlight, or -1
	field  string // Paramify field to highlight, or ""
}

// previewStep reports whether this screen shows the pane (the CSV is loaded
// and there's a row to turn into a requirement).
func (m *Model) previewStep() bool {
	switch m.step {
	case StepIDColumnSelect, StepIDPrefixStrip, StepColumnMapping, StepMetadata,
		StepOutputPath, StepReview:
		return len(m.csvRows) > 0
	}
	return false
}

func (m *Model) previewWanted() bool {
	return m.previewStep() && !m.previewHidden
}

func (m *Model) previewActive() bool {
	return m.previewWanted() && m.width >= minPreviewWidth
}

// typingText reports whether keys currently go into a text box, so preview
// shortcuts like [ and ] don't steal typed characters.
func (m *Model) typingText() bool {
	switch m.step {
	case StepIDPrefixStrip, StepOutputPath:
		return true
	case StepMetadata:
		return !isCyclingField(m.metadata.focused)
	}
	return false
}

// movePreviewRow steps the previewed CSV row.
func (m *Model) movePreviewRow(delta int) {
	if n := len(m.csvRows); n > 0 {
		m.previewRow = (m.previewRow + delta + n) % n
	}
}

// livePreviewConfig builds the config the file would be generated with if the
// user confirmed what's on screen right now, plus what the screen points at.
func (m *Model) livePreviewConfig() (GenerateConfig, previewFocus) {
	cfg := m.generateConfig()
	cfg.ColumnMappings = append([]MappingTarget(nil), m.columnMappings...)
	focus := previewFocus{column: -1}

	// Details values: live from the form once it exists, else the form's defaults
	cfg.DefaultStatus, cfg.DefaultMethod, cfg.DefaultStepStatus = statusOptions[0], methodOptions[1], StepStatusOptions[0]
	cfg.AssignedEmail, cfg.OwnerEmail, cfg.ReviewerEmail = "", "", ""
	if md := m.metadata; len(md.inputs) == metaFieldCount {
		cfg.DefaultStatus = statusOptions[md.statusIdx]
		cfg.DefaultMethod = methodOptions[md.methodIdx]
		cfg.DefaultStepStatus = StepStatusOptions[md.stepStatusIdx]
		cfg.AssignedEmail = md.inputs[metaAssigned].Value()
		cfg.OwnerEmail = md.inputs[metaOwner].Value()
		cfg.ReviewerEmail = md.inputs[metaReviewer].Value()
	}
	if md := m.metadata; len(md.inputs) == metaFieldCount {
		cfg.PlanText = md.inputs[metaPlanText].Value()
	}

	idOnly := func(col int) []MappingTarget {
		mappings := make([]MappingTarget, len(m.csvHeaders))
		if col >= 0 && col < len(mappings) {
			mappings[col] = MapID
		}
		return mappings
	}

	switch m.step {
	case StepIDColumnSelect:
		cfg.IDColumnIdx = m.idColumnCursor
		cfg.ColumnMappings = idOnly(m.idColumnCursor)
		// Show the match the next step's suggested prefixes would give
		cfg.IDPrefixes = SuggestPrefixes(columnValues(m.csvRows, m.idColumnCursor), m.existingRequirementIDs())
		focus = previewFocus{m.idColumnCursor, fieldID}
	case StepIDPrefixStrip:
		cfg.ColumnMappings = idOnly(m.idColumnIdx)
		cfg.IDPrefixes = parsePrefixes(m.idPrefixInput.Value())
		focus = previewFocus{m.idColumnIdx, fieldID}
	case StepColumnMapping:
		col := m.mappingColumns[m.mappingCursor]
		hovered := AllMappingTargets()[m.mappingTargetCursor]
		if !hovered.MultiColumn() {
			for i, t := range cfg.ColumnMappings {
				if t == hovered {
					cfg.ColumnMappings[i] = MapSkip
				}
			}
		}
		cfg.ColumnMappings[col] = hovered
		focus = previewFocus{col, targetField[hovered]}
	case StepMetadata:
		f := m.metadata.focused
		if src, ok := metaSource[f]; ok {
			f = src.field
		}
		focus.field = detailsField[f]
		for t, field := range targetField {
			if field == focus.field {
				focus.column = m.targetColumn(t)
			}
		}
	}
	return cfg, focus
}

// simulateRow runs one CSV row through the real generate/merge code. before is
// the requirement as it is in the download (nil when new).
func (m *Model) simulateRow(cfg GenerateConfig, row []string) (after ProgramRequirementAssessment, before *ProgramRequirementAssessment, ok bool) {
	data := &CSVData{Headers: m.csvHeaders, Rows: [][]string{row}}
	gen := GenerateAssessment(data, cfg)
	if len(gen.ProgramRequirementAssessments) == 0 {
		return after, nil, false
	}
	after = gen.ProgramRequirementAssessments[0]
	if m.selectedAssessmentIdx >= len(m.existingData) {
		return after, nil, true
	}
	for _, pra := range m.existingData[m.selectedAssessmentIdx].ProgramRequirementAssessments {
		if pra.ProgramRequirement != after.ProgramRequirement {
			continue
		}
		// Merge into a copy so the real data is untouched
		cp := pra
		cp.AssuranceSteps = append([]AssuranceStep(nil), pra.AssuranceSteps...)
		tmp := &Assessment{ProgramRequirementAssessments: []ProgramRequirementAssessment{cp}}
		MergeIntoAssessment(tmp, data, cfg)
		original := pra
		return tmp.ProgramRequirementAssessments[0], &original, true
	}
	return after, nil, true
}

// plainRich flattens rich text to one line of plain text.
func plainRich(nodes []RichTextNode) string {
	var parts []string
	for _, n := range nodes {
		var line strings.Builder
		for _, c := range n.Children {
			if c.IsLink {
				line.WriteString(c.LinkText)
			} else {
				line.WriteString(c.Text)
			}
		}
		if s := strings.TrimSpace(line.String()); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " / ")
}

func rawRich(raw json.RawMessage) []RichTextNode {
	var nodes []RichTextNode
	_ = json.Unmarshal(raw, &nodes)
	return nodes
}

var (
	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	diffRemoveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	gridHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	gridRowNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	gridRowStyle    = lipgloss.NewStyle().Underline(true)
)

// jsonLine is one line of a requirement's JSON and the preview field it belongs to.
type jsonLine struct {
	text  string
	field string
}

// requirementJSON pretty-prints a requirement exactly as the file stores it,
// tagging each line with the field it belongs to so the focused one can be lit up.
func requirementJSON(p ProgramRequirementAssessment) []jsonLine {
	data, _ := json.MarshalIndent(p, "", "  ")
	reqFields := map[string]string{`"status"`: fieldStatus, `"owner"`: fieldOwner, `"reviewer"`: fieldReviewer, `"programRequirement"`: fieldID}
	stepFields := map[string]string{`"method"`: fieldMethod, `"status"`: fieldStepStatus, `"assigned"`: fieldAssigned,
		`"plan"`: fieldPlan, `"result"`: fieldResult, `"remediation"`: fieldRemediation}

	var out []jsonLine
	reqField, stepField, step := "", "", -1
	for _, line := range strings.Split(string(data), "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		key := ""
		if i := strings.Index(trimmed, ":"); i > 0 && strings.HasPrefix(trimmed, `"`) {
			key = trimmed[:i]
		}
		switch {
		case indent == 2 && key != "":
			reqField, stepField = reqFields[key], ""
		case indent == 4 && trimmed == "{":
			step++ // an assurance step starts
			stepField = ""
		case indent == 6 && key != "":
			stepField = ""
			if step == 0 {
				stepField = stepFields[key]
			}
		}
		field := reqField
		if indent >= 6 || (indent == 4 && trimmed != "{") {
			field = stepField
		}
		out = append(out, jsonLine{line, field})
	}
	return out
}

// diffOp is one line of a line diff: ' ' unchanged, '-' removed, '+' added.
type diffOp struct {
	op   byte
	line jsonLine
}

// diffLines is a longest-common-subsequence line diff, fine for one requirement.
func diffLines(a, b []jsonLine) []diffOp {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i].text == b[j].text {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var out []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i].text == b[j].text:
			out = append(out, diffOp{' ', b[j]})
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, diffOp{'-', a[i]})
			i++
		default:
			out = append(out, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffOp{'+', b[j]})
	}
	return out
}

// fitCell flattens and pads/truncates a cell to exactly w characters.
func fitCell(v string, w int) string {
	v = strings.Join(strings.Fields(v), " ")
	return fmt.Sprintf("%-*s", w, truncate(v, w))
}

// viewCSVGrid renders the CSV like a spreadsheet: row numbers, the header row,
// a row showing each column's mapping, then data rows around the previewed row.
func (m *Model) viewCSVGrid(cfg GenerateConfig, focus previewFocus, width, height int) string {
	const numW = 4
	// Columns widen to fill the pane when there are few of them (14–30 chars)
	colW := 14
	if n := len(m.csvHeaders); n > 0 {
		colW = (width-numW-1)/n - 1
	}
	if colW < 14 {
		colW = 14
	}
	if colW > 30 {
		colW = 30
	}
	fit := (width - numW - 1) / (colW + 1)
	if fit < 1 {
		fit = 1
	}
	anchor := focus.column
	if anchor < 0 {
		anchor = 0
	}
	c0, c1 := visibleWindow(anchor, len(m.csvHeaders), fit)

	var b strings.Builder
	b.WriteString(previewSectionStyle.Render("CSV · "+filepath.Base(m.csvPath)) + "  " +
		dimStyle.Render(fmt.Sprintf("%d rows × %d columns", len(m.csvRows), len(m.csvHeaders))))
	if c0 > 0 || c1 < len(m.csvHeaders) {
		b.WriteString(dimStyle.Render(fmt.Sprintf(" · showing columns %d–%d", c0+1, c1)))
	}
	b.WriteString("\n")

	cell := func(col int, text string, style lipgloss.Style) string {
		t := fitCell(text, colW)
		if col == focus.column {
			return previewHighlightStyle.Render(t)
		}
		return style.Render(t)
	}
	plain := lipgloss.NewStyle()

	// Header row and mapping row
	var hdr, maps strings.Builder
	hdr.WriteString(gridRowNumStyle.Render(fmt.Sprintf("%-*s", numW, "")))
	maps.WriteString(gridRowNumStyle.Render(fmt.Sprintf("%-*s", numW, "")))
	for c := c0; c < c1; c++ {
		hdr.WriteString(" " + cell(c, m.csvHeaders[c], gridHeaderStyle))
		t := MapSkip
		if c < len(cfg.ColumnMappings) {
			t = cfg.ColumnMappings[c]
		}
		label := fitCell("· skip", colW)
		style := dimStyle
		if t != MapSkip {
			label, style = fitCell("→ "+t.String(), colW), mappingColor(t).UnsetPadding()
		}
		if c == focus.column {
			style = previewHighlightStyle
		}
		maps.WriteString(" " + style.Render(label))
	}
	b.WriteString(hdr.String() + "\n" + maps.String() + "\n")

	// Data rows around the previewed row
	rows := height - 3
	if rows < 1 {
		rows = 1
	}
	r0, r1 := visibleWindow(m.previewRow, len(m.csvRows), rows)
	for r := r0; r < r1; r++ {
		row := m.csvRows[r]
		num := fmt.Sprintf("%-*d", numW, r+2) // spreadsheet row number (header is row 1)
		style := plain
		if r == m.previewRow {
			num = fmt.Sprintf("%-*s", numW, fmt.Sprintf("▶%d", r+2))
			style = gridRowStyle
		}
		b.WriteString(gridRowNumStyle.Render(num))
		for c := c0; c < c1; c++ {
			v := ""
			if c < len(row) {
				v = row[c]
			}
			if r == m.previewRow && c == focus.column {
				b.WriteString(" " + previewHighlightStyle.Underline(true).Render(fitCell(v, colW)))
				continue
			}
			b.WriteString(" " + cell(c, v, style))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// viewJSON renders the requirement's entry in the output file, as a diff
// against the download, scrolled to the focused field.
func (m *Model) viewJSON(after ProgramRequirementAssessment, before *ProgramRequirementAssessment, focus previewFocus, width, height int) string {
	outName := filepath.Base(m.outputFilePath)
	if m.outputFilePath == "" {
		outName = filepath.Base(m.defaultOutputPath(time.Now()))
	}

	var ops []diffOp
	where := ""
	afterLines := requirementJSON(after)
	idStep := m.step == StepIDColumnSelect || m.step == StepIDPrefixStrip
	switch {
	case before != nil && idStep:
		// Nothing is mapped yet: show the matched entry as it is in the download
		for _, l := range requirementJSON(*before) {
			ops = append(ops, diffOp{' ', l})
		}
		where = "✓ matches this entry in " + filepath.Base(m.jsonPath)
	case before != nil:
		ops = diffLines(requirementJSON(*before), afterLines)
		where = "changes vs. " + filepath.Base(m.jsonPath)
	default:
		for _, l := range afterLines {
			ops = append(ops, diffOp{'+', l})
		}
		where = "not in this assessment, will be added"
	}

	var b strings.Builder
	b.WriteString(previewSectionStyle.Render("JSON · "+outName) + "  " + dimStyle.Render(after.ProgramRequirement+" · "+where) + "\n")

	// Scroll so the focused field is in view
	rows := height - 1
	if rows < 3 {
		rows = 3
	}
	first := 0
	for i, op := range ops {
		if focus.field != "" && op.line.field == focus.field {
			first = i
			break
		}
	}
	start := first - 2
	if start < 0 || focus.field == "" {
		start = 0
	}
	if start+rows > len(ops) {
		start = len(ops) - rows
		if start < 0 {
			start = 0
		}
	}
	end := start + rows
	if end > len(ops) {
		end = len(ops)
	}
	if start > 0 {
		rows-- // room for the "more above" line
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ↑ %d more lines", start)) + "\n")
		if end-start > rows {
			end = start + rows
		}
	}

	textW := width - 4
	for _, op := range ops[start:end] {
		text := truncate(op.line.text, textW)
		marker := "  "
		style := lipgloss.NewStyle()
		switch op.op {
		case '+':
			if before != nil {
				marker, style = "+ ", diffAddStyle
			}
		case '-':
			marker, style = "- ", diffRemoveStyle
		}
		lit := focus.field != "" && op.line.field == focus.field
		prefix := "  "
		if lit {
			prefix = previewHighlightStyle.Render("▶ ")
			if op.op == ' ' {
				style = previewHighlightStyle
			} else {
				style = style.Bold(true)
			}
		}
		b.WriteString(prefix + style.Render(marker+text) + "\n")
	}
	if end < len(ops) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ↓ %d more lines", len(ops)-end)) + "\n")
	}
	return b.String()
}

func (m *Model) viewPreview(width int) string {
	cfg, focus := m.livePreviewConfig()
	if m.previewRow >= len(m.csvRows) {
		m.previewRow = 0
	}
	inner := width - 2 // border + padding
	height := m.height
	if height <= 0 {
		height = 50
	}
	height -= 2 // title and the gap between the two files

	var b strings.Builder
	rowKeys := "[ ] or PgUp/PgDn"
	if m.typingText() {
		rowKeys = "PgUp/PgDn" // [ and ] are typed into the text box here
	}
	b.WriteString(titleStyle.UnsetMarginBottom().Render("Live Preview") + "  " +
		dimStyle.Render(fmt.Sprintf("row %d of %d · %s change row · ctrl+p hide", m.previewRow+1, len(m.csvRows), rowKeys)) + "\n")

	// Top: the CSV as a grid. It gets up to 40% of the height.
	gridH := len(m.csvRows) + 3
	if max := height * 2 / 5; gridH > max {
		gridH = max
	}
	b.WriteString(m.viewCSVGrid(cfg, focus, inner, gridH))
	b.WriteString("\n")

	// Bottom: the JSON entry the row becomes
	row := m.csvRows[m.previewRow]
	after, before, ok := m.simulateRow(cfg, row)
	if !ok {
		b.WriteString(previewSectionStyle.Render("JSON") + "\n" + warningStyle.Render("  This row has no ID, so it is skipped.") + "\n")
		return previewPaneStyle.Width(width).Render(b.String())
	}
	b.WriteString(m.viewJSON(after, before, focus, inner, height-gridH-1))
	return previewPaneStyle.Width(width).Render(b.String())
}

// contentWidth is the width available to the step on the left.
func (m *Model) contentWidth() int {
	if m.width == 0 {
		return 100
	}
	if m.previewActive() {
		return m.width - m.previewWidth() - 1
	}
	return m.width
}

func (m *Model) previewWidth() int {
	w := m.width / 2
	if w > 120 {
		w = 120
	}
	return w
}

// withPreview places the step on the left and the live preview on the right.
func (m *Model) withPreview(left string) string {
	if !m.previewActive() {
		switch {
		case m.previewStep() && m.previewHidden && m.width >= minPreviewWidth:
			left += "\n" + dimStyle.Render("  Live preview hidden · ctrl+p shows it")
		case m.previewWanted() && m.width > 0:
			left += "\n" + dimStyle.Render(fmt.Sprintf("  Widen the window to %d+ columns to see a live preview of the CSV and Paramify side by side.", minPreviewWidth))
		}
		return left
	}
	rightW := m.previewWidth()
	leftW := m.width - rightW - 1
	leftPane := lipgloss.NewStyle().Width(leftW).MaxWidth(leftW).Render(left)
	rightPane := lipgloss.NewStyle().MaxWidth(rightW).Render(m.viewPreview(rightW))
	out := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	if m.height > 0 {
		lines := strings.Split(out, "\n")
		if len(lines) > m.height {
			out = strings.Join(lines[:m.height], "\n")
		}
	}
	return out
}

// visibleWindow returns the [start, end) slice of a list of total items that
// fits in height lines and keeps the cursor on screen.
func visibleWindow(cursor, total, height int) (int, int) {
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}
