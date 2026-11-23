package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m model) renderLabels(w, h int) string {
	return panel(renderList(m.labels, m.selectedLabel), w, h, "Labels", m.activePanel == 0)
}
func (m model) renderTasks(w, h int) string {
	return panel(renderList(m.tasks, m.selectedTask), w, h, "Tasks", m.activePanel == 1)
}
func (m model) renderDetails(w, h int) string {
	return panel(strings.TrimSpace(m.details), w, h, "Task Details", m.activePanel == 2)
}
func (m model) renderCounts(w, h int) string {
	return panel(m.taskCounts(), w, h, "Task Counts", m.activePanel == 3)
}
func (m model) renderHelp(w, h int) string {
	return panel("Tab       Switch panel\n↑ / ↓     Navigate\nEnter     Select / Open\nBackspace Go to parent\na         Actions\ne         Edit selected\nw         Add work log\nr         Refresh\nPgUp/PgDn Scroll logs\nq         Quit", w, h, "Help", m.activePanel == 4)
}
func (m model) renderWorklogs(w, h int) string {
	return panel(m.logs.View(), w, h, "Work Logs", m.activePanel == 5)
}
func (m model) View() tea.View {
	if m.width < 90 || m.height < 24 {
		return tea.NewView("Please resize terminal to at least 90 columns and 24 rows.")
	}
	topH := m.height * 2 / 3
	bottomH := m.height - topH
	w1 := m.width / 3
	w2 := m.width / 3
	w3 := m.width - w1 - w2
	top := lipgloss.JoinHorizontal(lipgloss.Top, m.renderLabels(w1, topH), m.renderTasks(w2, topH), m.renderDetails(w3, topH))
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, m.renderCounts(w1, bottomH), m.renderHelp(w2, bottomH), m.renderWorklogs(w3, bottomH))
	body := lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	if m.mode != "" {
		var overlay string
		switch m.mode {
		case "menu":
			overlay = titleStyle.Render(m.menuTitle) + "\n" + renderChoices(m.menuItems, m.menuIndex) + "\n↑/↓ select • Enter confirm • Esc cancel"
		case "input":
			overlay = titleStyle.Render(m.inputTitle) + "\n" + m.inputValue + "▏\nEnter save • Esc cancel"
		case "confirm":
			overlay = titleStyle.Render(m.menuTitle) + "\n" + renderChoices([]string{"Yes", "No"}, m.menuIndex) + "\nEnter confirm • Esc cancel"
		case "viewlog":
			overlay = titleStyle.Render("Selected Work Log") + "\n" + m.message + "\n\ne edit • d delete • Esc back"
		}
		box := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(focusedBorder).Padding(1, 2).Width(52).Render(overlay)
		body = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	if m.message != "" && m.mode == "" {
		body += "\n" + normalStyle.Render(m.message)
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}
func renderChoices(items []string, selected int) string {
	var b strings.Builder
	for i, s := range items {
		if i == selected {
			b.WriteString(selectedStyle.Render("> " + s))
		} else {
			b.WriteString("  " + s)
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
}
func fieldTitle(f string) string {
	switch f {
	case "title":
		return "Title"
	case "description":
		return "Description"
	case "start_time":
		return "Start date/time (YYYY-MM-DD HH:MM)"
	case "due_time":
		return "Due date/time (YYYY-MM-DD HH:MM)"
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
			m.openMenu("Task Actions", "Create Task", "Edit", "Add Work Log", "Work Logs", "Alerts", "Add Sub-task", "Complete", "Cancel", "Delete", "Back")
		} else {
			m.openMenu("Sub-task Actions", "Edit", "Add Work Log", "Work Logs", "Alerts", "Complete", "Cancel", "Delete", "Back")
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
		case "Edit":
			m.beginForm("Edit Task", "edit", []string{"title", "description", "start_time", "due_time", "priority", "status", "recurrence"}, map[string]string{"path": p, "title": readField(p, "title"), "description": readField(p, "description"), "start_time": readField(p, "start_time"), "due_time": readField(p, "due_time"), "priority": readField(p, "priority"), "status": readField(p, "status"), "recurrence": readField(p, "recurrence")})
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
		case "Edit":
			m.beginForm("Edit Sub-task", "edit", []string{"title", "description", "start_time", "due_time", "priority", "status", "recurrence"}, map[string]string{"path": p, "title": readField(p, "title"), "description": readField(p, "description"), "start_time": readField(p, "start_time"), "due_time": readField(p, "due_time"), "priority": readField(p, "priority"), "status": readField(p, "status"), "recurrence": readField(p, "recurrence")})
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
		m.width = msg.Width
		m.height = msg.Height
		m.logs.SetWidth(m.width/3 - 6)
		m.logs.SetHeight(m.height/3 - 5)
	case tea.KeyPressMsg:
		key := msg.String()
		if m.mode != "" {
			switch m.mode {
			case "menu":
				switch key {
				case "esc":
					m.mode = ""
				case "up", "k":
					if m.menuIndex > 0 {
						m.menuIndex--
					}
				case "down", "j":
					if m.menuIndex < len(m.menuItems)-1 {
						m.menuIndex++
					}
				case "enter":
					m.handleMenu()
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
			case "input":
				switch key {
				case "esc":
					m.mode = ""
				case "enter":
					m.nextFormField()
				case "backspace":
					if len(m.inputValue) > 0 {
						m.inputValue = m.inputValue[:len(m.inputValue)-1]
					}
				default:
					if len(key) == 1 {
						m.inputValue += key
					} else if strings.HasPrefix(key, " ") {
						m.inputValue += " "
					}
				}
			case "viewlog":
				switch key {
				case "esc":
					m.mode = ""
				case "e":
					if m.selectedLog < len(m.worklogs) {
						f := m.worklogs[m.selectedLog]
						m.beginForm("Edit Work Log", "editlog", []string{"log"}, map[string]string{"path": m.selectedPath(), "file": f, "log": workLogDescription(f)})
					}
				case "d":
					if m.selectedLog < len(m.worklogs) {
						_ = os.Remove(m.worklogs[m.selectedLog])
						m.mode = ""
						m.refresh()
					}
				}
			}
			return m, nil
		}
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right":
			m.activePanel = (m.activePanel + 1) % 6
		case "shift+tab", "left":
			m.activePanel = (m.activePanel + 5) % 6
		case "up", "k":
			switch m.activePanel {
			case 0:
				if m.selectedLabel > 0 {
					m.selectedLabel--
					m.currentParent = ""
					m.selectedTask = 0
					m.loadTasks()
				}
			case 1:
				if m.selectedTask > 0 {
					m.selectedTask--
					m.loadSelectedTask()
				}
			case 5:
				var c tea.Cmd
				m.logs, c = m.logs.Update(msg)
				return m, c
			}
		case "down", "j":
			switch m.activePanel {
			case 0:
				if m.selectedLabel < len(m.labels)-1 {
					m.selectedLabel++
					m.currentParent = ""
					m.selectedTask = 0
					m.loadTasks()
				}
			case 1:
				if m.selectedTask < len(m.tasks)-1 {
					m.selectedTask++
					m.loadSelectedTask()
				}
			case 5:
				var c tea.Cmd
				m.logs, c = m.logs.Update(msg)
				return m, c
			}
		case "enter":
			if m.activePanel == 0 && len(m.labels) > 0 {
				m.currentParent = m.selectedLabelPath()
				m.selectedTask = 0
				m.loadTasks()
				m.activePanel = 1
			} else if m.activePanel == 1 && len(m.tasks) > 0 {
				children := getDirs(m.selectedPath())
				if len(children) > 0 {
					m.currentParent = m.selectedPath()
					m.selectedTask = 0
					m.loadTasks()
				}
			}
		case "backspace":
			if m.currentParent != "" && m.currentParent != m.selectedLabelPath() {
				m.currentParent = filepath.Dir(m.currentParent)
				m.selectedTask = 0
				m.loadTasks()
				m.activePanel = 1
			}
		case "a":
			m.openActions()
		case "e":
			if p := m.selectedPath(); p != "" {
				m.openMenu("Task Actions", "Edit", "Back")
				m.activePanel = 1
			}
		case "w":
			if p := m.selectedPath(); p != "" {
				m.beginForm("Add Work Log", "log", []string{"log"}, map[string]string{"path": p})
			}
		case "r":
			m.refresh()
		case "pgdown", "pgup":
			if m.activePanel == 5 {
				var c tea.Cmd
				m.logs, c = m.logs.Update(msg)
				return m, c
			}
		}
	}
	return m, nil
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
