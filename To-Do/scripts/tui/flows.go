package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const tl = "2006-01-02 15:04"

var errName = errors.New("Names cannot be empty, start with '.', 'DONE ', 'CANCEL ' or 'DELETED ', or contain '/' or '_'.")

func validateName(parent, current string) func(string) error {
	return func(v string) error {
		if v == current {
			return nil
		}
		if !cleanName(v) {
			return errName
		}
		if _, e := os.Stat(filepath.Join(parent, folderName(v))); e == nil {
			return fmt.Errorf("'%s' already exists.", v)
		}
		return nil
	}
}

func nonEmpty(msg string) func(string) error {
	return func(v string) error {
		if strings.TrimSpace(v) == "" {
			return errors.New(msg)
		}
		return nil
	}
}

// fkey runs a function key shortcut.
func (m *model) fkey(n int) tea.Cmd {
	switch n {
	case 1:
		if m.focus == pLabels {
			return m.openAddLabel()
		}
		return m.openAddTask()
	case 2:
		return m.openAddSub()
	case 3:
		return m.openAddWorkLog()
	case 4:
		return m.openClose("DONE", "Completed")
	case 5:
		return m.openClose("CANCEL", "Canceled")
	case 6:
		return m.openDeleteLabel()
	case 7:
		return m.openUpcoming(false)
	case 8:
		m.refresh()
		m.say("Reloaded.", false)
	case 9:
		m.dlg = &helpDialog{w: dlgWidth(m, 62)}
	}
	return nil
}

func (m *model) after(path, msg string) {
	m.selPath = path
	m.refresh()
	m.say(msg, false)
}

func (m *model) fail(err error) { m.say(err.Error(), true) }

// ------------------------------------------------------------------ add

func (m *model) openAddLabel() tea.Cmd {
	m.dlg = newInput(m, "Add label", "Label name", "", validateName(m.workDir, ""), func(m *model, v string) tea.Cmd {
		p := filepath.Join(m.workDir, folderName(v))
		if e := os.MkdirAll(p, 0755); e != nil {
			m.fail(e)
			return nil
		}
		m.refresh()
		for i, l := range m.labels {
			if l.path == p {
				m.li = i
				m.onLabelChanged()
			}
		}
		m.say("Label '"+v+"' created.", false)
		return nil
	})
	return nil
}

type draft struct {
	parent, kind, title, desc, priority, status, repeat string
	start, due                                          time.Time
	descs                                               []string
	custom                                              string
}

func (m *model) openAddTask() tea.Cmd {
	lp := m.labelPath()
	if lp == "" {
		m.say("Add a label first (select Labels, then F1).", true)
		return nil
	}
	return m.startCreate(lp, "task", m.selectedLabel())
}

func (m *model) openAddSub() tea.Cmd {
	if m.sel == nil || m.sel.closed != "" {
		m.say("Select an open task first.", true)
		return nil
	}
	parent := m.sel.path
	if m.sel.isSub {
		parent = filepath.Dir(m.sel.path) // only a task can have sub-tasks
	}
	return m.startCreate(parent, "sub-task", loadEntity(m.workDir, parent).title)
}

func (m *model) startCreate(parent, kind, where string) tea.Cmd {
	d := &draft{parent: parent, kind: kind}
	name := "Add " + kind + " · "
	m.dlg = newInput(m, name+"Title (1/9)", "Title of the "+kind+" in '"+where+"'", "", validateName(parent, ""), func(m *model, v string) tea.Cmd {
		d.title = v
		m.createDesc(d, name)
		return nil
	})
	return nil
}

func (m *model) createDesc(d *draft, name string) {
	t := newText(m, name+"Description (2/9)", "Short description (optional)", "", func(m *model, v string) tea.Cmd {
		d.desc = v
		m.createStart(d, name)
		return nil
	})
	m.dlg = t
}

func (m *model) createStart(d *draft, name string) {
	init := time.Now().Truncate(15 * time.Minute).Add(15 * time.Minute)
	m.dlg = newDT(m, name+"Start (3/9)", d.title+" › Start", init, nil, func(m *model, t time.Time) tea.Cmd {
		d.start = t
		m.createDue(d, name)
		return nil
	})
}

func (m *model) createDue(d *draft, name string) {
	m.dlg = newDT(m, name+"Due (4/9)", d.title+" › Due", d.start.AddDate(0, 0, 1), func(t time.Time) error {
		if t.Before(d.start) {
			return errors.New("Due time is before the start time.")
		}
		return nil
	}, func(m *model, t time.Time) tea.Cmd {
		d.due = t
		m.createPriority(d, name)
		return nil
	})
}

func indexOf(opts []string, v string) int {
	for i, o := range opts {
		if o == v {
			return i
		}
	}
	return 0
}

func (m *model) radio(title, sub string, opts []string, cur string, onPick func(m *model, v string) tea.Cmd) *listDialog {
	d := newList(m, title, opts)
	d.mode = lmRadio
	d.sub = sub
	d.cur = indexOf(opts, cur)
	if cur != "" {
		d.note = "Current value: " + cur
	}
	d.onOK = func(m *model, d *listDialog) tea.Cmd { return onPick(m, d.opts[d.cur]) }
	return d
}

var priorities = []string{"Low", "Medium", "High", "Critical"}
var statuses = []string{"Todo", "In Progress", "On Hold"}

func (m *model) createPriority(d *draft, name string) {
	m.dlg = m.radio(name+"Priority (5/9)", d.title, priorities, "Medium", func(m *model, v string) tea.Cmd {
		d.priority = v
		m.createStatus(d, name)
		return nil
	})
}

func (m *model) createStatus(d *draft, name string) {
	m.dlg = m.radio(name+"Status (6/9)", d.title, statuses, "Todo", func(m *model, v string) tea.Cmd {
		d.status = v
		m.askRepeat(name+"Repeat (7/9)", d.title, "", func(m *model, step string) tea.Cmd {
			d.repeat = step
			m.askAlerts(name+"Alerts (8/9)", d.title, d.start, d.due, func(m *model, lines []string) tea.Cmd {
				m.createConfirm(d, name, lines)
				return nil
			})
			return nil
		})
		return nil
	})
}

func (m *model) createConfirm(d *draft, name string, lines []string) {
	msg := fmt.Sprintf("Create '%s'?\nStart %s · Due %s\n%s priority · %s · Repeat: %s · %d alert(s)",
		d.title, d.start.Format("02 Jan 15:04"), d.due.Format("02 Jan 15:04"), d.priority, d.status, displayRecurrence(d.repeat), len(lines))
	m.dlg = newConfirm(m, name+"Confirm (9/9)", msg, false, func(m *model) tea.Cmd {
		if e := createEntity(d.parent, d.title, d.desc, d.start.Format(tl), d.due.Format(tl), d.priority, d.status, d.repeat, lines); e != nil {
			m.fail(e)
			return nil
		}
		p := filepath.Join(d.parent, folderName(d.title))
		if d.kind == "sub-task" {
			m.expanded[d.parent] = true
		}
		m.focus = pTasks
		m.after(p, strings.ToUpper(d.kind[:1])+d.kind[1:]+" '"+d.title+"' created.")
		return nil
	})
}

// askRepeat asks for the repeat step; custom repeats ask for a number and a unit.
func (m *model) askRepeat(title, sub, cur string, done func(m *model, step string) tea.Cmd) {
	opts := []string{"None", "Daily", "Weekly", "Every 2 Weeks", "Monthly", "Yearly", "Custom"}
	steps := map[string]string{"None": "", "Daily": "1 day", "Weekly": "1 week", "Every 2 Weeks": "2 weeks", "Monthly": "1 month", "Yearly": "1 year"}
	curName := "None"
	for k, v := range steps {
		if v == cur && cur != "" {
			curName = k
		}
	}
	if cur != "" && curName == "None" {
		curName = "Custom"
	}
	d := m.radio(title, "Repeat · "+sub, opts, curName, func(m *model, v string) tea.Cmd {
		if v != "Custom" {
			return done(m, steps[v])
		}
		m.dlg = newInput(m, title, "Repeat every how many?", "1", func(s string) error {
			if n, e := strconv.Atoi(s); e != nil || n < 1 {
				return errors.New("Enter a whole number greater than 0.")
			}
			return nil
		}, func(m *model, n string) tea.Cmd {
			m.dlg = m.radio(title, "Unit", []string{"days", "weeks", "months", "years"}, "weeks", func(m *model, u string) tea.Cmd {
				return done(m, n+" "+u)
			})
			return nil
		})
		return nil
	})
	d.note = "Completing a repeating task creates the next one. Canceling stops the repeat."
	m.dlg = d
}

func alertPresets() []string {
	out := []string{"At start time", "At due time"}
	for _, o := range []string{"5 minutes", "10 minutes", "15 minutes", "30 minutes", "1 hour", "2 hours", "1 day", "2 days", "3 days", "1 week"} {
		out = append(out, o+" before start")
	}
	for _, o := range []string{"1 day", "2 days", "3 days", "4 days", "5 days", "6 days", "1 week"} {
		out = append(out, o+" before due")
	}
	return out
}

func alertLineFor(desc string, start, due time.Time) (string, bool) {
	var at time.Time
	switch {
	case desc == "At start time":
		at = start
	case desc == "At due time":
		at = due
	case strings.HasSuffix(desc, " before start"):
		d, ok := offsetSeconds(strings.TrimSuffix(desc, " before start"))
		if !ok {
			return "", false
		}
		at = start.Add(-d)
	case strings.HasSuffix(desc, " before due"):
		d, ok := offsetSeconds(strings.TrimSuffix(desc, " before due"))
		if !ok {
			return "", false
		}
		at = due.Add(-d)
	default:
		return "", false
	}
	return at.Format(tl) + "|" + desc, true
}

// askAlerts lets the user tick up to 5 preset alerts and optionally one custom time.
func (m *model) askAlerts(title, sub string, start, due time.Time, done func(m *model, lines []string) tea.Cmd) {
	d := newList(m, title, alertPresets())
	d.mode = lmMulti
	d.limit = 5
	d.sub = "Alerts · " + sub
	d.note = "Space ticks an alert (up to 5). Enter continues."
	d.onOK = func(m *model, d *listDialog) tea.Cmd {
		var lines []string
		for _, p := range d.picked() {
			if l, ok := alertLineFor(p, start, due); ok {
				lines = append(lines, l)
			}
		}
		if len(lines) >= 5 {
			return done(m, lines)
		}
		c := newConfirm(m, title, "Add a custom alert time?", true, func(m *model) tea.Cmd {
			m.dlg = newDT(m, title, sub+" › Custom alert", start, nil, func(m *model, t time.Time) tea.Cmd {
				return done(m, append(lines, t.Format(tl)+"|Custom alert"))
			})
			return nil
		})
		c.onCancel = func(m *model) tea.Cmd { return done(m, lines) }
		m.dlg = c
		return nil
	}
	m.dlg = d
}

// ------------------------------------------------------------------ edit a field

func (m *model) editCurrentField() tea.Cmd {
	if m.sel == nil {
		return nil
	}
	if m.sel.closed != "" {
		m.say("Completed and canceled tasks are read-only.", true)
		return nil
	}
	lines := m.dLines()
	stops := fieldStops(lines)
	if len(stops) == 0 {
		return nil
	}
	m.cur = clamp(m.cur, 0, len(stops)-1)
	return m.editField(*m.sel, lines[stops[m.cur]].field)
}

func (m *model) editField(e entity, field string) tea.Cmd {
	parent := filepath.Dir(e.path)
	save := func(m *model, msg string, fields map[string]string) tea.Cmd {
		if err := saveEntityFields(e.path, fields); err != nil {
			m.fail(err)
			return nil
		}
		np := e.path
		if t, ok := fields["title"]; ok {
			np = filepath.Join(parent, folderName(t))
		}
		m.after(np, msg)
		return nil
	}
	switch field {
	case "title":
		m.dlg = newInput(m, "Edit: Title", "Title", e.title, validateName(parent, e.title), func(m *model, v string) tea.Cmd {
			if v == e.title {
				return nil
			}
			return save(m, "Title changed.", map[string]string{"title": v})
		})
	case "description":
		d := newText(m, "Edit: Description", e.title, readField(e.path, "description"), func(m *model, v string) tea.Cmd {
			return save(m, "Description updated.", map[string]string{"description": v})
		})
		m.dlg = d
	case "priority":
		m.dlg = m.radio("Edit: Priority", e.title, priorities, e.priority, func(m *model, v string) tea.Cmd {
			return save(m, "Priority updated.", map[string]string{"priority": v})
		})
	case "status":
		m.dlg = m.radio("Edit: Status", e.title, statuses, e.status, func(m *model, v string) tea.Cmd {
			return save(m, "Status updated.", map[string]string{"status": v})
		})
	case "start_time", "due_time":
		m.editTime(e, field)
	case "recurrence":
		m.askRepeat("Edit: Repeat", e.title, e.repeat, func(m *model, step string) tea.Cmd {
			t := filepath.Join(e.path, ".tmp", "alerts_template")
			if _, err := os.Stat(t); err != nil { // repeating tasks rebuild alerts from this copy
				if b, err := os.ReadFile(filepath.Join(e.path, ".tmp", "alerts")); err == nil {
					_ = os.WriteFile(t, b, 0644)
				}
			}
			return save(m, "Repeat updated.", map[string]string{"recurrence": step})
		})
	case "alerts":
		m.editAlerts(e)
	}
	return nil
}

func (m *model) editTime(e entity, field string) {
	start, _ := parseTaskTime(e.start)
	due, _ := parseTaskTime(e.due)
	label, init := "Start", start
	if field == "due_time" {
		label, init = "Due", due
	}
	m.dlg = newDT(m, "Edit: "+label+" Date & Time", e.title+" › "+label, init, func(t time.Time) error {
		if field == "due_time" && t.Before(start) {
			return errors.New("Due time is before the start time.")
		}
		if field == "start_time" && t.After(due) {
			return errors.New("Start time is after the due time.")
		}
		return nil
	}, func(m *model, t time.Time) tea.Cmd {
		if err := saveEntityFields(e.path, map[string]string{field: t.Format(tl)}); err != nil {
			m.fail(err)
			return nil
		}
		ns, nd := start, due
		if field == "start_time" {
			ns = t
		} else {
			nd = t
		}
		m.after(e.path, label+" time updated.")
		// Alerts are stored as exact times, so offer to rebuild them for the new dates.
		c := newConfirm(m, "Edit: "+label, "Dates changed. Reconfigure the alerts now?", false, func(m *model) tea.Cmd {
			m.askAlerts("Edit: Alerts", e.title, ns, nd, func(m *model, lines []string) tea.Cmd {
				if err := writeAlerts(e.path, lines); err != nil {
					m.fail(err)
					return nil
				}
				m.after(e.path, "Alerts rebuilt.")
				return nil
			})
			return nil
		})
		c.onCancel = func(m *model) tea.Cmd { return nil } // "No" keeps the alerts as they are
		m.dlg = c
		return nil
	})
}

func (m *model) editAlerts(e entity) {
	start, _ := parseTaskTime(e.start)
	due, _ := parseTaskTime(e.due)
	d := newList(m, "Edit: Alerts", []string{"Add Alert", "Remove Alert", "Clear All Alerts"})
	d.sub = e.title
	d.onOK = func(m *model, d *listDialog) tea.Cmd {
		switch d.opts[d.cur] {
		case "Add Alert":
			m.askAlerts("Add alerts", e.title, start, due, func(m *model, lines []string) tea.Cmd {
				if len(lines) == 0 {
					m.say("No alert selected.", false)
					return nil
				}
				if err := addAlerts(e.path, lines); err != nil {
					m.fail(err)
					return nil
				}
				m.after(e.path, fmt.Sprintf("%d alert(s) added.", len(lines)))
				return nil
			})
		case "Remove Alert":
			cur := readAlerts(e.path)
			if len(cur) == 0 {
				m.say("No alerts configured.", false)
				return nil
			}
			r := newList(m, "Remove alerts", cur)
			r.mode = lmMulti
			r.sub = e.title
			r.note = "Space ticks the alerts to remove."
			r.onOK = func(m *model, r *listDialog) tea.Cmd {
				for _, a := range r.picked() {
					if err := removeAlert(e.path, a); err != nil {
						m.fail(err)
						return nil
					}
				}
				m.after(e.path, "Alert(s) removed.")
				return nil
			}
			m.dlg = r
		case "Clear All Alerts":
			m.dlg = newConfirm(m, "Clear alerts", "Remove all alerts of '"+e.title+"'?", true, func(m *model) tea.Cmd {
				if err := writeAlerts(e.path, nil); err != nil {
					m.fail(err)
					return nil
				}
				m.after(e.path, "All alerts removed.")
				return nil
			})
		}
		return nil
	}
	m.dlg = d
}

// ------------------------------------------------------------------ work logs

func (m *model) openAddWorkLog() tea.Cmd {
	if m.sel == nil {
		m.say("Select a task or sub-task first.", true)
		return nil
	}
	if m.sel.closed != "" {
		m.say("Completed and canceled tasks are read-only.", true)
		return nil
	}
	e := *m.sel
	t := newText(m, "Add work log", e.title+" — what did you work on?", "", func(m *model, v string) tea.Cmd {
		if err := addWorkLog(e.path, v); err != nil {
			m.fail(err)
			return nil
		}
		m.focus = pLogs
		m.after(e.path, "Work log added.")
		return nil
	})
	t.validate = nonEmpty("Write something first.")
	m.dlg = t
	return nil
}

func (m *model) openLogMenu() tea.Cmd {
	if m.sel == nil || len(m.logs) == 0 || m.lcur >= len(m.logs) {
		return nil
	}
	if m.sel.closed != "" {
		m.say("Completed and canceled tasks are read-only.", true)
		return nil
	}
	e, lg := *m.sel, m.logs[m.lcur]
	d := newList(m, "Work log · "+lg.stamp, []string{"Edit", "Delete"})
	d.sub = e.title
	d.onOK = func(m *model, d *listDialog) tea.Cmd {
		if d.opts[d.cur] == "Edit" {
			t := newText(m, "Edit work log · "+lg.stamp, e.title, lg.text, func(m *model, v string) tea.Cmd {
				if err := editWorkLog(lg.file, v); err != nil {
					m.fail(err)
					return nil
				}
				_ = buildDetails(e.path)
				m.after(e.path, "Work log updated.")
				return nil
			})
			t.validate = nonEmpty("A work log cannot be empty.")
			m.dlg = t
			return nil
		}
		m.dlg = newConfirm(m, "Delete work log", "Delete the work log of "+lg.stamp+"?", true, func(m *model) tea.Cmd {
			if err := os.Remove(lg.file); err != nil {
				m.fail(err)
				return nil
			}
			_ = buildDetails(e.path)
			m.after(e.path, "Work log deleted.")
			return nil
		})
		return nil
	}
	m.dlg = d
	return nil
}

// ------------------------------------------------------------------ close / delete

func (m *model) openClose(prefix, status string) tea.Cmd {
	if m.sel == nil || m.sel.closed != "" {
		m.say("Select an open task or sub-task first.", true)
		return nil
	}
	e := *m.sel
	msg := fmt.Sprintf("Mark '%s' as %s?", e.title, status)
	if n := len(getDirs(e.path)); n > 0 {
		msg = fmt.Sprintf("Mark '%s' and its %d open sub-task(s) as %s?", e.title, n, status)
	}
	if e.repeat != "" {
		if prefix == "DONE" {
			msg += " The next occurrence will be created."
		} else {
			msg += " The repeat will stop."
		}
	}
	closeIt := func(m *model, addLog, desc string) tea.Cmd {
		err := closeEntity(e.path, prefix, status, addLog, desc)
		m.selPath = e.path
		m.refresh()
		if err != nil {
			m.fail(err)
			return nil
		}
		out := fmt.Sprintf("'%s' marked as %s.", e.title, status)
		if _, serr := os.Stat(e.path); serr == nil && prefix == "DONE" {
			out += " Next occurrence created."
		}
		m.say(out, false)
		return nil
	}
	m.dlg = newConfirm(m, status+" · "+e.title, msg, false, func(m *model) tea.Cmd {
		q := newConfirm(m, status+" · work log", "Add a work log before closing?", true, func(m *model) tea.Cmd {
			t := newText(m, "Work log · "+e.title, "What did you work on?", "", func(m *model, v string) tea.Cmd {
				return closeIt(m, "Yes", v)
			})
			t.validate = nonEmpty("Write something first.")
			m.dlg = t
			return nil
		})
		q.onCancel = func(m *model) tea.Cmd { return closeIt(m, "No", "") }
		m.dlg = q
		return nil
	})
	return nil
}

func (m *model) openDeleteLabel() tea.Cmd {
	lp := m.labelPath()
	if lp == "" {
		m.say("No label selected.", true)
		return nil
	}
	name := m.selectedLabel()
	n := len(getDirs(lp))
	m.dlg = newConfirm(m, "Delete label · "+name,
		fmt.Sprintf("Delete label '%s' and cancel its %d open task(s)? Open sub-tasks are canceled too.", name, n), true, func(m *model) tea.Cmd {
			if err := deleteLabel(lp); err != nil {
				m.fail(err)
				return nil
			}
			m.selPath = ""
			m.refresh()
			m.say("Label '"+name+"' deleted.", false)
			return nil
		})
	return nil
}

// ------------------------------------------------------------------ upcoming

func (m *model) openUpcoming(fromToday bool) tea.Cmd {
	from := dayStart(time.Now())
	items := m.agenda(from, from.AddDate(0, 0, 30))
	path, kind := "", ""
	if fromToday && m.tdCur < len(m.today) {
		path, kind = m.today[m.tdCur].path, m.today[m.tdCur].kind
	}
	d := newAgenda(m, items, path, kind)
	d.onShow = func(m *model, it agendaItem) tea.Cmd {
		m.jumpTo(it.path)
		return nil
	}
	m.dlg = d
	return nil
}
