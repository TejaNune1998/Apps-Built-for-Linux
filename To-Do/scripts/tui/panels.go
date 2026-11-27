package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func counter(i, n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", i+1, n)
}

func (m *model) focusHit(x, y, w, h int, p panel) {
	m.addHit(x, y, w, h, func(m *model) tea.Cmd { m.focus = p; return nil })
}

// ------------------------------------------------------------------ Labels

func (m *model) labelsPanel(l layout) []string {
	w, h := l.lw, l.topH
	ih := h - 2
	focused := m.focus == pLabels
	m.focusHit(0, 1, w, h, pLabels)
	m.lOff = windowStart(m.li, m.lOff, ih)
	var lines []string
	for i := m.lOff; i < len(m.labels) && i < m.lOff+ih; i++ {
		i := i
		sel := i == m.li
		marker := " "
		if sel {
			marker = ">"
		}
		n := fmt.Sprint(len(getDirs(m.labels[i].path)))
		lines = append(lines, listRow(w-2, marker, []seg{{m.labels[i].name, sPlain}}, n, sDim, sel, focused))
		m.addHit(1, 2+i-m.lOff, w-2, 1, func(m *model) tea.Cmd {
			m.focus = pLabels
			if m.li != i {
				m.li = i
				m.onLabelChanged()
			}
			return nil
		})
	}
	if len(m.labels) == 0 {
		lines = []string{sDim.Render(" No labels yet."), sDim.Render(" Press F1 to add one.")}
	}
	return box("Labels", counter(m.li, len(m.labels)), lines, w, h, focused, false, nil)
}

// ------------------------------------------------------------------ Tasks

func statusGlyph(status string) (string, seg) {
	switch status {
	case "In Progress":
		return "◐ ", seg{"◐ ", sYel}
	case "On Hold":
		return "◼ ", seg{"◼ ", sRed}
	}
	return "○ ", seg{"○ ", sDim}
}

func shortDue(e entity) (string, bool) {
	t, err := parseTaskTime(e.due)
	if err != nil {
		return "", false
	}
	over := t.Before(time.Now()) && e.status != "Completed" && e.closed == ""
	s := t.Format("02 Jan")
	if over {
		s += "!"
	}
	return s, over
}

func (m *model) tasksPanel(l layout) []string {
	x0, w, h := l.lw, l.cw, l.topH
	ih := h - 2
	focused := m.focus == pTasks
	m.focusHit(x0, 1, w, h, pTasks)
	m.tOff = windowStart(m.ti, m.tOff, ih)
	var lines []string
	for i := m.tOff; i < len(m.rows) && i < m.tOff+ih; i++ {
		i := i
		r := m.rows[i]
		sel := i == m.ti
		marker := " "
		if sel {
			marker = ">"
		}
		var segs []seg
		right, rst := "", sDim
		switch r.kind {
		case rowTask:
			tree := "  "
			if r.e.children > 0 {
				tree = "▸ "
				if m.expanded[r.e.path] {
					tree = "▾ "
				}
			}
			_, g := statusGlyph(r.e.status)
			segs = []seg{{tree, sYel}, g, {r.e.title, sPlain}}
			if r.e.repeat != "" {
				segs = append(segs, seg{" ↻", sCyan})
			}
			var over bool
			if right, over = shortDue(r.e); over {
				rst = sRed
			}
		case rowSub:
			_, g := statusGlyph(r.e.status)
			segs = []seg{{"      ", sPlain}, g, {r.e.title, sPlain}}
			if r.e.repeat != "" {
				segs = append(segs, seg{" ↻", sCyan})
			}
			var over bool
			if right, over = shortDue(r.e); over {
				rst = sRed
			}
		case rowGroup:
			tree := "▸ "
			if m.closedOpen {
				tree = "▾ "
			}
			segs = []seg{{tree, sYel}, {fmt.Sprintf("Completed & Canceled (%d)", r.count), sYelB}}
		case rowClosed:
			g := seg{"✓ ", sGreen}
			if r.e.closed == "CANCEL" {
				g = seg{"✗ ", sRed}
			}
			segs = []seg{{"      ", sPlain}, g, {r.e.title, sDim}}
			right, _ = shortDue(r.e)
		}
		lines = append(lines, listRow(w-2, marker, segs, right, rst, sel, focused))
		m.addHit(x0+1, 2+i-m.tOff, w-2, 1, func(m *model) tea.Cmd {
			m.focus = pTasks
			m.ti = i
			m.loadSelection(false)
			row := m.rows[i]
			return m.clickRow(row, i)
		})
	}
	title := "Tasks"
	if lab := m.selectedLabel(); lab != "" {
		title = "Tasks · " + lab
	}
	if len(m.rows) == 0 {
		msg := " Select a label."
		if m.selectedLabel() != "" {
			msg = " No open tasks. F1 adds one."
		}
		lines = []string{sDim.Render(msg)}
	}
	return box(title, counter(m.ti, len(m.rows)), lines, w, h, focused, false, nil)
}

// clickRow: a click on the group row, or on a task that has sub-tasks, also expands it.
func (m *model) clickRow(r row, i int) tea.Cmd {
	if r.kind == rowGroup || (r.kind == rowTask && r.e.children > 0) {
		m.toggleRow(i)
	}
	return nil
}

// ------------------------------------------------------------------ Task Details

func (m *model) detailsPanel(l layout) []string {
	x0, w, h := l.lw+l.cw, l.rw, l.topH
	ih, iw := h-2, w-2
	focused := m.focus == pDetails
	m.focusHit(x0, 1, w, h, pDetails)
	title := "Task Details"
	if m.sel != nil && m.sel.isSub {
		title = "Task Details · sub-task"
	}
	if m.sel == nil {
		msg := " Nothing selected."
		if len(m.labels) == 0 {
			msg = " Add a label (F1), then tasks."
		}
		return box(title, "", []string{sDim.Render(msg)}, w, h, focused, false, nil)
	}
	lines := m.dLines()
	stops := fieldStops(lines)
	cur := -1
	if len(stops) > 0 {
		m.cur = clamp(m.cur, 0, len(stops)-1)
		if focused {
			cur = stops[m.cur]
		}
	}
	maxOff := maxInt(len(lines)-ih, 0)
	m.dOff = clamp(m.dOff, 0, maxOff)
	var out []string
	for i := m.dOff; i < len(lines) && i < m.dOff+ih; i++ {
		i := i
		ln := lines[i]
		if i == cur {
			out = append(out, sBar.Render(fit("> "+ln.plain, iw)))
		} else {
			out = append(out, "  "+ln.text)
		}
		field := ln.field
		m.addHit(x0+1, 1+1+i-m.dOff, iw, 1, func(m *model) tea.Cmd {
			m.focus = pDetails
			if field == "" {
				return nil
			}
			for k, s := range stops {
				if lines[s].field == field {
					m.cur = k
				}
			}
			return m.editCurrentField()
		})
	}
	pct := 100
	if maxOff > 0 {
		pct = m.dOff * 100 / maxOff
	}
	return box(title, fmt.Sprintf("%d%%", pct), out, w, h, focused, false, &sbar{len(lines), ih, m.dOff})
}

// ------------------------------------------------------------------ Task Counts (display only)

func (m *model) countsPanel(l layout) []string {
	w, h := l.lw, l.botH
	c := m.cnt
	lab := m.selectedLabel()
	if lab == "" {
		lab = "-"
	}
	lines := []string{
		"",
		" " + sBold.Render("Label: ") + sYelB.Render(lab),
		"",
		sDim.Render(fmt.Sprintf(" %-10s%5s%6s", "", "Tasks", "Subs")),
		fmt.Sprintf(" %s%s%s", sGreen.Bold(true).Render(fmt.Sprintf("%-10s", "Open")), sBold.Render(fmt.Sprintf("%5d", c.openTasks)), sBold.Render(fmt.Sprintf("%6d", c.openSubs))),
		fmt.Sprintf(" %s%s%s", sCyanB.Render(fmt.Sprintf("%-10s", "Completed")), sBold.Render(fmt.Sprintf("%5d", c.doneTasks)), sBold.Render(fmt.Sprintf("%6d", c.doneSubs))),
	}
	return box("Task Counts", "", lines, w, h, false, true, nil)
}

// ------------------------------------------------------------------ Today

func (m *model) todayPanel(l layout) []string {
	x0, y0, w, h := l.lw, 1+l.topH, l.cw, l.botH
	ih, iw := h-2, w-2
	focused := m.focus == pToday
	m.focusHit(x0, y0, w, h, pToday)
	rows := maxInt(ih-1, 1) // the last line shows a hint
	m.tdOff = windowStart(m.tdCur, m.tdOff, rows)
	var lines []string
	for i := m.tdOff; i < len(m.today) && i < m.tdOff+rows; i++ {
		i := i
		it := m.today[i]
		glyph, gst, tag := "▶", sGreen, "Start"
		if it.kind == "due" {
			glyph, gst, tag = "■", sYel, "Due  "
			if it.overdue {
				gst = sRed
			}
		}
		title := it.title
		if it.isSub {
			title = "↳ " + title
		}
		tst := sPlain
		if it.overdue {
			tst = sRed
		}
		sel := i == m.tdCur
		marker := " "
		if sel {
			marker = ">"
		}
		segs := []seg{{it.at.Format("15:04"), sDim}, {"  ", sPlain}, {glyph, gst}, {" ", sPlain}, {tag, tst}, {"  ", sPlain}, {title, tst}}
		lines = append(lines, listRow(iw, marker, segs, "", sDim, sel, focused))
		m.addHit(x0+1, y0+1+i-m.tdOff, iw, 1, func(m *model) tea.Cmd {
			m.focus = pToday
			m.tdCur = i
			return m.openUpcoming(true)
		})
	}
	if len(m.today) == 0 {
		lines = []string{sDim.Render(" Nothing starts or is due today.")}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	lines = append(lines, sDim.Render(" Enter ▸ upcoming start / due"))
	footer := fmt.Sprintf("%d items", len(m.today))
	if len(m.today) == 1 {
		footer = "1 item"
	}
	return box("Today · "+time.Now().Format("Mon 02 Jan"), footer, lines, w, h, focused, false, nil)
}

// ------------------------------------------------------------------ Work Logs

type logLine struct {
	text  string
	plain string
	entry int
	head  bool
}

func (m *model) logLines(w int) []logLine {
	var out []logLine
	for i, e := range m.logs {
		out = append(out, logLine{text: sYelB.Render(e.stamp), plain: e.stamp, entry: i, head: true})
		for _, t := range wrapPlain(e.text, w-2) {
			out = append(out, logLine{text: "  " + t, plain: "  " + t, entry: i})
		}
		out = append(out, logLine{entry: i})
	}
	return out
}

// logScrollFor returns a scroll offset that shows the header of entry i.
func (m *model) logScrollFor(i int) int {
	l := m.layout()
	ih := l.botH - 2
	for n, ln := range m.logLines(l.rw - 4) {
		if ln.head && ln.entry == i {
			if n < m.lScroll {
				return n
			}
			if n >= m.lScroll+ih {
				return n
			}
			return m.lScroll
		}
	}
	return m.lScroll
}

func (m *model) logsPanel(l layout) []string {
	x0, y0, w, h := l.lw+l.cw, 1+l.topH, l.rw, l.botH
	ih, iw := h-2, w-2
	focused := m.focus == pLogs
	m.focusHit(x0, y0, w, h, pLogs)
	if m.sel == nil {
		return box("Work Logs", "", []string{sDim.Render(" Select a task or sub-task.")}, w, h, focused, false, nil)
	}
	if len(m.logs) == 0 {
		return box("Work Logs", "", []string{sDim.Render(" No work log yet. F3 adds one.")}, w, h, focused, false, nil)
	}
	lines := m.logLines(iw - 2)
	m.lScroll = clamp(m.lScroll, 0, maxInt(len(lines)-ih, 0))
	var out []string
	for i := m.lScroll; i < len(lines) && i < m.lScroll+ih; i++ {
		ln := lines[i]
		entry := ln.entry
		switch {
		case ln.head && focused && entry == m.lcur:
			out = append(out, sBar.Render(fit("> "+ln.plain, iw)))
		case ln.head && entry == m.lcur:
			out = append(out, sYel.Render(">")+" "+ln.text)
		case ln.head:
			out = append(out, "  "+ln.text)
		default:
			out = append(out, ln.text)
		}
		m.addHit(x0+1, y0+1+i-m.lScroll, iw, 1, func(m *model) tea.Cmd {
			m.focus = pLogs
			m.lcur = entry
			return nil
		})
	}
	footer := fmt.Sprintf("%d logs", len(m.logs))
	return box("Work Logs", footer, out, w, h, focused, false, &sbar{len(lines), ih, m.lScroll})
}

var _ = strings.TrimSpace
