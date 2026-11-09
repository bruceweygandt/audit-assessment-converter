package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	metaStatus = iota
	metaMethod
	metaStepStatus
	metaAssigned
	metaOwner
	metaReviewer
	// "From column" rows: which CSV column supplies the field below, per row
	metaStatusSrc
	metaMethodSrc
	metaStepStatusSrc
	metaAssignedSrc
	metaOwnerSrc
	metaReviewerSrc
	metaPlanText // typed plan for rows whose plan columns are all blank
	metaFieldCount
)

// metaOrder is the on-screen (and focus) order. Each per-row field shows its
// "From column" row first, because the column wins, then the value used when
// that row's cell is blank.
var metaOrder = []int{
	metaStatusSrc, metaStatus,
	metaMethodSrc, metaMethod,
	metaStepStatusSrc, metaStepStatus,
	metaAssignedSrc, metaAssigned,
	metaOwnerSrc, metaOwner,
	metaReviewerSrc, metaReviewer,
	metaPlanText,
}

// metaSourceOf maps a per-row field to its "From column" row.
var metaSourceOf = map[int]int{
	metaStatus: metaStatusSrc, metaMethod: metaMethodSrc, metaStepStatus: metaStepStatusSrc,
	metaAssigned: metaAssignedSrc, metaOwner: metaOwnerSrc, metaReviewer: metaReviewerSrc,
}

// metaSource describes a "From column" row: the field it feeds and its column type.
var metaSource = map[int]struct {
	field  int
	target MappingTarget
	parse  func(string) string // nil for free-text fields
}{
	metaStatusSrc:     {metaStatus, MapStatus, ParseRequirementStatus},
	metaMethodSrc:     {metaMethod, MapMethod, ParseMethod},
	metaStepStatusSrc: {metaStepStatus, MapStepStatus, ParseStepStatus},
	metaAssignedSrc:   {metaAssigned, MapAssigned, nil},
	metaOwnerSrc:      {metaOwner, MapOwner, nil},
	metaReviewerSrc:   {metaReviewer, MapReviewer, nil},
}

var metaLabels = map[int]string{
	metaStatus:     "Review Status:",
	metaMethod:     "Method:",
	metaStepStatus: "Step Status:",
	metaAssigned:   "Assigned:",
	metaOwner:      "Assurance Owner:",
	metaReviewer:   "Reviewer:",
	metaPlanText:   "Plan:",
}

var (
	statusOptions = ReviewStatusOptions // Paramify's Review Status dropdown, in its order
	methodOptions = MethodOptions       // Test / Examine / Interview / Unknown, as Paramify lists them
)

type metadataModel struct {
	inputs        []textinput.Model
	focused       int
	statusIdx     int
	methodIdx     int
	stepStatusIdx int    // index into StepStatusOptions
	sourceNote    string // explains a column that switched fields
}

func (m *Model) initMetadata() {
	m.borrowed = nil // choices made while browsing on another screen are now final
	inputs := make([]textinput.Model, metaFieldCount)

	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 200
		inputs[i].Width = 70
		inputs[i].Prompt = "" // the "> " on the focused row already marks the active field
	}
	for _, f := range []int{metaAssigned, metaOwner, metaReviewer, metaPlanText} {
		inputs[f].Width = 34 // leaves room for the row count beside it
	}
	inputs[metaPlanText].CharLimit = 2000
	inputs[metaPlanText].Placeholder = "(optional) e.g. The assessor examined configurations..."
	inputs[metaPlanText].SetValue(m.planText)

	for _, f := range []int{metaAssigned, metaOwner, metaReviewer} {
		inputs[f].Placeholder = "(empty: keep what's already set)"
	}

	// Keep earlier answers when coming back to this screen
	prev := m.metadata
	statusIdx, methodIdx, stepStatusIdx := 0, 1, 0 // OPEN, EXAMINE, OPEN
	if len(prev.inputs) == metaFieldCount {
		for _, f := range []int{metaAssigned, metaOwner, metaReviewer, metaPlanText} {
			inputs[f].SetValue(prev.inputs[f].Value())
		}
		statusIdx, methodIdx, stepStatusIdx = prev.statusIdx, prev.methodIdx, prev.stepStatusIdx
	}

	m.metadata = metadataModel{
		inputs:        inputs,
		statusIdx:     statusIdx,
		methodIdx:     methodIdx,
		stepStatusIdx: stepStatusIdx,
	}
	m.focusMetadata(metaStatusSrc)
}

func (m *Model) focusMetadata(field int) {
	m.metadata.inputs[m.metadata.focused].Blur()
	m.metadata.focused = field
	if !isCyclingField(field) {
		m.metadata.inputs[field].Focus()
	}
}

func (m *Model) moveMetadataFocus(delta int) {
	pos := 0
	for i, f := range metaOrder {
		if f == m.metadata.focused {
			pos = i
		}
	}
	pos = (pos + delta + len(metaOrder)) % len(metaOrder)
	m.focusMetadata(metaOrder[pos])
}

// cycleMetadataOption handles ←/→ on a cycling field. delta is -1 or +1.
func (m *Model) cycleMetadataOption(delta int) bool {
	md := &m.metadata
	switch md.focused {
	case metaStatus:
		md.statusIdx = (md.statusIdx + delta + len(statusOptions)) % len(statusOptions)
	case metaMethod:
		md.methodIdx = (md.methodIdx + delta + len(methodOptions)) % len(methodOptions)
	case metaStepStatus:
		md.stepStatusIdx = (md.stepStatusIdx + delta + len(StepStatusOptions)) % len(StepStatusOptions)
	default:
		src, ok := metaSource[md.focused]
		if !ok {
			return false
		}
		md.sourceNote = m.cycleTargetColumn(src.target, delta)
	}
	return true
}

func (m *Model) updateMetadata(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.metadata.sourceNote = ""
		switch msg.String() {
		case "esc":
			m.mappingCursor = len(m.mappingColumns) - 1
			m.mappingTargetCursor = targetIndex(m.columnMappings[m.mappingColumns[m.mappingCursor]])
			m.step = StepColumnMapping
			return m, nil
		case "down", "enter":
			m.moveMetadataFocus(1)
			return m, nil
		case "up":
			m.moveMetadataFocus(-1)
			return m, nil
		case "left", "h":
			if isCyclingField(m.metadata.focused) && m.cycleMetadataOption(-1) {
				return m, nil
			}
		case "right", "l":
			if isCyclingField(m.metadata.focused) && m.cycleMetadataOption(1) {
				return m, nil
			}
		case "tab":
			// Populate model fields and advance
			m.defaultStatus = statusOptions[m.metadata.statusIdx]
			m.defaultMethod = methodOptions[m.metadata.methodIdx]
			m.defaultStepStatus = StepStatusOptions[m.metadata.stepStatusIdx]
			m.assignedEmail = m.metadata.inputs[metaAssigned].Value()
			m.ownerEmail = m.metadata.inputs[metaOwner].Value()
			m.reviewerEmail = m.metadata.inputs[metaReviewer].Value()
			m.planText = strings.TrimSpace(m.metadata.inputs[metaPlanText].Value())

			m.initOutputPath()
			m.step = StepOutputPath
			return m, nil
		}
	}

	// Update the focused text input (only if it's not a cycling selector)
	if !isCyclingField(m.metadata.focused) {
		var cmd tea.Cmd
		m.metadata.inputs[m.metadata.focused], cmd = m.metadata.inputs[m.metadata.focused].Update(msg)
		return m, cmd
	}

	return m, nil
}

// fallbackValue names what a field's fallback rows get, for the counts.
func (m *Model) fallbackValue(field int) string {
	switch field {
	case metaStatus:
		return statusOptions[m.metadata.statusIdx]
	case metaMethod:
		return methodOptions[m.metadata.methodIdx]
	case metaStepStatus:
		return StepStatusOptions[m.metadata.stepStatusIdx]
	}
	if v := strings.TrimSpace(m.metadata.inputs[field].Value()); v != "" {
		return fmt.Sprintf("%q", v)
	}
	return ""
}

// planCountNotes returns the counts for the Plan group: how many rows get a
// plan from the columns mapped on step 8, and how many use the typed text.
func (m *Model) planCountNotes() (srcNote, valueNote string) {
	cols := m.columnsFor(MapPlan)
	total := len(m.csvRows)
	typed := strings.TrimSpace(m.metadata.inputs[metaPlanText].Value()) != ""
	rest := func(n int) string {
		switch {
		case typed && n == 1:
			return "1 row uses this text"
		case typed:
			return rowCount(n) + " use this text"
		case n == 1:
			return "1 row keeps its current plan"
		}
		return rowCount(n) + " keep their current plan"
	}
	if len(cols) == 0 {
		return "no columns mapped", "all " + rest(total)
	}
	filled := m.rowsCovered(cols)
	srcNote = rowCount(filled) + " read from these columns"
	if filled == total {
		return srcNote, "not needed (every row has a plan)"
	}
	return srcNote, rest(total - filled)
}

// fieldCountNotes returns the counts shown beside a field's "From column"
// and value rows.
func (m *Model) fieldCountNotes(field int) (srcNote, valueNote string) {
	src := metaSource[metaSourceOf[field]]
	c := m.countField(src.target, src.parse)
	name := map[int]string{metaStatus: "review status", metaMethod: "method", metaStepStatus: "step status",
		metaAssigned: "assessor", metaOwner: "assurance owner", metaReviewer: "reviewer"}[field]

	// verb picks the singular or plural form: "1 row uses", "3 rows use"
	verb := func(n int, one, many string) string {
		if n == 1 {
			return rowCount(n) + " " + one
		}
		return rowCount(n) + " " + many
	}
	uses := func(n int) string {
		if v := m.fallbackValue(field); v != "" {
			return verb(n, "uses", "use") + " " + v
		}
		return verb(n, "keeps its", "keep their") + " current " + name
	}
	if c.column < 0 {
		if c.total == 0 {
			return "", ""
		}
		return "no column picked", "all " + uses(c.total)
	}
	srcNote = rowCount(c.fromColumn) + " read from this column"
	switch {
	case c.fallback() == 0:
		valueNote = "not needed (every row has a value)"
	default:
		valueNote = uses(c.fallback())
		var why []string
		if c.blank > 0 {
			why = append(why, fmt.Sprintf("%d blank", c.blank))
		}
		if c.unreadable > 0 {
			why = append(why, fmt.Sprintf("%d not recognized", c.unreadable))
		}
		valueNote += " (" + strings.Join(why, ", ") + ")"
	}
	return srcNote, valueNote
}

func (m *Model) viewMetadata() string {
	var b strings.Builder
	b.WriteString(m.header("Assessment Details"))
	b.WriteString(guide(
		"Each field reads its CSV column first (\"From column\"). The value under it",
		"is used only for rows where that cell is blank or can't be read (\"If blank\"),",
		"or for every row when no column is picked (\"Every row\"). The counts on the",
		"right show what each row will get.",
	))

	const valueW = 46 // the counts line up after this many characters
	pad := func(s string) string {
		if w := lipgloss.Width(s); w < valueW {
			return s + strings.Repeat(" ", valueW-w)
		}
		return s + " "
	}

	if m.selectedAssessmentIdx < len(m.existingData) {
		a := m.existingData[m.selectedAssessmentIdx]
		b.WriteString(fmt.Sprintf("  %s %s  %s\n", labelStyle.Render("Assessment:"), a.Name, dimStyle.Render("· program "+a.Program+" (from your download)")))
		b.WriteString("\n")
	}

	for _, f := range metaOrder {
		focused := f == m.metadata.focused
		prefix := "  "
		if focused {
			prefix = cursorStyle.Render("> ")
		}
		render := func(s string) string {
			if focused {
				return activeFieldStyle.Render(s)
			}
			return s
		}

		// "From column" row, under the field's heading
		if src, ok := metaSource[f]; ok {
			b.WriteString("  " + activeFieldStyle.Render(strings.TrimSuffix(metaLabels[src.field], ":")) + "\n")
			srcNote, _ := m.fieldCountNotes(src.field)
			line := render("◀ ") + m.sourceLabel(src.target) + render(" ▶")
			b.WriteString(fmt.Sprintf("%s  %s %s%s\n", prefix, labelStyle.Render("From column:"), pad(line), dimStyle.Render(srcNote)))
			if w := m.columnWarning(src.target, src.parse); w != "" {
				b.WriteString("      " + warningStyle.Render("! "+w) + "\n")
			}
			continue
		}

		// Plan: its columns are chosen on the mapping step; the typed text fills the rest
		if f == metaPlanText {
			srcNote, valueNote := m.planCountNotes()
			b.WriteString("  " + activeFieldStyle.Render("Plan") + "\n")
			from := "none (map columns to plan on step 8)"
			if cols := m.columnsFor(MapPlan); len(cols) > 0 {
				var names []string
				for _, c := range cols {
					names = append(names, fmt.Sprintf("%q", truncate(m.csvHeaders[c], 18)))
				}
				from = strings.Join(names, " + ")
			}
			b.WriteString(fmt.Sprintf("    %s %s%s\n", labelStyle.Render("From columns:"), pad(from), dimStyle.Render(srcNote)))
			label := "If blank:"
			if len(m.columnsFor(MapPlan)) == 0 {
				label = "Every row:"
			}
			b.WriteString(fmt.Sprintf("%s  %s %s%s\n", prefix, labelStyle.Render(label), pad(m.metadata.inputs[f].View()), dimStyle.Render(valueNote)))
			continue
		}

		// The value used when the column doesn't supply one
		label := "If blank:"
		if m.targetColumn(metaSource[metaSourceOf[f]].target) < 0 {
			label = "Every row:"
		}
		var value string
		switch f {
		case metaStatus:
			value = render(fmt.Sprintf("◀ %s ▶", statusOptions[m.metadata.statusIdx]))
		case metaMethod:
			value = render(fmt.Sprintf("◀ %s ▶", methodOptions[m.metadata.methodIdx]))
		case metaStepStatus:
			value = render(fmt.Sprintf("◀ %s ▶", StepStatusOptions[m.metadata.stepStatusIdx]))
		default:
			value = m.metadata.inputs[f].View()
		}
		_, valueNote := m.fieldCountNotes(f)
		b.WriteString(fmt.Sprintf("%s  %s %s%s\n", prefix, labelStyle.Render(label), pad(value), dimStyle.Render(valueNote)))
	}

	if m.metadata.sourceNote != "" {
		b.WriteString("\n  " + warningStyle.Render(m.metadata.sourceNote) + "\n")
	}

	// Explain the focused row
	title := strings.TrimSuffix(metaLabels[m.metadata.focused], ":")
	if src, ok := metaSource[m.metadata.focused]; ok {
		title = strings.TrimSuffix(metaLabels[src.field], ":") + " · From column"
	} else if srcRow, perRow := metaSourceOf[m.metadata.focused]; perRow {
		if m.targetColumn(metaSource[srcRow].target) < 0 {
			title += " · Every row"
		} else {
			title += " · If blank"
		}
	}
	if m.metadata.focused == metaPlanText {
		title = "Plan · Every row"
		if len(m.columnsFor(MapPlan)) > 0 {
			title = "Plan · If blank"
		}
	}
	b.WriteString("\n  " + activeFieldStyle.Render(title) + "\n")
	for _, line := range m.metadataHelp(m.metadata.focused) {
		b.WriteString("  " + tipStyle.Render(line) + "\n")
	}

	b.WriteString(helpStyle.Render("\n  ↑/↓ or enter move between rows • ←/→ change option or column • tab continue • esc back"))
	return b.String()
}

// statusHelp and methodHelp describe each option in plain words.
var (
	statusHelp = map[string]string{
		"OPEN":             "Not finished. Use when the audit is still in progress.",
		"OVERDUE":          "Past its due date and not yet submitted.",
		"SUBMITTED":        "The assessor's work is done and ready for review.",
		"ACCEPTED":         "Reviewed and accepted.",
		"ARCHIVED":         "No longer active; kept for the record.",
		"CHANGE_REQUESTED": "A reviewer asked for changes before accepting.",
		"QA_REQUESTED":     "Sent for a quality-assurance check.",
		"REVIEWED":         "A reviewer has gone through it.",
	}
	stepStatusHelp = map[string]string{
		"OPEN":    "Not assessed yet, or still in progress.",
		"PASS":    "The step was assessed and passed.",
		"FAIL":    "The step was assessed and failed.",
		"PARTIAL": "The step partly passed.",
	}
	methodHelp = map[string]string{
		"TEST":      "Exercising the control to see that it works.",
		"EXAMINE":   "Reviewing documents, configurations, or evidence.",
		"INTERVIEW": "Talking with the people responsible for the control.",
		"UNKNOWN":   "The method wasn't recorded.",
	}
)

// metadataHelp returns the explanation shown under the form for a field.
func (m *Model) metadataHelp(field int) []string {
	if src, ok := metaSource[field]; ok {
		name := strings.ToLower(strings.TrimSuffix(metaLabels[src.field], ":"))
		lines := []string{
			fmt.Sprintf("Which CSV column holds each row's %s? ←/→ steps through every column, or none.", name),
			"The column wins: rows with a value here use it, whatever the setting below says.",
			"Pick any column, whatever it's named. If it already feeds another field, it switches here.",
		}
		if src.parse != nil {
			lines = append(lines, "A cell counts only if it is exactly one of the values listed for this field; anything else uses the value below.")
		}
		if found := m.valuesFound(src.target, src.parse); found != "" {
			lines = append(lines, found)
		}
		return lines
	}

	var source string
	for srcRow, s := range metaSource {
		if s.field == field {
			source = m.mappedColumn(metaSource[srcRow].target)
		}
	}
	fallback := "Used for every row, because no column is picked above."
	if source != "" {
		fallback = fmt.Sprintf("Used only for rows where %q is blank or can't be read; every other row uses the column.", source)
	}
	keep := "Leave it empty to keep the person already set on each requirement."

	if field == metaPlanText {
		lines := []string{
			"The Plan says how each requirement was assessed.",
			"Its columns are the ones mapped to plan on step 8; each is added under a bold heading with the column's name.",
			"Text typed here is used for rows where those columns are all blank, or for every row if no column is mapped.",
		}
		if note := m.stackOrderNote(MapPlan); note != "" {
			lines = append(lines, strings.TrimSpace(note))
		}
		return lines
	}

	switch field {
	case metaStatus:
		s := statusOptions[m.metadata.statusIdx]
		return []string{"The requirement's Review Status in Paramify.", s + ": " + statusHelp[s], fallback}
	case metaMethod:
		s := methodOptions[m.metadata.methodIdx]
		return []string{"How the assessor checked the requirement, shown on the assurance step.",
			s + ": " + methodHelp[s], fallback}
	case metaStepStatus:
		s := StepStatusOptions[m.metadata.stepStatusIdx]
		return []string{"The assurance step's status, one of Paramify's four: Open, Pass, Fail or Partial.",
			s + ": " + stepStatusHelp[s], fallback}
	case metaAssigned:
		return []string{"Email of the assessor assigned to the assurance step.", fallback, keep}
	case metaOwner:
		return []string{"The requirement's Assurance Owner in Paramify: an email or a team name (e.g. \"Client Success\").", fallback, keep}
	case metaReviewer:
		return []string{"Email of the person who reviews the requirement.", fallback, keep}
	}
	return nil
}

// isCyclingField returns true for fields that use ←/→ cycling instead of text input.
func isCyclingField(idx int) bool {
	_, isSource := metaSource[idx]
	return isSource || idx == metaStatus || idx == metaMethod || idx == metaStepStatus
}
