package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const version = "1.0.0"

type panel int

const (
	pLabels panel = iota
	pTasks
	pDetails
	pToday
	pLogs
)

// Task Counts is display only, so it is not part of the Tab order.
var tabOrder = []panel{pLabels, pTasks, pDetails, pToday, pLogs}

type hit struct {
	x, y, w, h int
	fn         func(m *model) tea.Cmd
}

type tickMsg time.Time

type model struct {
	workDir       string
	width, height int
	alt           bool
	focus         panel

	labels     []item
	li, lOff   int
	expanded   map[string]bool
	closedOpen bool
	rows       []row
	ti, tOff   int

	sel     *entity
	selPath string
	md      string
	cur     int // index into the field stops of Task Details
	dOff    int

	logs    []logEntry
	lcur    int
	lScroll int

	today        []agendaItem
	tdCur, tdOff int
	cnt          counts

	dlg      dialog
	hits     []hit
	msg      string
	msgErr   bool
	quitting bool
}

func newModel(workDir string, alt bool) *model {
	m := &model{workDir: workDir, alt: alt, width: 120, height: 36, expanded: map[string]bool{}}
	m.refresh()
	return m
}

func (m *model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) say(s string, isErr bool) { m.msg, m.msgErr = s, isErr }

// ------------------------------------------------------------------ layout

type layout struct{ lw, cw, rw, topH, botH int }

func (m *model) usableH() int {
	if m.alt {
		return m.height
	}
	return m.height - 1
}

func (m *model) layout() layout {
	avail := m.usableH() - 3 // title bar, status line, function keys
	botH := clamp((avail*3+5)/8, 9, 14)
	lw := clamp(m.width*22/100, 20, 34)
	cw := clamp(m.width*34/100, 30, 60)
	return layout{lw: lw, cw: cw, rw: m.width - lw - cw, topH: avail - botH, botH: botH}
}

func (m *model) tooSmall() bool { return m.width < 90 || m.usableH() < 24 }

func (m *model) detailsTextW() int { return m.layout().rw - 5 }

// ------------------------------------------------------------------ data

func (m *model) refresh() {
	keepLabel := ""
	if m.li >= 0 && m.li < len(m.labels) {
		keepLabel = m.labels[m.li].path
	}
	keep := m.selPath
	m.labels = getDirs(m.workDir)
	m.li = clamp(m.li, 0, maxInt(len(m.labels)-1, 0))
	for i, l := range m.labels {
		if l.path == keepLabel {
			m.li = i
		}
	}
	m.loadRows(keep)
	m.today = m.agenda(dayStart(time.Now()), dayStart(time.Now()).AddDate(0, 0, 1))
	m.tdCur = clamp(m.tdCur, 0, maxInt(len(m.today)-1, 0))
}

func (m *model) loadRows(keep string) {
	m.buildRows()
	m.ti = clamp(m.ti, 0, maxInt(len(m.rows)-1, 0))
	for i, r := range m.rows {
		if r.kind != rowGroup && keep != "" && r.e.path == keep {
			m.ti = i
		}
	}
	m.cnt = counts{}
	if lp := m.labelPath(); lp != "" {
		m.cnt = labelCounts(lp)
	}
	m.loadSelection(true)
}

func (m *model) loadSelection(force bool) {
	var e *entity
	if m.ti >= 0 && m.ti < len(m.rows) && m.rows[m.ti].kind != rowGroup {
		x := m.rows[m.ti].e
		e = &x
	}
	path := ""
	if e != nil {
		path = e.path
	}
	changed := path != m.selPath
	m.sel, m.selPath = e, path
	if e == nil {
		m.md, m.logs = "", nil
		return
	}
	if changed || force {
		_ = buildDetails(path)
		b, _ := os.ReadFile(filepath.Join(path, "details.md"))
		m.md = string(b)
		m.logs = loadLogs(path)
	}
	if changed {
		m.cur, m.dOff, m.lcur, m.lScroll = 0, 0, 0, 0
	}
	m.lcur = clamp(m.lcur, 0, maxInt(len(m.logs)-1, 0))
}

func (m *model) onLabelChanged() {
	m.ti, m.tOff = 0, 0
	m.selPath = ""
	m.loadRows("")
}

// jumpTo opens the label of a task, expands its parent task when it is a
// sub-task, selects it and shows it in Task Details.
func (m *model) jumpTo(path string) {
	rel, err := filepath.Rel(m.workDir, path)
	if err != nil {
		return
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	for i, l := range m.labels {
		if l.path == filepath.Join(m.workDir, parts[0]) {
			if i != m.li {
				m.li = i
			}
		}
	}
	if len(parts) >= 3 {
		m.expanded[filepath.Join(m.workDir, parts[0], parts[1])] = true
	}
	m.focus = pTasks
	m.loadRows(path)
	m.say("", false)
}

func (m *model) selectedLabel() string {
	if m.li >= 0 && m.li < len(m.labels) {
		return m.labels[m.li].name
	}
	return ""
}

// ------------------------------------------------------------------ update

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		if m.dlg == nil {
			m.refresh()
		}
		return m, tick()
	case tea.KeyPressMsg:
		return m, m.onKey(msg)
	case tea.PasteMsg:
		if m.dlg != nil {
			return m, m.dlg.update(m, msg)
		}
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			return m, m.onClick(msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		m.onWheel(msg.X, msg.Y, msg.Button == tea.MouseWheelUp)
	}
	return m, nil
}

func (m *model) onKey(k tea.KeyPressMsg) tea.Cmd {
	if k.String() == "ctrl+c" {
		m.quitting = true
		return tea.Quit
	}
	if m.dlg != nil {
		if k.String() == "f10" {
			m.quitting = true
			return tea.Quit
		}
		return m.dlg.update(m, k)
	}
	m.msg = ""
	s := k.String()
	switch s {
	case "q", "esc", "f10":
		m.quitting = true
		return tea.Quit
	case "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9":
		return m.fkey(int(s[1] - '0'))
	case "tab":
		m.cycleFocus(1)
	case "shift+tab":
		m.cycleFocus(-1)
	case "up", "k", "down", "j", "pgup", "pgdown", "home", "end":
		m.navigate(s)
	case "left", "h", "right", "l", "enter":
		return m.horizontal(s)
	}
	return nil
}

func (m *model) cycleFocus(d int) {
	i := 0
	for n, p := range tabOrder {
		if p == m.focus {
			i = n
		}
	}
	m.focus = tabOrder[(i+d+len(tabOrder))%len(tabOrder)]
}

func (m *model) navigate(s string) {
	delta := 0
	switch s {
	case "up", "k":
		delta = -1
	case "down", "j":
		delta = 1
	case "home":
		delta = -1 << 20
	case "end":
		delta = 1 << 20
	}
	page := s == "pgup" || s == "pgdown"
	dir := -1
	if s == "pgdown" {
		dir = 1
	}
	l := m.layout()
	switch m.focus {
	case pLabels:
		if page {
			delta = dir * maxInt(l.topH-3, 1)
		}
		if len(m.labels) > 0 && delta != 0 {
			m.li = clamp(m.li+delta, 0, len(m.labels)-1)
			m.onLabelChanged()
		}
	case pTasks:
		if len(m.rows) > 0 {
			if page {
				delta = dir * maxInt(l.topH-3, 1)
			}
			m.ti = clamp(m.ti+delta, 0, len(m.rows)-1)
			m.loadSelection(false)
		}
	case pDetails:
		stops := fieldStops(m.dLines())
		switch {
		case page:
			m.dOff = maxInt(m.dOff+dir*maxInt(l.topH-4, 1), 0)
		case len(stops) > 0 && delta != 0:
			m.cur = clamp(m.cur+delta, 0, len(stops)-1)
			m.dOff = windowStart(stops[m.cur], m.dOff, l.topH-2)
		}
	case pToday:
		if page {
			delta = dir * maxInt(l.botH-3, 1)
		}
		if len(m.today) > 0 && delta != 0 {
			m.tdCur = clamp(m.tdCur+delta, 0, len(m.today)-1)
		}
	case pLogs:
		switch {
		case page:
			m.lScroll = maxInt(m.lScroll+dir*maxInt(l.botH-3, 1), 0)
		case delta != 0 && len(m.logs) > 0:
			m.lcur = clamp(m.lcur+delta, 0, len(m.logs)-1)
			m.lScroll = m.logScrollFor(m.lcur)
		}
	}
}

func (m *model) horizontal(s string) tea.Cmd {
	right := s == "right" || s == "l"
	enter := s == "enter"
	switch m.focus {
	case pLabels:
		if right || enter {
			m.focus = pTasks
		}
	case pTasks:
		if m.ti >= len(m.rows) {
			return nil
		}
		r := m.rows[m.ti]
		switch {
		case enter:
			if !m.toggleRow(m.ti) {
				m.focus = pDetails
			}
		case right:
			if (r.kind == rowTask && r.e.children > 0 && !m.expanded[r.e.path]) || (r.kind == rowGroup && !m.closedOpen) {
				m.toggleRow(m.ti)
			} else {
				m.focus = pDetails
			}
		default: // left
			switch {
			case r.kind == rowTask && m.expanded[r.e.path]:
				m.toggleRow(m.ti)
			case r.kind == rowGroup && m.closedOpen:
				m.toggleRow(m.ti)
			case r.kind == rowSub:
				m.moveToParent()
			default:
				m.focus = pLabels
			}
		}
	case pDetails:
		switch {
		case enter:
			return m.editCurrentField()
		case !right:
			m.focus = pTasks
		}
	case pToday:
		switch {
		case enter:
			return m.openUpcoming(true)
		case !right:
			m.focus = pDetails
		}
	case pLogs:
		switch {
		case enter:
			return m.openLogMenu()
		case !right:
			m.focus = pToday
		}
	}
	return nil
}

// toggleRow expands or collapses a task or the Completed & Canceled group.
func (m *model) toggleRow(i int) bool {
	r := m.rows[i]
	switch {
	case r.kind == rowGroup:
		m.closedOpen = !m.closedOpen
	case r.kind == rowTask && r.e.children > 0:
		m.expanded[r.e.path] = !m.expanded[r.e.path]
	default:
		return false
	}
	m.loadRows(m.selPath)
	for n, x := range m.rows { // keep the cursor on the same row
		if x.kind == r.kind && x.e.path == r.e.path {
			m.ti = n
		}
	}
	m.loadSelection(false)
	return true
}

func (m *model) moveToParent() {
	for i := m.ti; i >= 0; i-- {
		if m.rows[i].kind == rowTask {
			m.ti = i
			m.loadSelection(false)
			return
		}
	}
}

func (m *model) dLines() []mdLine { return renderMD(m.md, m.detailsTextW()) }

// ------------------------------------------------------------------ mouse

func (m *model) onClick(x, y int) tea.Cmd {
	for i := len(m.hits) - 1; i >= 0; i-- {
		h := m.hits[i]
		if x >= h.x && x < h.x+h.w && y >= h.y && y < h.y+h.h {
			return h.fn(m)
		}
	}
	return nil
}

func (m *model) panelAt(x, y int) (panel, bool) {
	l := m.layout()
	top, bot := 1, 1+l.topH
	switch {
	case y >= top && y < top+l.topH:
		switch {
		case x < l.lw:
			return pLabels, true
		case x < l.lw+l.cw:
			return pTasks, true
		}
		return pDetails, true
	case y >= bot && y < bot+l.botH:
		switch {
		case x < l.lw:
			return pLabels, false // Task Counts: no panel
		case x < l.lw+l.cw:
			return pToday, true
		}
		return pLogs, true
	}
	return pLabels, false
}

func (m *model) onWheel(x, y int, up bool) {
	if m.dlg != nil {
		return
	}
	p, ok := m.panelAt(x, y)
	if !ok {
		return
	}
	d := 3
	if up {
		d = -3
	}
	switch p {
	case pDetails:
		m.dOff = maxInt(m.dOff+d, 0)
	case pLogs:
		m.lScroll = maxInt(m.lScroll+d, 0)
	case pTasks:
		if len(m.rows) > 0 {
			m.ti = clamp(m.ti+d/3, 0, len(m.rows)-1)
			m.loadSelection(false)
		}
	case pLabels:
		if len(m.labels) > 0 {
			m.li = clamp(m.li+d/3, 0, len(m.labels)-1)
			m.onLabelChanged()
		}
	case pToday:
		if len(m.today) > 0 {
			m.tdCur = clamp(m.tdCur+d/3, 0, len(m.today)-1)
		}
	}
}

// ------------------------------------------------------------------ view

func (m *model) addHit(x, y, w, h int, fn func(m *model) tea.Cmd) {
	m.hits = append(m.hits, hit{x, y, w, h, fn})
}

func (m *model) View() tea.View {
	m.hits = m.hits[:0]
	var content string
	if m.tooSmall() {
		content = fmt.Sprintf("Please make the terminal at least 90x%d (now %dx%d).", 24+boolInt(!m.alt), m.width, m.height)
	} else {
		screen := m.screen()
		if m.dlg != nil {
			m.hits = m.hits[:0]
			screen = m.overlay(screen)
		}
		content = strings.Join(screen, "\n")
	}
	v := tea.NewView(content)
	v.AltScreen = m.alt
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *model) screen() []string {
	l := m.layout()
	var out []string
	out = append(out, m.titleBar())
	join := func(a, b, c []string) []string {
		var r []string
		for i := range a {
			r = append(r, a[i]+b[i]+c[i])
		}
		return r
	}
	out = append(out, join(m.labelsPanel(l), m.tasksPanel(l), m.detailsPanel(l))...)
	out = append(out, join(m.countsPanel(l), m.todayPanel(l), m.logsPanel(l))...)
	out = append(out, m.statusLine())
	out = append(out, m.fkeyBar())
	return out
}

func (m *model) titleBar() string {
	crumb := m.selectedLabel()
	if m.sel != nil {
		if m.sel.isSub {
			if p := loadEntity(m.workDir, filepath.Dir(m.sel.path)); p.title != "" {
				crumb += " › " + p.title
			}
		}
		crumb += " › " + m.sel.title
	}
	left := " To-Do " + version + " │ " + crumb
	clock := time.Now().Format("Mon 02 Jan  15:04") + " "
	gap := maxInt(m.width-ansi.StringWidth(left)-ansi.StringWidth(clock), 1)
	return sTitle.Render(fit(left+strings.Repeat(" ", gap)+clock, m.width))
}

func (m *model) statusLine() string {
	if m.msg != "" {
		if m.msgErr {
			return fit(" "+sRedB.Render("✗ "+m.msg), m.width)
		}
		return fit(" "+sGreen.Render(m.msg), m.width)
	}
	return fit(" "+sGreen.Render("Ready")+sDim.Render(" · "+m.workDir), m.width)
}

var fkeyLabels = []string{"Add", "Sub", "Log", "Done", "Cancel", "Delete", "Upcoming", "Reload", "Help", "Quit"}

func (m *model) fkeyBar() string {
	y := m.usableH() - 1
	var b strings.Builder
	x := 0
	for i, lb := range fkeyLabels {
		n := i + 1
		num := fmt.Sprint(n)
		cell := sBold.Render(num) + sBar.Render(fit(lb, 8))
		w := len(num) + 8
		b.WriteString(cell + "  ")
		nn := n
		m.addHit(x, y, w, 1, func(m *model) tea.Cmd {
			if nn == 10 {
				m.quitting = true
				return tea.Quit
			}
			return m.fkey(nn)
		})
		x += w + 2
	}
	return fit(b.String(), m.width)
}

// overlay draws the dialog over a dimmed copy of the screen, with a drop shadow.
func (m *model) overlay(screen []string) []string {
	dv := m.dlg.view(m)
	if len(dv.lines) == 0 {
		return screen
	}
	w, h := ansi.StringWidth(dv.lines[0]), len(dv.lines)
	x := maxInt((m.width-w)/2, 0)
	y := maxInt((len(screen)-h)/2, 1)
	shadow := sDim.Render("▒▒")
	out := make([]string, len(screen))
	for r, base := range screen {
		bl := sDim.Render(fit(ansi.Strip(base), m.width))
		switch {
		case r >= y && r < y+h:
			right := shadow
			if r == y {
				right = ""
			}
			rest := ansi.Cut(bl, x+w+ansi.StringWidth(right)/1, m.width)
			if r == y {
				rest = ansi.Cut(bl, x+w, m.width)
			}
			out[r] = ansi.Cut(bl, 0, x) + dv.lines[r-y] + right + rest
		case r == y+h:
			out[r] = ansi.Cut(bl, 0, x+2) + sDim.Render(strings.Repeat("▒", w)) + ansi.Cut(bl, x+2+w, m.width)
		default:
			out[r] = bl
		}
	}
	for _, hr := range dv.hits {
		m.hits = append(m.hits, hit{x + hr.x, y + hr.y, hr.w, hr.h, hr.fn})
	}
	return out
}
