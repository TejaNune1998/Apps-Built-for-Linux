package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func k(s string) tea.KeyPressMsg {
	fk := []rune{tea.KeyF1, tea.KeyF2, tea.KeyF3, tea.KeyF4, tea.KeyF5, tea.KeyF6, tea.KeyF7, tea.KeyF8, tea.KeyF9, tea.KeyF10}
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	if len(s) >= 2 && s[0] == 'f' {
		for i := 1; i <= 10; i++ {
			if s == "f"+string(rune('0'+i%10)) || (i == 10 && s == "f10") {
				if i == 10 && s == "f10" {
					return tea.KeyPressMsg{Code: fk[9]}
				}
				if i < 10 && s == "f"+string(rune('0'+i)) {
					return tea.KeyPressMsg{Code: fk[i-1]}
				}
			}
		}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func press(m *model, keys ...string) {
	for _, s := range keys {
		if len([]rune(s)) > 1 && !strings.Contains("enter esc tab shift+tab up down left right pgup pgdown space ctrl+s ctrl+c backspace f1 f2 f3 f4 f5 f6 f7 f8 f9 f10", s) {
			for _, r := range s {
				m.Update(k(string(r)))
			}
			continue
		}
		m.Update(k(s))
	}
}

func newTest(t *testing.T) *model {
	t.Helper()
	m := newModel(t.TempDir(), true)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return m
}

func mkLabel(t *testing.T, m *model, name string) string {
	t.Helper()
	p := filepath.Join(m.workDir, folderName(name))
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

func mkTask(t *testing.T, parent, title, repeat string, start, due time.Time, alerts []string) string {
	t.Helper()
	if err := createEntity(parent, title, "desc of "+title, start.Format(tl), due.Format(tl), "High", "Todo", repeat, alerts); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, folderName(title))
}

func screenText(m *model) []string {
	v := m.View()
	var out []string
	for _, l := range strings.Split(v.Content, "\n") {
		out = append(out, ansi.Strip(l))
	}
	return out
}

func TestLayoutSizes(t *testing.T) {
	for _, alt := range []bool{true, false} {
		for _, sz := range [][2]int{{90, 26}, {120, 34}, {160, 45}, {220, 60}} {
			m := newTest(t)
			m.alt = alt
			lab := mkLabel(t, m, "Work")
			mkTask(t, lab, "Weekly report", "1 week", time.Now(), time.Now().Add(26*time.Hour), []string{time.Now().Add(time.Hour).Format(tl) + "|At due time"})
			m.refresh()
			m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			for _, dlg := range []string{"", "f9", "f7", "f1"} {
				m.dlg = nil
				if dlg != "" {
					press(m, dlg)
				}
				lines := screenText(m)
				if len(lines) != m.usableH() {
					t.Errorf("alt=%v %v dlg=%q: %d lines, want %d", alt, sz, dlg, len(lines), m.usableH())
				}
				for i, l := range lines {
					if w := ansi.StringWidth(l); w != m.width {
						t.Errorf("alt=%v %v dlg=%q line %d width %d want %d: %q", alt, sz, dlg, i, w, m.width, l)
						break
					}
				}
			}
		}
	}
}

func TestQuitKeys(t *testing.T) {
	for _, key := range []string{"q", "esc", "f10", "ctrl+c"} {
		m := newTest(t)
		_, cmd := m.Update(k(key))
		if !m.quitting || cmd == nil {
			t.Errorf("%s should quit", key)
		}
	}
	m := newTest(t)
	press(m, "f9") // help open: Esc closes the box and does not quit
	press(m, "esc")
	if m.quitting || m.dlg != nil {
		t.Error("esc in a dialog must only close the dialog")
	}
}

func TestTabNeverReachesCounts(t *testing.T) {
	m := newTest(t)
	seen := map[panel]bool{}
	for i := 0; i < 12; i++ {
		seen[m.focus] = true
		press(m, "tab")
	}
	if len(seen) != 5 {
		t.Errorf("tab visits %d panels, want 5 (counts is display only)", len(seen))
	}
	// clicks on the counts panel do nothing
	l := m.layout()
	m.View()
	before := m.focus
	m.Update(tea.MouseClickMsg{X: 3, Y: 1 + l.topH + 3, Button: tea.MouseLeft})
	if m.focus != before || m.dlg != nil {
		t.Error("clicking Task Counts must do nothing")
	}
}

func TestAddFlowAndEditByKeyboard(t *testing.T) {
	m := newTest(t)
	press(m, "f1") // focus is Labels: add label
	press(m, "Work", "enter")
	if len(m.labels) != 1 || m.labels[0].name != "Work" {
		t.Fatalf("labels %+v", m.labels)
	}
	press(m, "enter") // Labels -> Tasks
	press(m, "f1")    // add task
	press(m, "Weekly report", "enter")
	press(m, "Prepare the numbers", "ctrl+s")
	press(m, "enter", "enter")                        // start, due: defaults
	press(m, "enter", "enter")                        // priority, status: defaults
	press(m, "down", "down", "down", "down", "enter") // repeat -> Monthly
	press(m, "space", "down", "space", "enter")       // tick two alerts
	press(m, "enter")                                 // custom alert? default No
	press(m, "enter")                                 // confirm
	if m.dlg != nil || len(m.rows) != 1 || m.rows[0].e.title != "Weekly report" {
		t.Fatalf("task not created: dlg=%v rows=%+v msg=%q", m.dlg, m.rows, m.msg)
	}
	e := m.rows[0].e
	if e.repeat != "1 month" || e.priority != "Medium" || e.status != "Todo" {
		t.Errorf("fields %+v", e)
	}
	if n := len(readAlerts(e.path)); n != 2 {
		t.Errorf("alerts %d", n)
	}
	// edit priority from Task Details with the > cursor
	press(m, "tab") // Tasks -> Details
	if m.focus != pDetails {
		t.Fatalf("focus %v", m.focus)
	}
	press(m, "down", "down", "down", "down") // title, description, start, due, priority
	press(m, "enter")
	if m.dlg == nil {
		t.Fatal("enter on the Priority line should open the edit dialog")
	}
	press(m, "down", "enter") // Medium -> High
	if got := readField(e.path, "priority"); got != "High" {
		t.Errorf("priority %q", got)
	}
	// rename through the title field: folder follows
	press(m, "up", "up", "up", "up", "up", "up") // clamp to the first stop (title)
	press(m, "enter")
	for i := 0; i < 20; i++ {
		press(m, "backspace")
	}
	press(m, "Report", "enter")
	if m.sel == nil || m.sel.title != "Report" || filepath.Base(m.sel.path) != "Report" {
		t.Fatalf("rename: %+v", m.sel)
	}
}

func TestMouseOpensFieldDialog(t *testing.T) {
	m := newTest(t)
	lab := mkLabel(t, m, "Work")
	mkTask(t, lab, "Pay rent", "", time.Now(), time.Now().Add(48*time.Hour), nil)
	m.refresh()
	lines := screenText(m)
	l := m.layout()
	y := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Priority:") && strings.Contains(ln, "•") {
			y = i
		}
	}
	if y < 0 {
		t.Fatalf("no Priority line on screen:\n%s", strings.Join(lines, "\n"))
	}
	m.Update(tea.MouseClickMsg{X: l.lw + l.cw + 8, Y: y, Button: tea.MouseLeft})
	ld, ok := m.dlg.(*listDialog)
	if !ok || ld.title != "Edit: Priority" {
		t.Fatalf("click on Priority opened %T", m.dlg)
	}
	// click High then OK
	m.View()
	for _, h := range m.hits {
		_ = h
	}
	press(m, "down", "enter")
	if readField(m.sel.path, "priority") != "Critical" && readField(m.sel.path, "priority") != "High" {
		t.Errorf("priority %q", readField(m.sel.path, "priority"))
	}
}

func TestTodayUpcomingAndJump(t *testing.T) {
	m := newTest(t)
	a := mkLabel(t, m, "Personal")
	w := mkLabel(t, m, "Work")
	now := time.Now()
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	mkTask(t, a, "Pay rent", "", noon.Add(-time.Hour), noon.AddDate(0, 0, 3), nil)
	parent := mkTask(t, w, "Weekly report", "", noon, noon.AddDate(0, 0, 2), nil)
	sub := mkTask(t, parent, "Draft slides", "", noon.AddDate(0, 0, 1), noon.AddDate(0, 0, 1).Add(2*time.Hour), nil)
	m.refresh()
	if len(m.today) != 2 { // Pay rent starts today, Weekly report starts today
		t.Fatalf("today items: %+v", m.today)
	}
	scr := strings.Join(screenText(m), "\n")
	if !strings.Contains(scr, "Today") || !strings.Contains(scr, "Start") {
		t.Errorf("Today panel missing:\n%s", scr)
	}
	// Today panel -> Enter opens the Upcoming dialog -> pick the sub-task -> jump
	press(m, "tab", "tab", "tab") // labels -> tasks -> details -> today
	if m.focus != pToday {
		t.Fatalf("focus %v", m.focus)
	}
	press(m, "enter")
	ad, ok := m.dlg.(*agendaDialog)
	if !ok {
		t.Fatalf("dialog %T", m.dlg)
	}
	target := -1
	for i, it := range ad.items {
		if it.path == sub && it.kind == "start" {
			target = i
		}
	}
	if target < 0 {
		t.Fatalf("sub-task missing from upcoming: %+v", ad.items)
	}
	for ad.cur < target {
		press(m, "down")
	}
	press(m, "enter")
	if m.dlg != nil || m.focus != pTasks || m.selPath != sub {
		t.Fatalf("jump failed: dlg=%v focus=%v sel=%q want %q", m.dlg, m.focus, m.selPath, sub)
	}
	if !m.expanded[parent] || m.selectedLabel() != "Work" {
		t.Errorf("parent not expanded / label not opened (expanded=%v label=%q)", m.expanded[parent], m.selectedLabel())
	}
	if !strings.Contains(strings.Join(screenText(m), "\n"), "Draft slides") {
		t.Error("details of the sub-task not shown")
	}
}

func TestCloseCountsAndClosedGroup(t *testing.T) {
	m := newTest(t)
	lab := mkLabel(t, m, "Work")
	now := time.Now()
	a := mkTask(t, lab, "Task A", "", now, now.AddDate(0, 0, 1), nil)
	mkTask(t, a, "Sub 1", "", now, now.AddDate(0, 0, 1), nil)
	mkTask(t, a, "Sub 2", "", now, now.AddDate(0, 0, 1), nil)
	mkTask(t, lab, "Task B", "", now, now.AddDate(0, 0, 1), nil)
	m.refresh()
	if m.cnt.openTasks != 2 || m.cnt.openSubs != 2 {
		t.Fatalf("counts %+v", m.cnt)
	}
	// expand Task A in the tree with the right arrow
	press(m, "enter", "right")
	if !m.expanded[a] || len(m.rows) != 4 {
		t.Fatalf("expand: rows=%d", len(m.rows))
	}
	press(m, "left")
	if m.expanded[a] {
		t.Error("left should collapse")
	}
	// Done on Task A: sub-tasks follow; answer Yes, then No to the work log question
	press(m, "f4", "enter")
	press(m, "enter") // "No" is the default focus of the work log question
	if m.dlg != nil {
		t.Fatalf("dialog still open: %T", m.dlg)
	}
	if m.cnt.openTasks != 1 || m.cnt.doneTasks != 1 || m.cnt.doneSubs != 2 || m.cnt.openSubs != 0 {
		t.Errorf("counts after done %+v", m.cnt)
	}
	// Completed & Canceled group appears collapsed, then expands
	var group = -1
	for i, r := range m.rows {
		if r.kind == rowGroup {
			group = i
		}
	}
	if group < 0 || m.closedOpen {
		t.Fatalf("group row missing or open: %+v", m.rows)
	}
	m.ti = group
	m.loadSelection(false)
	press(m, "enter")
	if !m.closedOpen || len(m.rows) != group+2 {
		t.Fatalf("group should list the closed task: %d rows", len(m.rows))
	}
	// closed tasks are read-only
	m.ti = group + 1
	m.loadSelection(false)
	m.focus = pDetails
	press(m, "enter")
	if m.dlg != nil || !m.msgErr {
		t.Error("closed task must not open an edit dialog")
	}
}

func TestDeleteLabel(t *testing.T) {
	m := newTest(t)
	lab := mkLabel(t, m, "Work")
	mkTask(t, lab, "Task A", "", time.Now(), time.Now().Add(time.Hour), nil)
	m.refresh()
	press(m, "f6") // default focus is No
	press(m, "left", "enter")
	if len(m.labels) != 0 {
		t.Errorf("label not deleted: %+v", m.labels)
	}
	if _, err := os.Stat(filepath.Join(m.workDir, "DELETED_Work", "CANCEL_Task_A")); err != nil {
		t.Errorf("task not canceled under DELETED_Work: %v", err)
	}
}

func TestWorkLogAddEditDelete(t *testing.T) {
	m := newTest(t)
	lab := mkLabel(t, m, "Work")
	p := mkTask(t, lab, "Task A", "", time.Now(), time.Now().Add(time.Hour), nil)
	m.refresh()
	press(m, "f3", "did the thing", "ctrl+s")
	if len(m.logs) != 1 || m.logs[0].text != "did the thing" {
		t.Fatalf("logs %+v msg=%q", m.logs, m.msg)
	}
	if !strings.Contains(readFileT(t, filepath.Join(p, "details.md")), "did the thing") {
		t.Error("latest work log must be in details.md")
	}
	if m.focus != pLogs {
		t.Fatalf("focus %v", m.focus)
	}
	press(m, "enter", "enter") // menu: Edit
	for i := 0; i < 20; i++ {
		press(m, "backspace")
	}
	press(m, "better text", "ctrl+s")
	if m.logs[0].text != "better text" {
		t.Errorf("edit: %q", m.logs[0].text)
	}
	press(m, "enter", "down", "enter", "left", "enter") // Delete -> Yes
	if len(m.logs) != 0 {
		t.Errorf("log not deleted: %+v", m.logs)
	}
}

func TestDetailsRenderedAsMarkdown(t *testing.T) {
	m := newTest(t)
	lab := mkLabel(t, m, "Work")
	mkTask(t, lab, "Task A", "1 week", time.Now(), time.Now().Add(time.Hour), nil)
	m.refresh()
	press(m, "f3", "log line", "ctrl+s")
	scr := strings.Join(screenText(m), "\n")
	for _, want := range []string{"Task A", "Description", "Work Log — ", "• Start Time:", "• Priority:", "Every week"} {
		if !strings.Contains(scr, want) {
			t.Errorf("details panel missing %q", want)
		}
	}
	for _, bad := range []string{"# Title", "## Description", "### Work Log"} {
		if strings.Contains(scr, bad) {
			t.Errorf("heading must not show its # marks: found %q", bad)
		}
	}
}

func readFileT(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDateTimeDialog(t *testing.T) {
	m := newTest(t)
	init := time.Date(2026, 11, 24, 15, 7, 0, 0, time.Local) // rounds to 15:00
	var got time.Time
	d := newDT(m, "t", "s", init, nil, func(m *model, v time.Time) tea.Cmd { got = v; return nil })
	m.dlg = d
	if d.h12 != 3 || d.min != 0 || !d.pm {
		t.Fatalf("init %d:%02d pm=%v", d.h12, d.min, d.pm)
	}
	press(m, "right", "down")               // 25 Nov -> +7 days = 2 Dec
	press(m, "pgup")                        // -1 month = 2 Nov
	press(m, "tab", "up")                   // hour 3 -> 4
	press(m, "tab", "up", "up", "up", "up") // minutes 00 -> 15 -> 30 -> 45 -> 00
	press(m, "tab", "up")                   // PM -> AM
	press(m, "enter")
	want := time.Date(2026, 11, 2, 4, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
	// hour wraps 12 -> 1 and 1 -> 12; minutes wrap
	d2 := newDT(m, "t", "s", time.Date(2026, 1, 31, 12, 45, 0, 0, time.Local), nil, nil)
	d2.hour(1)
	d2.minute(1)
	if d2.h12 != 1 || d2.min != 0 {
		t.Errorf("wrap up: %d:%02d", d2.h12, d2.min)
	}
	d2.hour(-1)
	d2.minute(-1)
	if d2.h12 != 12 || d2.min != 45 {
		t.Errorf("wrap down: %d:%02d", d2.h12, d2.min)
	}
	if got := addMonths(time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local), 1); got.Day() != 28 || got.Month() != time.February {
		t.Errorf("month end clamp: %v", got)
	}
	// validation keeps the dialog open
	bad := newDT(m, "t", "s", init, func(time.Time) error { return os.ErrInvalid }, func(m *model, v time.Time) tea.Cmd { t.Error("must not submit"); return nil })
	m.dlg = bad
	press(m, "enter")
	if m.dlg != bad || bad.err == "" {
		t.Error("invalid time should keep the dialog open with an error")
	}
}

func TestEditor(t *testing.T) {
	e := newEditor("hello world", true)
	e.pos = 5
	e.insert("\nbig")
	if e.value() != "hello\nbig world" {
		t.Errorf("%q", e.value())
	}
	vl := e.vlines(6)
	if len(vl) < 3 {
		t.Errorf("wrap lines %d", len(vl))
	}
	e.pos = len(e.r)
	e.key(k("up"), 6)
	if e.pos >= len(e.r) {
		t.Error("up should move the cursor")
	}
	one := newEditor("", false)
	one.insert("a\nb")
	if one.value() != "a b" {
		t.Errorf("single line keeps no newline: %q", one.value())
	}
}

func TestAddAlertsKeepsSentAlertsInTemplate(t *testing.T) {
	dir := t.TempDir()
	p := mkTask(t, dir, "T", "1 week", time.Now(), time.Now().Add(time.Hour), []string{"2026-12-01 09:00|At start time", "2026-12-02 09:00|At due time"})
	// the reminder removes a fired alert from alerts but not from the template
	_ = os.WriteFile(filepath.Join(p, ".tmp", "alerts"), []byte("2026-12-02 09:00|At due time\n"), 0644)
	if err := addAlerts(p, []string{"2026-12-03 09:00|Custom alert"}); err != nil {
		t.Fatal(err)
	}
	tmpl := readFileT(t, filepath.Join(p, ".tmp", "alerts_template"))
	if !strings.Contains(tmpl, "At start time") || !strings.Contains(tmpl, "Custom alert") {
		t.Errorf("template lost alerts:\n%s", tmpl)
	}
}

// findOnScreen returns the column and row of the first line containing text.
func findOnScreen(m *model, text string) (int, int) {
	for y, ln := range screenText(m) {
		if i := strings.Index(ln, text); i >= 0 {
			return ansi.StringWidth(ln[:i]), y
		}
	}
	return -1, -1
}

func TestDialogMouseClicks(t *testing.T) {
	m := newTest(t)
	lab := mkLabel(t, m, "Work")
	p := mkTask(t, lab, "Pay rent", "", time.Now(), time.Now().Add(48*time.Hour), nil)
	m.refresh()
	m.focus = pDetails
	press(m, "down", "down", "down", "down", "enter") // Priority dialog (Medium -> list)
	if m.dlg == nil {
		t.Fatal("no dialog")
	}
	x, y := findOnScreen(m, "Critical")
	if x < 0 {
		t.Fatal("Critical not on screen")
	}
	m.Update(tea.MouseClickMsg{X: x + 2, Y: y, Button: tea.MouseLeft})
	x, y = findOnScreen(m, "[ OK ]")
	m.Update(tea.MouseClickMsg{X: x + 3, Y: y, Button: tea.MouseLeft})
	if m.dlg != nil || readField(p, "priority") != "Critical" {
		t.Errorf("clicks: dlg=%v priority=%q", m.dlg, readField(p, "priority"))
	}

	// date/time dialog: ▲ above the hour raises it, Cancel closes
	m.focus = pDetails
	m.cur = 2 // Start Time
	press(m, "enter")
	d, ok := m.dlg.(*dtDialog)
	if !ok {
		t.Fatalf("dialog %T", m.dlg)
	}
	h0 := d.h12
	x, y = findOnScreen(m, "▲")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if d.h12 != h0%12+1 {
		t.Errorf("hour %d -> %d", h0, d.h12)
	}
	x, y = findOnScreen(m, "[ Cancel ]")
	m.Update(tea.MouseClickMsg{X: x + 4, Y: y, Button: tea.MouseLeft})
	if m.dlg != nil {
		t.Error("Cancel click should close the dialog")
	}
}

func TestSelectedRowKeepsColours(t *testing.T) {
	row := listRow(30, ">", []seg{{"◐ ", sYel}, {"Task", sPlain}}, "01 Jan", sDim, true, false)
	if !strings.Contains(row, "\x1b[") || ansi.StringWidth(row) != 30 {
		t.Errorf("width %d row %q", ansi.StringWidth(row), row)
	}
}
