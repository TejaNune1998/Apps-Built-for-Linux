package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
)

func (m model) renderLabels(w, h int) string {
	return panel(renderLabelRows(m.labels, m.selectedLabel, w-4), w, h, "Labels", m.activePanel == 0)
}

func (m model) renderTasks(w, h int) string {
	return panel(m.renderTaskRows(w-4), w, h, "Tasks · "+filepath.Base(m.currentParent), m.activePanel == 1)
}

func renderLabelRows(items []item, selected, width int) string {
	if len(items) == 0 {
		return "  No labels"
	}
	var b strings.Builder
	for i, x := range items {
		count := x.childCount
		labelWidth := maxInt(6, width-6)
		line := fmt.Sprintf("  %-*s %2d", labelWidth, x.name, count)
		if i == selected {
			line = ">" + line[1:]
			b.WriteString(selectedStyle.Width(width).Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) renderTaskRows(width int) string {
	if len(m.tasks) == 0 {
		return "  No tasks"
	}
	var b strings.Builder
	for i, x := range m.tasks {
		if x.group {
			arrow := "▸"
			if x.expanded {
				arrow = "▾"
			}
			line := fmt.Sprintf("  %s %s", arrow, x.name)
			if i == m.selectedTask {
				line = "> " + strings.TrimLeft(line, " ")
				b.WriteString(selectedStyle.Width(width).Render(line))
			} else {
				b.WriteString(titleStyle.Render(line))
			}
			b.WriteByte('\n')
			continue
		}
		status := x.status
		mark := "○"
		if x.closed {
			if x.closedPrefix == "DONE" {
				mark = "✓"
			} else {
				mark = "✗"
			}
		}
		switch status {
		case "In Progress":
			mark = "◐"
		case "On Hold":
			mark = "◼"
		}
		childMark := ""
		if x.childCount > 0 {
			childMark = fmt.Sprintf(" ▸%d", x.childCount)
		}
		repeat := ""
		if x.recurrence != "" {
			repeat = " ↻"
		}
		date := x.dueTime
		if t, err := parseTaskTime(date); err == nil {
			date = t.Format("02 Jan")
			if t.Before(time.Now()) && status != "Completed" {
				date += dangerStyle.Render("!")
			}
		} else {
			date = ""
		}
		indent := strings.Repeat("  ", x.depth)
		if x.closed {
			childMark = ""
			repeat = ""
		}
		left := fmt.Sprintf("%s%s %s%s%s", indent, mark, x.name, childMark, repeat)
		available := maxInt(10, width-10)
		if lipgloss.Width(left) > available {
			left = lipgloss.NewStyle().MaxWidth(available-1).Render(left) + "…"
		}
		line := fmt.Sprintf("  %-*s %7s", available, left, date)
		if i == m.selectedTask {
			line = ">" + line[1:]
			b.WriteString(selectedStyle.Width(width).Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (m model) renderDetails(w, h int) string {
	content := addScrollbar(m.detailsView.View(), w-4, h-2, m.detailsView.ScrollPercent())
	return panel(content, w, h, "Task Details", m.activePanel == 2)
}
func (m model) renderCounts(w, h int) string {
	return panel(countStyle.Render(m.counts), w, h, "Task Counts", m.activePanel == 3)
}
func (m model) renderToday(w, h int) string {
	var b strings.Builder
	if len(m.todayItems) == 0 {
		b.WriteString("No starts or dues today")
	}
	for i, x := range m.todayItems {
		mark := "▶"
		if x.kind == "Due" {
			mark = "■"
		}
		line := fmt.Sprintf("%s %s %s", x.when.Format("15:04"), mark, x.title)
		if x.subtask {
			line = "↳ " + line
		}
		if x.when.Before(time.Now()) && x.kind == "Due" {
			line = dangerStyle.Render(line)
		}
		if i == m.selectedToday {
			line = selectedStyle.Render("> " + line + "  " + x.label)
		} else {
			line = line + "  " + x.label
		}
		b.WriteString(line + "\n")
	}
	return panel(b.String(), w, h, "Today · "+time.Now().Format("Mon 02 Jan"), m.activePanel == 4)
}
func (m model) renderWorklogs(w, h int) string {
	content := addScrollbar(m.logs.View(), w-4, h-2, m.logs.ScrollPercent())
	return panel(content, w, h, "Work Logs", m.activePanel == 5)
}

func (m model) View() tea.View {
	if m.width < 90 || m.height < 24 {
		return tea.NewView("Please resize terminal to at least 90 columns and 24 rows.")
	}

	bodyHeight := m.height - 3
	topH := bodyHeight * 2 / 3
	bottomH := bodyHeight - topH
	w1 := m.width * 24 / 112
	w2 := m.width * 38 / 112
	w3 := m.width - w1 - w2

	top := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderLabels(w1, topH),
		m.renderTasks(w2, topH),
		m.renderDetails(w3, topH),
	)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderCounts(w1, bottomH),
		m.renderToday(w2, bottomH),
		m.renderWorklogs(w3, bottomH),
	)

	crumb := "To-Do " + versionString() + "  " + m.breadcrumb() + "  " + time.Now().Format("Mon 02 Jan  15:04")
	header := titleStyle.Render(crumb)
	status := "Ready · " + m.workDir
	if m.message != "" {
		status = m.message
	}
	footer := renderFunctionBar()
	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		top,
		bottom,
		normalStyle.Render(status),
		normalStyle.Render(footer),
	)

	if m.mode != "" {
		var overlay string
		boxWidth := minInt(m.width-8, 62)

		switch m.mode {
		case "menu", "fieldmenu":
			if m.mode == "fieldmenu" {
				overlay = m.renderEditChoiceDialog()
			} else {
				overlay = titleStyle.Render(m.menuTitle) + "\n\n" +
					renderChoices(m.menuItems, m.menuIndex) +
					"\n\n↑/↓ move     Enter select     Esc cancel"
			}
		case "input", "fieldedit":
			overlay = titleStyle.Render(m.inputTitle) + "\n\n" +
				m.inputValue + "▏\n\nEnter save     Esc cancel"
		case "confirm":
			overlay = titleStyle.Render(m.menuTitle) + "\n\n" +
				renderChoices([]string{"Yes", "No"}, m.menuIndex) +
				"\n\nEnter confirm     Esc cancel"
		case "viewlog":
			overlay = titleStyle.Render("Selected Work Log") + "\n\n" + m.message + "\n\nEsc back"
		case "help":
			overlay = m.renderHelpDialog()
			boxWidth = minInt(m.width-12, 90)
		case "datetime":
			overlay = m.renderDatePicker()
			boxWidth = minInt(m.width-12, 84)
		case "upcoming":
			overlay = m.renderUpcomingDialog()
			boxWidth = minInt(m.width-12, 94)
		}

		if boxWidth < 30 {
			boxWidth = 30
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(focusedBorder).
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("7")).
			Padding(1, 2).
			Width(boxWidth).
			Render(overlay)

		// Composite the modal over the existing screen so the panels remain
		// visible behind it, as in the supplied reference screenshots.
		boxW, boxH := lipgloss.Width(box), lipgloss.Height(box)
		x := maxInt(0, (m.width-boxW)/2)
		y := maxInt(0, (m.height-boxH)/2)

		shadowLine := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(strings.Repeat("░", boxW+2))
		shadowRows := make([]string, boxH+1)
		for i := range shadowRows {
			shadowRows[i] = shadowLine
		}
		shadow := strings.Join(shadowRows, "\n")
		dimmedBody := lipgloss.NewStyle().Faint(true).Render(body)

		canvas := lipgloss.NewCanvas(m.width, m.height).
			Compose(lipgloss.NewLayer(dimmedBody).X(0).Y(0).Z(0)).
			Compose(lipgloss.NewLayer(shadow).X(x + 2).Y(y + 1).Z(1)).
			Compose(lipgloss.NewLayer(box).X(x).Y(y).Z(2))

		body = canvas.Render()

	}

	v := tea.NewView(body)
	v.AltScreen = false
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func renderFunctionBar() string {
	keys := []string{
		"F1 Add", "F2 Sub", "F3 Log", "F4 Done", "F5 Cancel",
		"F6 Delete", "F7 Upcoming", "F8 Reload", "F9 Help", "F10 Quit",
	}
	var parts []string
	for _, key := range keys {
		parts = append(parts, lipgloss.NewStyle().
			Background(lipgloss.Color("6")).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Padding(0, 1).
			Render(key))
	}
	return strings.Join(parts, " ")
}

func (m model) renderEditChoiceDialog() string {
	var b strings.Builder
	p := m.selectedPath()
	name := strings.ReplaceAll(filepath.Base(p), "_", " ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(name) + "\n\n")
	for i, choice := range m.menuItems {
		mark := "( )"
		if i == m.menuIndex {
			mark = "(•)"
			b.WriteString(selectedStyle.Width(34).Render("> "+mark+" "+choice) + "\n")
		} else {
			b.WriteString("  " + mark + " " + choice + "\n")
		}
	}
	current := ""
	if p != "" {
		current = readField(p, m.fieldEdit)
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Current value: " + current))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11")).Bold(true).Padding(0, 2).Render("[ OK ]"))
	b.WriteString("       [ Cancel ]")
	return b.String()
}

func (m model) renderHelpDialog() string {
	left := "Move\n" +
		"Tab / Shift+Tab   Next / previous panel\n" +
		"↑ ↓               Move the cursor\n" +
		"← →               Collapse / expand a task\n" +
		"Enter / click     Open / edit a field\n" +
		"PgUp / PgDn       Scroll details / work logs\n" +
		"Esc               Close a dialog / quit"
	right := "Function keys\n" +
		"F1   Add label / task       F6   Delete label\n" +
		"F2   Add sub-task           F7   Upcoming\n" +
		"F3   Add work log           F8   Reload\n" +
		"F4   Mark done              F9   This help\n" +
		"F5   Mark canceled          F10  Quit"
	return titleStyle.Render("Help · F9") + "\n\n" +
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(43).Render(left),
			lipgloss.NewStyle().Width(47).Render(right),
		) +
		"\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Task Counts is display only (no focus).") +
		"\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11")).Bold(true).Padding(0, 2).Render("[ Close ]")
}

func (m model) renderUpcomingDialog() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Upcoming · start / due") + "\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Next 30 days · ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("▶ starts") + "   " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("■ due") + "\n\n")

	lastDay := ""
	for i, x := range m.upcomingItems {
		day := x.when.Format("Mon 02 Jan")
		if day != lastDay {
			b.WriteString(titleStyle.Render(day) + "\n")
			lastDay = day
		}
		mark := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("▶")
		kind := "Start"
		if x.kind == "Due" {
			mark = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("■")
			kind = "Due"
		}
		title := x.title
		if x.subtask {
			title = "↳ " + title
		}
		line := fmt.Sprintf("  %s  %s  %-7s  %-42s  %s",
			x.when.Format("15:04"), mark, kind, title, x.label)
		if i == m.selectedSchedule {
			b.WriteString(selectedStyle.Width(84).Render("> "+strings.TrimLeft(line, " ")) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	if len(m.upcomingItems) == 0 {
		b.WriteString("No scheduled starts or due dates in the next 30 days.\n")
	}
	b.WriteString("\n↑↓ move     Enter show task     Esc close\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11")).Bold(true).Padding(0, 2).Render("[ Show task ]"))
	b.WriteString("       [ Close ]")
	return b.String()
}

func versionString() string { return "1.0.0" }

func (m model) breadcrumb() string {
	label := strings.ReplaceAll(filepath.Base(m.selectedLabelPath()), "_", " ")
	if label == "." || label == "" {
		return ""
	}
	parts := []string{label}
	if p := m.selectedPath(); p != "" {
		rel, err := filepath.Rel(m.selectedLabelPath(), p)
		if err == nil {
			for _, part := range strings.Split(rel, string(os.PathSeparator)) {
				parts = append(parts, strings.ReplaceAll(part, "_", " "))
			}
		}
	}
	return strings.Join(parts, " › ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var detailRenderers = map[int]*glamour.TermRenderer{}

func (m *model) refreshDetails(width int) {
	if width < 20 {
		width = 20
	}
	if m.detailCache == nil || m.detailCacheSource != m.details || m.detailCacheWidth != width {
		m.detailCache = make(map[int]string)
		m.detailCacheSource = m.details
		m.detailCacheWidth = width
	}
	if cached, ok := m.detailCache[m.detailField]; ok {
		m.detailsView.SetContent(cached)
		m.detailsView.SetWidth(width - 5)
		return
	}
	source := m.details
	fieldLines := map[string]string{"title": "# ", "description": "## Description", "start_time": "- **Start Time:**", "due_time": "- **Due Time:**", "priority": "- **Priority:**", "status": "- **Status:**", "recurrence": "- **Repeat:**", "alerts": "## Alerts"}
	if m.detailField >= 0 && m.detailField < len(m.detailFields) {
		needle := fieldLines[m.detailFields[m.detailField]]
		if needle != "" {
			lines := strings.Split(source, "\n")
			for i, line := range lines {
				if strings.Contains(line, needle) {
					lines[i] = "> " + line
					break
				}
			}
			source = strings.Join(lines, "\n")
		}
	}
	renderer := detailRenderers[width]
	if renderer == nil {
		var err error
		renderer, err = glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width-7))
		if err == nil {
			detailRenderers[width] = renderer
		}
	}
	if renderer != nil {
		if rendered, err := renderer.Render(source); err == nil {
			source = rendered
		}
	}
	m.detailCache[m.detailField] = source
	m.detailsView.SetContent(source)
	m.detailsView.SetWidth(width - 5)
}

func (m *model) openDetailEditor() {
	p := m.selectedPath()
	if p == "" || m.tasks[m.selectedTask].closed {
		return
	}
	if m.detailField < 0 || m.detailField >= len(m.detailFields) {
		m.detailField = 0
	}
	field := m.detailFields[m.detailField]
	m.fieldEdit = field
	switch field {
	case "priority":
		m.fieldEdit = field
		m.mode = "fieldmenu"
		m.menuTitle = "Edit: Priority"
		m.menuItems = []string{"Low", "Medium", "High", "Critical"}
		m.menuIndex = choiceIndex(m.menuItems, readField(p, field))
	case "status":
		m.fieldEdit = field
		m.mode = "fieldmenu"
		m.menuTitle = "Edit: Status"
		m.menuItems = []string{"Todo", "In Progress", "On Hold"}
		m.menuIndex = choiceIndex(m.menuItems, readField(p, field))
	case "recurrence":
		m.fieldEdit = field
		m.mode = "fieldmenu"
		m.menuTitle = "Edit: Repeat"
		m.menuItems = []string{"None", "Every day", "Every week", "Every 2 weeks", "Every month", "Every year", "Custom"}
		repeat := readField(p, field)
		m.menuIndex = choiceIndex(m.menuItems, displayRecurrence(repeat))
		if repeat != "" && m.menuIndex == 0 {
			m.menuIndex = len(m.menuItems) - 1
		}
	case "start_time", "due_time":
		m.startDatePicker("detail")
	case "alerts":
		m.openMenu("Configure Alerts", "At Start Time", "5 minutes before start", "10 minutes before start", "15 minutes before start", "30 minutes before start", "1 hour before start", "1 day before start", "At Due Time", "1 day before due", "2 days before due", "3 days before due", "1 week before due", "Custom", "Remove Alert", "Clear All Alerts", "Back")
	default:
		m.mode = "fieldedit"
		m.fieldEdit = field
		m.inputTitle = "Edit: " + fieldTitle(field)
		m.inputValue = readField(p, field)
	}
}

func choiceIndex(items []string, value string) int {
	for i, item := range items {
		if item == value {
			return i
		}
	}
	return 0
}

func (m *model) saveDetailField(value string) {
	p := m.selectedPath()
	if p == "" {
		m.mode = ""
		return
	}
	field := m.fieldEdit
	if field == "recurrence" {
		switch value {
		case "None":
			value = ""
		case "Every day":
			value = "1 day"
		case "Every week":
			value = "1 week"
		case "Every 2 weeks":
			value = "2 weeks"
		case "Every month":
			value = "1 month"
		case "Every year":
			value = "1 year"
		}
	}
	if field == "start_time" || field == "due_time" {
		if _, err := parseTaskTime(strings.TrimSpace(value)); err != nil {
			m.message = "Invalid date/time. Use YYYY-MM-DD HH:MM."
			return
		}
	}
	if field == "recurrence" && value != "" {
		if _, _, ok := parseStep(value); !ok {
			m.message = "Invalid repeat interval. Example: 3 days or 2 weeks."
			return
		}
	}
	fields := map[string]string{field: value}
	if field != "title" {
		fields["title"] = readField(p, "title")
	}
	if err := saveEntityFields(p, fields); err != nil {
		m.message = "Error: " + err.Error()
	} else {
		m.message = fieldTitle(field) + " updated."
	}
	m.mode = ""
	m.refresh()
}

func renderChoices(items []string, selected int) string {
	var b strings.Builder
	for i, s := range items {
		if i == selected {
			b.WriteString(selectedStyle.Render("> (•) " + s))
		} else {
			b.WriteString("  ( ) " + s)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func (m *model) beginForm(title, target string, fields []string, values map[string]string) {
	m.mode = "input"
	m.menuTitle = title
	m.formTarget = target
	m.formFields = fields
	m.inputStep = 0
	m.form = map[string]string{}
	for k, v := range values {
		m.form[k] = v
	}
	m.inputField = fields[0]
	m.inputTitle = fieldTitle(fields[0])
	m.inputValue = m.form[fields[0]]
	if m.inputField == "start_time" || m.inputField == "due_time" || m.inputField == "alert_custom" {
		m.startDatePicker("form")
	}
}
func fieldTitle(f string) string {
	switch f {
	case "title":
		return "Title"
	case "description":
		return "Description"
	case "start_time":
		return "Start Date & Time"
	case "due_time":
		return "Due Date & Time"
	case "priority":
		return "Priority: Low / Medium / High / Critical"
	case "status":
		return "Status: Todo / In Progress / On Hold"
	case "recurrence":
		return "Repeat: blank, 1 day, 1 week, 2 weeks, 1 month, 1 year"
	case "log":
		return "Work log description"
	case "alert_custom":
		return "Custom alert date/time (YYYY-MM-DD HH:MM)"
	case "label":
		return "Label name"
	}
	return f
}
func (m *model) nextFormField() {
	m.form[m.inputField] = m.inputValue
	m.inputStep++
	if m.inputStep >= len(m.formFields) {
		m.finishForm()
		return
	}
	m.inputField = m.formFields[m.inputStep]
	m.inputTitle = fieldTitle(m.inputField)
	m.inputValue = m.form[m.inputField]
	if m.inputField == "start_time" || m.inputField == "due_time" || m.inputField == "alert_custom" {
		m.startDatePicker("form")
	}
}
func (m *model) finishForm() {
	m.mode = ""
	var err error
	switch m.formTarget {
	case "label":
		name := strings.TrimSpace(m.form["label"])
		if !cleanName(name) {
			err = fmt.Errorf("invalid label name")
		} else {
			p := filepath.Join(m.workDir, folderName(name))
			if _, e := os.Stat(p); e == nil {
				err = fmt.Errorf("label already exists")
			} else {
				err = os.MkdirAll(p, 0755)
			}
		}
	case "create":
		err = createEntity(m.form["parent"], m.form["title"], m.form["description"], m.form["start_time"], m.form["due_time"], m.form["priority"], m.form["status"], m.form["recurrence"], nil)
	case "edit":
		err = saveEntityFields(m.form["path"], m.form)
	case "log":
		err = addWorkLog(m.form["path"], m.form["log"])
	case "editlog":
		err = editWorkLog(m.form["file"], m.form["log"])
		if err == nil {
			err = buildDetails(m.form["path"])
		}
	case "customalert":
		line, e := makeAlert(m.form["path"], "Custom", "", m.form["alert_custom"])
		err = e
		if e == nil {
			err = addAlerts(m.form["path"], []string{line})
		}
	}
	if err != nil {
		m.message = "Error: " + err.Error()
	} else {
		m.message = "Saved successfully."
	}
	m.refresh()
}
func (m *model) openActions() {
	switch m.activePanel {
	case 0:
		m.openMenu("Label Actions", "Create Label", "Delete Selected Label", "Back")
	case 1:
		if m.currentParent == m.selectedLabelPath() {
			m.openMenu("Task Actions", "Create Task", "Add Work Log", "Work Logs", "Alerts", "Add Sub-task", "Complete", "Cancel", "Delete", "Back")
		} else {
			m.openMenu("Sub-task Actions", "Add Work Log", "Work Logs", "Alerts", "Complete", "Cancel", "Delete", "Back")
		}
	case 5:
		if len(m.worklogs) > 0 {
			m.openMenu("Work Log Actions", "Edit Selected Work Log", "Delete Selected Work Log", "Back")
		}
	}
}
func (m *model) handleMenu() {
	choice := m.menuItems[m.menuIndex]
	if strings.HasPrefix(m.menuTitle, "Confirm ") || strings.HasPrefix(m.menuTitle, "Permanently delete") {
		m.confirmAction(choice == "Yes")
		return
	}
	m.mode = ""
	p := m.selectedPath()
	switch m.menuTitle {
	case "Label Actions":
		switch choice {
		case "Create Label":
			m.beginForm("Create Label", "label", []string{"label"}, nil)
		case "Delete Selected Label":
			if lp := m.selectedLabelPath(); lp != "" {
				m.menuTitle = "Delete label and cancel all open tasks?"
				m.mode = "confirm"
				m.returnMode = "delete-label"
			}
		}
	case "Task Actions":
		switch choice {
		case "Create Task":
			m.beginForm("Create Task", "create", []string{"title", "description", "start_time", "due_time", "priority", "status", "recurrence"}, map[string]string{"parent": m.currentParent, "start_time": time.Now().Format("2006-01-02 15:04"), "due_time": time.Now().Format("2006-01-02 15:04"), "priority": "Medium", "status": "Todo"})
		case "Add Sub-task":
			m.beginForm("Create Sub-task", "create", []string{"title", "description", "start_time", "due_time", "priority", "status", "recurrence"}, map[string]string{"parent": p, "start_time": time.Now().Format("2006-01-02 15:04"), "due_time": time.Now().Format("2006-01-02 15:04"), "priority": "Medium", "status": "Todo"})
		case "Add Work Log":
			m.beginForm("Add Work Log", "log", []string{"log"}, map[string]string{"path": p})
		case "Work Logs":
			m.openMenu("Select Work Log", m.logChoices()...)
		case "Alerts":
			m.openMenu("Configure Alerts", "At Start Time", "5 minutes before start", "10 minutes before start", "15 minutes before start", "30 minutes before start", "1 hour before start", "1 day before start", "At Due Time", "1 day before due", "2 days before due", "3 days before due", "1 week before due", "Custom", "Remove Alert", "Clear All Alerts", "Back")
		case "Complete", "Cancel":
			status, prefix := "Canceled", "CANCEL"
			if choice == "Complete" {
				status, prefix = "Completed", "DONE"
			}
			m.message = "Confirm through action menu again to apply: " + status
			m.openMenu("Confirm "+status+"?", "Yes", "No")
			m.mode = "confirm"
			m.returnMode = "close:" + prefix + ":" + status
		case "Delete":
			m.openMenu("Permanently delete this task and its subtasks?", "Yes", "No")
			m.mode = "confirm"
			m.returnMode = "delete-task"
		case "Back":
		}
	case "Sub-task Actions":
		switch choice {
		case "Add Work Log":
			m.beginForm("Add Work Log", "log", []string{"log"}, map[string]string{"path": p})
		case "Work Logs":
			m.openMenu("Select Work Log", m.logChoices()...)
		case "Alerts":
			m.openMenu("Configure Alerts", "At Start Time", "5 minutes before start", "10 minutes before start", "15 minutes before start", "30 minutes before start", "1 hour before start", "1 day before start", "At Due Time", "1 day before due", "2 days before due", "3 days before due", "1 week before due", "Custom", "Remove Alert", "Clear All Alerts", "Back")
		case "Complete", "Cancel":
			status, prefix := "Canceled", "CANCEL"
			if choice == "Complete" {
				status, prefix = "Completed", "DONE"
			}
			m.openMenu("Confirm "+status+"?", "Yes", "No")
			m.mode = "confirm"
			m.returnMode = "close:" + prefix + ":" + status
		case "Delete":
			m.openMenu("Permanently delete this sub-task?", "Yes", "No")
			m.mode = "confirm"
			m.returnMode = "delete-task"
		}
	case "Configure Alerts":
		if choice == "Custom" {
			m.beginForm("Custom Alert", "customalert", []string{"alert_custom"}, map[string]string{"path": p, "alert_custom": time.Now().Format("2006-01-02 15:04")})
		} else if choice == "Remove Alert" {
			choices := readAlerts(p)
			if len(choices) == 0 {
				m.message = "No alerts configured."
			} else {
				m.openMenu("Remove Alert", choices...)
			}
		} else if choice == "Clear All Alerts" {
			m.openMenu("Clear all alerts?", "Yes", "No")
			m.returnMode = "clear-alerts"
		} else if choice != "Back" {
			kind, offset := choice, ""
			if strings.Contains(choice, " before start") {
				kind, offset = "Before Start Time", strings.TrimSuffix(choice, " before start")
			} else if strings.Contains(choice, " before due") {
				kind, offset = "Before Due Time", strings.TrimSuffix(choice, " before due")
			}
			line, err := makeAlert(p, kind, offset, "")
			if err == nil {
				err = addAlerts(p, []string{line})
			}
			if err != nil {
				m.message = err.Error()
			} else {
				m.message = "Alert added."
			}
			m.refresh()
		}
	case "Remove Alert":
		if choice != "Back" {
			if err := removeAlert(p, choice); err != nil {
				m.message = err.Error()
			} else {
				m.message = "Alert removed."
			}
			m.refresh()
		}
	case "Clear all alerts?":
		if choice == "Yes" {
			_ = writeAlerts(p, nil)
			m.message = "Alerts cleared."
			m.refresh()
		}
	case "Select Work Log":
		if choice != "Back" {
			m.selectedLog = m.menuIndex
			if m.selectedLog < len(m.worklogs) {
				m.mode = "viewlog"
				b, _ := os.ReadFile(m.worklogs[m.selectedLog])
				m.message = string(b)
			}
		}
	case "Work Log Actions":
		if choice == "Edit Selected Work Log" && m.selectedLog < len(m.worklogs) {
			f := m.worklogs[m.selectedLog]
			m.beginForm("Edit Work Log", "editlog", []string{"log"}, map[string]string{"path": p, "file": f, "log": workLogDescription(f)})
		} else if choice == "Delete Selected Work Log" && m.selectedLog < len(m.worklogs) {
			_ = os.Remove(m.worklogs[m.selectedLog])
			m.refresh()
		}
	}
}
func (m *model) logChoices() []string {
	var out []string
	for _, f := range m.worklogs {
		out = append(out, filepath.Base(f))
	}
	if len(out) == 0 {
		return []string{"No work logs", "Back"}
	}
	out = append(out, "Back")
	return out
}
func (m *model) confirmAction(yes bool) {
	mode := m.returnMode
	m.returnMode = ""
	m.mode = ""
	if !yes {
		return
	}
	switch {
	case mode == "delete-label":
		if p := m.selectedLabelPath(); p != "" {
			if e := deleteLabel(p); e != nil {
				m.message = e.Error()
			}
		}
	case mode == "clear-alerts":
		if p := m.selectedPath(); p != "" {
			_ = writeAlerts(p, nil)
			m.message = "Alerts cleared."
		}
	case mode == "delete-task":
		if p := m.selectedPath(); p != "" {
			if e := os.RemoveAll(p); e != nil {
				m.message = e.Error()
			} else {
				m.message = "Task deleted permanently."
			}
		}
	case strings.HasPrefix(mode, "close:"):
		p := m.selectedPath()
		parts := strings.Split(mode, ":")
		if p != "" && len(parts) == 3 {
			if e := closeEntity(p, parts[1], parts[2], "No", ""); e != nil {
				m.message = e.Error()
			} else {
				m.message = parts[2] + "."
			}
		}
	}
	m.refresh()
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w1, w2 := m.width*24/112, m.width*38/112
		w3 := m.width - w1 - w2
		topH := (m.height - 3) * 2 / 3
		bottomH := m.height - 3 - topH
		m.logs.SetWidth(w3 - 5)
		m.logs.SetHeight(bottomH - 2)
		m.refreshDetails(w3)
		m.detailsView.SetHeight(topH - 2)
	case tea.PasteMsg:
		if m.mode == "fieldedit" || m.mode == "input" {
			m.inputValue += msg.Content
		}
	case tea.MouseWheelMsg:
		if m.activePanel == 2 {
			var c tea.Cmd
			m.detailsView, c = m.detailsView.Update(msg)
			return m, c
		}
		if m.activePanel == 5 {
			var c tea.Cmd
			m.logs, c = m.logs.Update(msg)
			return m, c
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && m.mode == "datetime" {
			// The date dialog is centred and uses a two-column calendar/clock layout.
			boxWidth, boxHeight := minInt(m.width-12, 84), 30
			boxLeft := (m.width - boxWidth) / 2
			boxTop := (m.height - boxHeight) / 2
			calendarLeft := boxLeft + 4
			calendarRow := msg.Y - (boxTop + 8)
			calendarCol := (msg.X - calendarLeft) / 3

			if calendarRow >= 0 && calendarRow < 6 && calendarCol >= 0 && calendarCol < 7 {
				first := time.Date(m.dateValue.Year(), m.dateValue.Month(), 1, 0, 0, 0, 0, time.Local)
				start := first.AddDate(0, 0, -((int(first.Weekday()) + 6) % 7))
				d := start.AddDate(0, 0, calendarRow*7+calendarCol)
				if d.Month() == m.dateValue.Month() {
					m.dateValue = d
				}
				m.datePart = 0
			} else if msg.Y == boxTop+4 {
				if msg.X < boxLeft+22 {
					m.dateValue = m.dateValue.AddDate(0, -1, 0)
				} else if msg.X < boxLeft+40 {
					m.dateValue = m.dateValue.AddDate(0, 1, 0)
				}
			} else if msg.Y >= boxTop+8 && msg.Y <= boxTop+12 && msg.X >= boxLeft+46 {
				clockX := msg.X - (boxLeft + 46)
				part := 1
				if clockX >= 16 && clockX < 29 {
					part = 2
				} else if clockX >= 29 {
					part = 3
				}
				m.datePart = part
				if msg.Y <= boxTop+9 {
					m.adjustDateTime(1)
				} else {
					m.adjustDateTime(-1)
				}
			}
			return m, nil
		}
		if msg.Button == tea.MouseLeft && m.mode == "upcoming" {
			row := msg.Y - m.height/2 - 3
			if row >= 0 && row < len(m.upcomingItems) {
				m.selectedSchedule = row
				m.showUpcomingTask()
			}
			return m, nil
		}
		if msg.Button == tea.MouseLeft && m.mode == "" {
			w1, w2 := m.width*24/112, m.width*38/112
			topH := (m.height - 3) * 2 / 3
			if msg.X >= w1+w2 && msg.Y >= topH+1 {
				m.activePanel = 5
				row := msg.Y - topH - 2
				if row >= 0 && len(m.worklogs) > 0 {
					idx := row / 4
					if idx >= len(m.worklogs) {
						idx = len(m.worklogs) - 1
					}
					m.selectedLog = idx
					m.openMenu("Work Log Actions", "Edit Selected Work Log", "Delete Selected Work Log", "Back")
				}
			} else if msg.X >= w1+w2 && msg.Y >= 1 && msg.Y < topH+1 && m.selectedPath() != "" {
				m.activePanel = 2
				row := msg.Y - 2
				if row < 0 {
					row = 0
				}
				m.detailField = row * len(m.detailFields) / maxInt(1, topH-4)
				if m.detailField >= len(m.detailFields) {
					m.detailField = len(m.detailFields) - 1
				}
				m.openDetailEditor()
			} else if msg.X < w1 {
				m.activePanel = 0
				row := msg.Y - 2
				if row >= 0 && row < len(m.labels) {
					m.selectedLabel = row
					m.selectedTask = 0
					m.loadTasks()
				}
			} else if msg.X < w1+w2 && msg.Y < topH+1 {
				m.activePanel = 1
				row := msg.Y - 2
				if row >= 0 && row < len(m.tasks) {
					m.selectedTask = row
					m.loadSelectedTask()
				}
			} else if msg.X >= w1 && msg.X < w1+w2 && msg.Y >= topH+1 {
				m.activePanel = 4
				row := msg.Y - topH - 2
				if row >= 0 && row < len(m.todayItems) {
					m.selectedToday = row
				}
			}
		}
	case tea.KeyPressMsg:
		key := msg.String()
		switch msg.Code {
		case tea.KeyF1:
			key = "f1"
		case tea.KeyF2:
			key = "f2"
		case tea.KeyF3:
			key = "f3"
		case tea.KeyF4:
			key = "f4"
		case tea.KeyF5:
			key = "f5"
		case tea.KeyF6:
			key = "f6"
		case tea.KeyF7:
			key = "f7"
		case tea.KeyF8:
			key = "f8"
		case tea.KeyF9:
			key = "f9"
		case tea.KeyF10:
			key = "f10"
		}
		if m.mode != "" {
			switch m.mode {
			case "help":
				if key == "esc" || key == "enter" {
					m.mode = ""
				}
			case "upcoming":
				switch key {
				case "esc":
					m.mode = ""
				case "up":
					if m.selectedSchedule > 0 {
						m.selectedSchedule--
					}
				case "down":
					if m.selectedSchedule < len(m.upcomingItems)-1 {
						m.selectedSchedule++
					}
				case "enter":
					m.showUpcomingTask()
				}
			case "datetime":
				switch key {
				case "esc":
					m.mode = ""
				case "tab":
					m.datePart = (m.datePart + 1) % 4
				case "shift+tab":
					m.datePart = (m.datePart + 3) % 4
				case "pgup":
					m.dateValue = m.dateValue.AddDate(0, -1, 0)
				case "pgdown":
					m.dateValue = m.dateValue.AddDate(0, 1, 0)
				case "left":
					if m.datePart == 0 {
						m.dateValue = m.dateValue.AddDate(0, 0, -1)
					} else {
						m.datePart = (m.datePart + 3) % 4
					}
				case "right":
					if m.datePart == 0 {
						m.dateValue = m.dateValue.AddDate(0, 0, 1)
					} else {
						m.datePart = (m.datePart + 1) % 4
					}
				case "up":
					m.adjustDateTime(1)
				case "down":
					m.adjustDateTime(-1)
				case "enter":
					m.commitDatePicker()
				}
			case "fieldmenu", "menu":
				switch key {
				case "esc":
					m.mode = ""
				case "up":
					if m.menuIndex > 0 {
						m.menuIndex--
					}
				case "down":
					if m.menuIndex < len(m.menuItems)-1 {
						m.menuIndex++
					}
				case "enter":
					if m.mode == "menu" {
						m.handleMenu()
					} else {
						choice := m.menuItems[m.menuIndex]
						if m.fieldEdit == "recurrence" && choice == "Custom" {
							m.mode = "fieldedit"
							m.inputTitle = "Custom repeat (e.g. 3 days, 2 weeks)"
							m.inputValue = readField(m.selectedPath(), "recurrence")
							if m.inputValue == "" {
								m.inputValue = "3 days"
							}
						} else {
							m.saveDetailField(choice)
						}
					}
				}
			case "confirm":
				switch key {
				case "esc":
					m.mode = ""
				case "up", "down":
					m.menuIndex = 1 - m.menuIndex
				case "enter":
					m.confirmAction(m.menuIndex == 0)
				}
			case "fieldedit":
				switch key {
				case "esc":
					m.mode = ""
				case "enter":
					m.saveDetailField(m.inputValue)
				case "shift+enter":
					m.inputValue += "\n"
				case "backspace":
					r := []rune(m.inputValue)
					if len(r) > 0 {
						m.inputValue = string(r[:len(r)-1])
					}
				case "space":
					m.inputValue += " "
				default:
					if len([]rune(key)) == 1 {
						m.inputValue += key
					}
				}
			case "input":
				switch key {
				case "esc":
					m.mode = ""
				case "enter":
					m.nextFormField()
				case "backspace":
					r := []rune(m.inputValue)
					if len(r) > 0 {
						m.inputValue = string(r[:len(r)-1])
					}
				case "space":
					m.inputValue += " "
				default:
					if len([]rune(key)) == 1 {
						m.inputValue += key
					}
				}
			case "viewlog":
				if key == "esc" {
					m.mode = ""
				}
			}
			return m, nil
		}
		switch key {
		case "f1":
			if m.activePanel == 0 {
				m.beginForm("Create Label", "label", []string{"label"}, nil)
			} else if m.activePanel == 1 && m.selectedLabelPath() != "" {
				m.beginForm("Create Task", "create", []string{"title", "description", "start_time", "due_time", "priority", "status", "recurrence"}, map[string]string{"parent": m.selectedLabelPath(), "start_time": time.Now().Format("2006-01-02 15:04"), "due_time": time.Now().Format("2006-01-02 15:04"), "priority": "Medium", "status": "Todo"})
			}
		case "f2":
			if p := m.selectedPath(); p != "" && !m.tasks[m.selectedTask].closed {
				m.beginForm("Create Sub-task", "create", []string{"title", "description", "start_time", "due_time", "priority", "status", "recurrence"}, map[string]string{"parent": p, "start_time": time.Now().Format("2006-01-02 15:04"), "due_time": time.Now().Format("2006-01-02 15:04"), "priority": "Medium", "status": "Todo"})
			}
		case "f3":
			if p := m.selectedPath(); p != "" && !m.tasks[m.selectedTask].closed {
				m.beginForm("Add Work Log", "log", []string{"log"}, map[string]string{"path": p})
			}
		case "f4", "f5":
			if p := m.selectedPath(); p != "" && !m.tasks[m.selectedTask].closed {
				status, prefix := "Completed", "DONE"
				if key == "f5" {
					status, prefix = "Canceled", "CANCEL"
				}
				m.openMenu("Confirm "+status+"?", "Yes", "No")
				m.mode = "confirm"
				m.returnMode = "close:" + prefix + ":" + status
			}
		case "f6":
			if m.selectedLabelPath() != "" {
				m.openMenu("Delete label and cancel all open tasks?", "Yes", "No")
				m.mode = "confirm"
				m.returnMode = "delete-label"
			}
		case "f7":
			m.loadToday()
			m.selectedSchedule = 0
			if m.activePanel == 4 && len(m.todayItems) > 0 {
				cur := m.todayItems[m.selectedToday]
				for i, x := range m.upcomingItems {
					if x.path == cur.path && x.kind == cur.kind {
						m.selectedSchedule = i
						break
					}
				}
			}
			m.mode = "upcoming"
		case "f8":
			m.refresh()
		case "f9":
			m.mode = "help"
		case "f10", "esc", "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab":
			order := []int{0, 1, 2, 4, 5}
			idx := 0
			for i, v := range order {
				if v == m.activePanel {
					idx = i
				}
			}
			if key == "tab" {
				m.activePanel = order[(idx+1)%len(order)]
			} else {
				m.activePanel = order[(idx+len(order)-1)%len(order)]
			}
		case "up", "down":
			delta := -1
			if key == "down" {
				delta = 1
			}
			switch m.activePanel {
			case 0:
				if n := m.selectedLabel + delta; n >= 0 && n < len(m.labels) {
					m.selectedLabel = n
					m.selectedTask = 0
					m.loadTasks()
				}
			case 1:
				if n := m.selectedTask + delta; n >= 0 && n < len(m.tasks) {
					m.selectedTask = n
					m.loadSelectedTask()
				}
			case 2:
				if n := m.detailField + delta; n >= 0 && n < len(m.detailFields) {
					m.detailField = n
					m.refreshDetails(m.width * 50 / 112)
				}
			case 4:
				if n := m.selectedToday + delta; n >= 0 && n < len(m.todayItems) {
					m.selectedToday = n
				}
			case 5:
				var c tea.Cmd
				m.logs, c = m.logs.Update(msg)
				return m, c
			}
		case "left":
			if m.activePanel == 1 && m.selectedPath() != "" {
				m.expanded[m.selectedPath()] = false
				m.loadTasks()
			}
		case "right":
			if m.activePanel == 1 && m.selectedPath() != "" && !m.tasks[m.selectedTask].closed {
				m.expanded[m.selectedPath()] = true
				m.loadTasks()
			}
		case "enter":
			if m.activePanel == 0 && len(m.labels) > 0 {
				m.selectedTask = 0
				m.loadTasks()
				m.activePanel = 1
			} else if m.activePanel == 1 && len(m.tasks) > 0 {
				x := m.tasks[m.selectedTask]
				if x.group {
					m.closedExpanded = !m.closedExpanded
					m.loadTasks()
				} else if x.childCount > 0 && !x.closed {
					m.expanded[x.path] = !m.expanded[x.path]
					m.loadTasks()
				}
			} else if m.activePanel == 2 && m.selectedPath() != "" {
				m.openDetailEditor()
			} else if m.activePanel == 4 {
				m.loadToday()
				m.selectedSchedule = 0
				if len(m.todayItems) > 0 {
					cur := m.todayItems[m.selectedToday]
					for i, x := range m.upcomingItems {
						if x.path == cur.path && x.kind == cur.kind {
							m.selectedSchedule = i
							break
						}
					}
				}
				m.mode = "upcoming"
			}
		case "pgdown", "pgup":
			if m.activePanel == 5 {
				var c tea.Cmd
				m.logs, c = m.logs.Update(msg)
				return m, c
			} else if m.activePanel == 2 {
				var c tea.Cmd
				m.detailsView, c = m.detailsView.Update(msg)
				return m, c
			}
		}
	}
	return m, nil
}

func (m *model) startDatePicker(returnMode string) {
	value := m.inputValue
	if m.fieldEdit != "" && returnMode == "detail" {
		value = readField(m.selectedPath(), m.fieldEdit)
	}
	t, err := parseTaskTime(value)
	if err != nil {
		t = time.Now()
	}
	m.dateValue = t
	h := t.Hour()
	m.datePart = 0
	if h >= 12 {
		m.dateHour = h - 12
		if m.dateHour == 0 {
			m.dateHour = 12
		}
		m.datePart = 0
	} else {
		m.dateHour = h
		if m.dateHour == 0 {
			m.dateHour = 12
		}
	}
	m.dateMinute = (t.Minute() / 15) * 15
	if m.dateMinute > 45 {
		m.dateMinute = 45
	}
	m.dateAMPM = 0
	if h >= 12 {
		m.dateAMPM = 1
	}
	m.dateReturnMode = returnMode
	m.menuTitle = "Date & Time"
	if returnMode == "detail" {
		m.menuTitle = "Edit: " + fieldTitle(m.fieldEdit)
	} else {
		m.menuTitle = fieldTitle(m.inputField)
	}
	m.mode = "datetime"
}

func (m *model) adjustDateTime(delta int) {
	switch m.datePart {
	case 0:
		m.dateValue = m.dateValue.AddDate(0, 0, delta)
	case 1:
		m.dateHour += delta
		if m.dateHour < 1 {
			m.dateHour = 12
		}
		if m.dateHour > 12 {
			m.dateHour = 1
		}
	case 2:
		m.dateMinute += delta * 15
		if m.dateMinute < 0 {
			m.dateMinute = 45
		}
		if m.dateMinute > 45 {
			m.dateMinute = 0
		}
	case 3:
		m.dateAMPM = 1 - m.dateAMPM
	}
}

func (m *model) commitDatePicker() {
	h := m.dateHour % 12
	if m.dateAMPM == 1 {
		h += 12
	}
	value := time.Date(m.dateValue.Year(), m.dateValue.Month(), m.dateValue.Day(), h, m.dateMinute, 0, 0, time.Local).Format("2006-01-02 15:04")
	ret := m.dateReturnMode
	m.mode = ""
	if ret == "detail" {
		m.saveDetailField(value)
		return
	}
	m.form[m.inputField] = value
	if m.formTarget == "customalert" && m.inputField == "alert_custom" {
		m.finishForm()
		return
	}
	m.nextFormField()
}

func (m model) renderDatePicker() string {
	var calendar strings.Builder
	var clock strings.Builder

	calendar.WriteString(titleStyle.Render("◀   "+m.dateValue.Format("January 2006")+"   ▶") + "\n\n")
	calendar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Mo Tu We Th Fr Sa Su") + "\n")

	first := time.Date(m.dateValue.Year(), m.dateValue.Month(), 1, 0, 0, 0, 0, time.Local)
	start := first.AddDate(0, 0, -((int(first.Weekday()) + 6) % 7))
	today := time.Now()
	for week := 0; week < 6; week++ {
		for day := 0; day < 7; day++ {
			d := start.AddDate(0, 0, week*7+day)
			cell := fmt.Sprintf("%2d", d.Day())
			style := lipgloss.NewStyle()
			if d.Month() != m.dateValue.Month() {
				style = style.Foreground(lipgloss.Color("8"))
			}
			if d.Year() == today.Year() && d.YearDay() == today.YearDay() {
				style = style.Foreground(lipgloss.Color("11")).Underline(true)
			}
			if d.Year() == m.dateValue.Year() && d.YearDay() == m.dateValue.YearDay() {
				style = selectedStyle
			}
			calendar.WriteString(style.Render(cell) + " ")
		}
		calendar.WriteByte('\n')
	}
	calendar.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Today is underlined"))

	ampm := "AM"
	if m.dateAMPM == 1 {
		ampm = "PM"
	}
	clock.WriteString(lipgloss.NewStyle().Bold(true).Render("Time") + "\n\n")
	clock.WriteString("      ▲          ▲          ▲\n")
	hour := fmt.Sprintf("%02d", m.dateHour)
	minute := fmt.Sprintf("%02d", m.dateMinute)
	parts := []string{hour, minute, ampm}
	for i, part := range parts {
		style := lipgloss.NewStyle().Bold(true).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("7")).Padding(0, 2)
		if m.datePart == i+1 {
			style = style.BorderForeground(focusedBorder)
		}
		clock.WriteString(style.Render(part))
		if i < 2 {
			clock.WriteString("  :  ")
		}
	}
	clock.WriteString("\n      ▼          ▼          ▼\n\n")
	clock.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Minutes: 00 · 15 · 30 · 45"))

	ampmLabel := ampm
	selected := fmt.Sprintf("Selected  %s · %02d:%02d %s", m.dateValue.Format("Mon 02 Jan 2006"), m.dateHour, m.dateMinute, ampmLabel)
	help := "Tab calendar / hour / min / AM-PM    ←→ day    PgUp/PgDn month    ↑↓ change    Enter OK    Esc cancel"

	return titleStyle.Render(m.menuTitle) + "\n\n" +
		lipgloss.NewStyle().Bold(true).Render(strings.ReplaceAll(filepath.Base(m.selectedPath()), "_", " ")) + "\n\n" +
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(38).Render(calendar.String()),
			lipgloss.NewStyle().PaddingLeft(4).Width(38).Render(clock.String()),
		) +
		"\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(selected) +
		"\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(help)
}

func (m *model) showUpcomingTask() {
	if len(m.upcomingItems) == 0 || m.selectedSchedule < 0 || m.selectedSchedule >= len(m.upcomingItems) {
		m.mode = ""
		return
	}
	x := m.upcomingItems[m.selectedSchedule]
	for i, l := range m.labels {
		if l.path == filepath.Join(m.workDir, strings.Split(strings.TrimPrefix(x.path, m.workDir+string(os.PathSeparator)), string(os.PathSeparator))[0]) {
			m.selectedLabel = i
			break
		}
	}
	m.expanded = map[string]bool{}
	for parent := filepath.Dir(x.path); parent != m.selectedLabelPath() && parent != m.workDir && parent != "."; parent = filepath.Dir(parent) {
		m.expanded[parent] = true
	}
	m.loadTasks()
	for i, t := range m.tasks {
		if t.path == x.path {
			m.selectedTask = i
			break
		}
	}
	m.loadSelectedTask()
	m.activePanel = 2
	m.mode = ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: to-do-tui <task-directory>")
		os.Exit(1)
	}
	dir := os.Args[1]
	if _, e := os.Stat(dir); e != nil {
		fmt.Println("Task directory not found:", dir)
		os.Exit(1)
	}
	p := tea.NewProgram(initialModel(dir))
	if _, e := p.Run(); e != nil {
		fmt.Println("Error:", e)
		os.Exit(1)
	}
}
