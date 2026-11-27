package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// hitRel is a clickable area, relative to the dialog's top-left corner.
type hitRel struct {
	x, y, w, h int
	fn         func(m *model) tea.Cmd
}

type dview struct {
	lines []string
	hits  []hitRel
}

type dialog interface {
	update(m *model, msg tea.Msg) tea.Cmd
	view(m *model) dview
}

const (
	dpad        = 3  // left padding inside a dialog
	maxListRows = 10 // visible options in a list dialog
)

func dlgWidth(m *model, want int) int { return clamp(want, 40, m.width-4) }

// frame wraps content lines in a double border. Hits are given relative to the
// inner area (inside the border).
func frame(title string, w int, content []string, hits []hitRel) dview {
	lines := box(title, "", content, w, len(content)+2, true, false, nil)
	for i := range hits {
		hits[i].x++
		hits[i].y++
	}
	return dview{lines: lines, hits: hits}
}

func kc(s string) string { return sKey.Render(" " + s + " ") }

// buttonRow draws "[ OK ]   [ Cancel ]" centred in innerW columns.
func buttonRow(labels []string, focus int, innerW int, y int, fns []func(m *model) tea.Cmd) (string, []hitRel) {
	total := 0
	for _, l := range labels {
		total += len(l) + 6
	}
	total += 3 * (len(labels) - 1)
	x := maxInt((innerW-total)/2, 0)
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", x))
	var hits []hitRel
	for i, l := range labels {
		txt := " [ " + l + " ] "
		if i == focus {
			b.WriteString(sBtnOn.Render(txt))
		} else {
			b.WriteString(txt)
		}
		hits = append(hits, hitRel{x: x, y: y, w: len(txt), h: 1, fn: fns[i]})
		x += len(txt) + 3
		b.WriteString("   ")
	}
	return b.String(), hits
}

// ============================================================ list dialog

type listMode int

const (
	lmPlain listMode = iota
	lmRadio
	lmMulti
)

// listDialog is a pick-one list, a radio list, a tick list, or (with no
// options) a Yes/No question.
type listDialog struct {
	title, sub, msg, note string
	opts                  []string
	mode                  listMode
	cur, off              int
	sel                   map[int]bool
	limit                 int
	btns                  []string
	fb                    int // -1 = list, otherwise the focused button
	err                   string
	w                     int
	onOK                  func(m *model, d *listDialog) tea.Cmd
	onCancel              func(m *model) tea.Cmd
}

func newList(m *model, title string, opts []string) *listDialog {
	return &listDialog{title: title, opts: opts, btns: []string{"OK", "Cancel"}, fb: -1, sel: map[int]bool{}, w: dlgWidth(m, 58)}
}

// newConfirm is a Yes/No question. defNo puts the focus on "No".
func newConfirm(m *model, title, msg string, defNo bool, onYes func(m *model) tea.Cmd) *listDialog {
	d := &listDialog{title: title, msg: msg, btns: []string{"Yes", "No"}, sel: map[int]bool{}, w: dlgWidth(m, 58)}
	if defNo {
		d.fb = 1
	}
	d.onOK = func(m *model, _ *listDialog) tea.Cmd { return onYes(m) }
	return d
}

func (d *listDialog) picked() []string {
	var out []string
	for i, o := range d.opts {
		if d.sel[i] {
			out = append(out, o)
		}
	}
	return out
}

func (d *listDialog) cancel(m *model) tea.Cmd {
	m.dlg = nil
	if d.onCancel != nil {
		return d.onCancel(m)
	}
	m.say("Cancelled.", false)
	return nil
}

func (d *listDialog) press(m *model, i int) tea.Cmd {
	if i != 0 {
		return d.cancel(m)
	}
	m.dlg = nil
	if d.onOK != nil {
		return d.onOK(m, d)
	}
	return nil
}

func (d *listDialog) toggle(i int) {
	d.err = ""
	switch {
	case d.sel[i]:
		delete(d.sel, i)
	case d.limit > 0 && len(d.picked()) >= d.limit:
		d.err = fmt.Sprintf("At most %d can be ticked.", d.limit)
	default:
		d.sel[i] = true
	}
}

func (d *listDialog) move(delta int) {
	if len(d.opts) == 0 {
		return
	}
	d.cur = clamp(d.cur+delta, 0, len(d.opts)-1)
	d.off = windowStart(d.cur, d.off, maxListRows)
}

func (d *listDialog) update(m *model, msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	nb, has := len(d.btns), len(d.opts) > 0
	switch k.String() {
	case "esc":
		return d.cancel(m)
	case "tab":
		if d.fb == nb-1 {
			d.fb = 0
			if has {
				d.fb = -1
			}
		} else {
			d.fb++
		}
	case "shift+tab":
		if d.fb == -1 {
			d.fb = nb - 1
		} else if d.fb == 0 && has {
			d.fb = -1
		} else {
			d.fb = maxInt(d.fb-1, 0)
		}
	case "up", "k":
		if d.fb == -1 {
			d.move(-1)
		} else if has {
			d.fb = -1
		}
	case "down", "j":
		if d.fb == -1 {
			d.move(1)
		}
	case "pgup":
		d.move(-maxListRows)
	case "pgdown":
		d.move(maxListRows)
	case "left", "h":
		if d.fb >= 0 {
			d.fb = maxInt(d.fb-1, 0)
		}
	case "right", "l":
		if d.fb >= 0 {
			d.fb = minInt(d.fb+1, nb-1)
		}
	case "space", " ":
		if d.fb == -1 && d.mode == lmMulti && has {
			d.toggle(d.cur)
		}
	case "enter":
		if d.fb == -1 {
			return d.press(m, 0)
		}
		return d.press(m, d.fb)
	}
	return nil
}

func (d *listDialog) view(m *model) dview {
	inner := d.w - 2
	text := inner - dpad - 2
	pad := strings.Repeat(" ", dpad)
	var lines []string
	var hits []hitRel
	add := func(s string) { lines = append(lines, pad+s) }
	add("")
	if d.sub != "" {
		for _, l := range wrapPlain(d.sub, text) {
			add(sBold.Render(l))
		}
		add("")
	}
	for _, l := range wrapPlain(d.msg, text) {
		if d.msg != "" {
			add(l)
		}
	}
	if d.msg != "" {
		add("")
	}
	if len(d.opts) > 0 {
		d.off = windowStart(d.cur, d.off, maxListRows)
		end := minInt(len(d.opts), d.off+maxListRows)
		for i := d.off; i < end; i++ {
			i := i
			prefix := ""
			switch d.mode {
			case lmRadio:
				prefix = "( ) "
				if i == d.cur {
					prefix = "(•) "
				}
			case lmMulti:
				prefix = "[ ] "
				if d.sel[i] {
					prefix = "[x] "
				}
			}
			marker := "  "
			if i == d.cur {
				marker = "> "
			}
			row := marker + prefix + d.opts[i]
			if i == d.cur && d.fb == -1 {
				row = sBar.Render(fit(row, text))
			} else if i == d.cur {
				row = sBold.Render(row)
			}
			hits = append(hits, hitRel{x: dpad, y: len(lines), w: text, h: 1, fn: func(m *model) tea.Cmd {
				d.cur, d.fb = i, -1
				if d.mode == lmMulti {
					d.toggle(i)
				}
				return nil
			}})
			add(row)
		}
		if len(d.opts) > maxListRows {
			add(sDim.Render(fmt.Sprintf("  (%d/%d)", d.cur+1, len(d.opts))))
		}
		add("")
	}
	if d.note != "" {
		add(sDim.Render(d.note))
		add("")
	}
	if d.err != "" {
		add(sRed.Render("✗ " + d.err))
		add("")
	}
	fns := make([]func(m *model) tea.Cmd, len(d.btns))
	for i := range d.btns {
		i := i
		fns[i] = func(m *model) tea.Cmd { return d.press(m, i) }
	}
	row, bh := buttonRow(d.btns, d.fb, inner, len(lines), fns)
	lines = append(lines, row)
	hits = append(hits, bh...)
	lines = append(lines, "")
	return frame(d.title, d.w, lines, hits)
}

// ============================================================ input / text dialog

type editDialog struct {
	title, prompt, hint string
	ed                  *editor
	multi               bool
	fb                  int // -1 = editor, 0 = OK, 1 = Cancel
	err                 string
	w                   int
	validate            func(v string) error
	onOK                func(m *model, v string) tea.Cmd
}

func newInput(m *model, title, prompt, value string, validate func(string) error, onOK func(m *model, v string) tea.Cmd) *editDialog {
	return &editDialog{title: title, prompt: prompt, ed: newEditor(value, false), fb: -1, w: dlgWidth(m, 58), validate: validate, onOK: onOK}
}

func newText(m *model, title, prompt, value string, onOK func(m *model, v string) tea.Cmd) *editDialog {
	return &editDialog{title: title, prompt: prompt, ed: newEditor(value, true), multi: true, fb: -1, w: dlgWidth(m, 66), onOK: onOK}
}

func (d *editDialog) edWidth() int { return d.w - 2 - dpad - 4 }

func (d *editDialog) cancel(m *model) tea.Cmd {
	m.dlg = nil
	m.say("Cancelled.", false)
	return nil
}

func (d *editDialog) submit(m *model) tea.Cmd {
	v := strings.TrimSpace(d.ed.value())
	if d.validate != nil {
		if err := d.validate(v); err != nil {
			d.err = err.Error()
			return nil
		}
	}
	m.dlg = nil
	return d.onOK(m, v)
}

func (d *editDialog) press(m *model, i int) tea.Cmd {
	if i == 0 {
		return d.submit(m)
	}
	return d.cancel(m)
}

func (d *editDialog) update(m *model, msg tea.Msg) tea.Cmd {
	if p, ok := msg.(tea.PasteMsg); ok {
		if d.fb == -1 {
			d.ed.insert(p.Content)
			d.err = ""
		}
		return nil
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch s := k.String(); s {
	case "esc":
		return d.cancel(m)
	case "tab":
		d.fb++
		if d.fb > 1 {
			d.fb = -1
		}
		return nil
	case "shift+tab":
		d.fb--
		if d.fb < -1 {
			d.fb = 1
		}
		return nil
	case "ctrl+s":
		if d.multi {
			return d.submit(m)
		}
	}
	if d.fb >= 0 {
		switch k.String() {
		case "left", "h":
			d.fb = 0
		case "right", "l":
			d.fb = 1
		case "enter":
			return d.press(m, d.fb)
		case "up", "k":
			d.fb = -1
		}
		return nil
	}
	if !d.multi && k.String() == "enter" {
		return d.submit(m)
	}
	if d.ed.key(k, d.edWidth()) {
		d.err = ""
	}
	return nil
}

func (d *editDialog) view(m *model) dview {
	inner := d.w - 2
	text := inner - dpad - 2
	pad := strings.Repeat(" ", dpad)
	var lines []string
	add := func(s string) { lines = append(lines, pad+s) }
	add("")
	for _, l := range wrapPlain(d.prompt, text) {
		add(sBold.Render(l))
	}
	add("")
	h, bar := 1, "› "
	if d.multi {
		h, bar = 7, "│ "
	}
	for _, l := range d.ed.view(d.edWidth(), h, d.fb == -1) {
		add(sDim.Render(bar) + l)
	}
	add("")
	hint := kc("Enter") + " OK   " + kc("Tab") + " buttons   " + kc("Esc") + " cancel"
	if d.multi {
		hint = kc("Ctrl+S") + " save   " + kc("Enter") + " new line   " + kc("Tab") + " buttons"
	}
	if d.hint != "" {
		add(sDim.Render(d.hint))
	}
	add(hint)
	if d.err != "" {
		add(sRed.Render("✗ " + d.err))
	}
	add("")
	fns := []func(m *model) tea.Cmd{
		func(m *model) tea.Cmd { return d.press(m, 0) },
		func(m *model) tea.Cmd { return d.press(m, 1) },
	}
	row, hits := buttonRow([]string{"OK", "Cancel"}, d.fb, inner, len(lines), fns)
	lines = append(lines, row, "")
	return frame(d.title, d.w, lines, hits)
}

// ============================================================ date & time dialog

// dtDialog is a calendar plus a digital clock. Minutes go 00, 15, 30, 45.
type dtDialog struct {
	title, sub string
	day        time.Time
	h12, min   int
	pm         bool
	fb         int // 0 calendar, 1 hour, 2 minute, 3 AM/PM, 4 OK, 5 Cancel
	err        string
	w          int
	validate   func(t time.Time) error
	onOK       func(m *model, t time.Time) tea.Cmd
}

func newDT(m *model, title, sub string, init time.Time, validate func(time.Time) error, onOK func(m *model, t time.Time) tea.Cmd) *dtDialog {
	if init.IsZero() {
		init = time.Now()
	}
	base := time.Date(init.Year(), init.Month(), init.Day(), init.Hour(), 0, 0, 0, init.Location())
	t := base.Add(time.Duration(((init.Minute()+7)/15)*15) * time.Minute)
	h := t.Hour()
	d := &dtDialog{title: title, sub: sub, day: dayStart(t), min: t.Minute(), pm: h >= 12, w: dlgWidth(m, 70), validate: validate, onOK: onOK}
	d.h12 = h % 12
	if d.h12 == 0 {
		d.h12 = 12
	}
	return d
}

func (d *dtDialog) value() time.Time {
	h := d.h12 % 12
	if d.pm {
		h += 12
	}
	return time.Date(d.day.Year(), d.day.Month(), d.day.Day(), h, d.min, 0, 0, d.day.Location())
}

func addMonths(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, t.Location())
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
	return time.Date(first.Year(), first.Month(), minInt(t.Day(), last), 0, 0, 0, 0, t.Location())
}

func (d *dtDialog) hour(delta int) {
	if delta > 0 {
		d.h12 = d.h12%12 + 1
	} else {
		d.h12 = (d.h12+10)%12 + 1
	}
}

func (d *dtDialog) minute(delta int) {
	if delta > 0 {
		d.min = (d.min + 15) % 60
	} else {
		d.min = (d.min + 45) % 60
	}
}

func (d *dtDialog) adjust(field, delta int) {
	switch field {
	case 1:
		d.hour(delta)
	case 2:
		d.minute(delta)
	case 3:
		d.pm = !d.pm
	}
	d.err = ""
}

func (d *dtDialog) cancel(m *model) tea.Cmd {
	m.dlg = nil
	m.say("Cancelled.", false)
	return nil
}

func (d *dtDialog) submit(m *model) tea.Cmd {
	t := d.value()
	if d.validate != nil {
		if err := d.validate(t); err != nil {
			d.err = err.Error()
			return nil
		}
	}
	m.dlg = nil
	return d.onOK(m, t)
}

func (d *dtDialog) update(m *model, msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	s := k.String()
	switch s {
	case "esc":
		return d.cancel(m)
	case "tab":
		d.fb = (d.fb + 1) % 6
		return nil
	case "shift+tab":
		d.fb = (d.fb + 5) % 6
		return nil
	case "enter":
		if d.fb == 5 {
			return d.cancel(m)
		}
		return d.submit(m)
	}
	switch {
	case d.fb == 0:
		d.err = ""
		switch s {
		case "left", "h":
			d.day = d.day.AddDate(0, 0, -1)
		case "right", "l":
			d.day = d.day.AddDate(0, 0, 1)
		case "up", "k":
			d.day = d.day.AddDate(0, 0, -7)
		case "down", "j":
			d.day = d.day.AddDate(0, 0, 7)
		case "pgup":
			d.day = addMonths(d.day, -1)
		case "pgdown":
			d.day = addMonths(d.day, 1)
		case "t":
			d.day = dayStart(time.Now())
		}
	case d.fb >= 1 && d.fb <= 3:
		switch s {
		case "up", "k", "+":
			d.adjust(d.fb, 1)
		case "down", "j", "-":
			d.adjust(d.fb, -1)
		case "space", " ":
			if d.fb == 3 {
				d.adjust(3, 1)
			}
		case "left", "h":
			d.fb = maxInt(d.fb-1, 1)
		case "right", "l":
			d.fb = minInt(d.fb+1, 3)
		}
	default:
		switch s {
		case "left", "h":
			d.fb = 4
		case "right", "l":
			d.fb = 5
		case "up", "k":
			d.fb = 0
		}
	}
	return nil
}

func (d *dtDialog) view(m *model) dview {
	inner := d.w - 2
	pad := strings.Repeat(" ", dpad)
	var lines []string
	var hits []hitRel
	add := func(s string) { lines = append(lines, pad+s) }
	add("")
	add(sBold.Render(d.sub))
	add("")

	// ---- left block: calendar
	const leftW = 30
	calStart := len(lines)
	head := fmt.Sprintf("%s %d", d.day.Month(), d.day.Year())
	hl := sBold
	if d.fb == 0 {
		hl = sYelB
	}
	left := []string{
		hl.Render("◄ ") + hl.Render(fit(strings.Repeat(" ", maxInt((16-len(head))/2, 0))+head, 16)) + hl.Render(" ►"),
		"",
		sDim.Render("Mo Tu We Th Fr Sa Su"),
	}
	first := time.Date(d.day.Year(), d.day.Month(), 1, 0, 0, 0, 0, d.day.Location())
	offset := (int(first.Weekday()) + 6) % 7
	daysIn := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, first.Location()).Day()
	today := dayStart(time.Now())
	for r := 0; r < 6; r++ {
		var cells []string
		for c := 0; c < 7; c++ {
			n := r*7 + c - offset + 1
			if n < 1 || n > daysIn {
				cells = append(cells, "  ")
				continue
			}
			day := time.Date(first.Year(), first.Month(), n, 0, 0, 0, 0, first.Location())
			txt := fmt.Sprintf("%2d", n)
			switch {
			case day.Equal(d.day) && d.fb == 0:
				txt = sBar.Render(txt)
			case day.Equal(d.day):
				txt = sCyanB.Render(txt)
			case day.Equal(today):
				txt = sYelB.Underline(true).Render(txt)
			case c >= 5:
				txt = sRed.Render(txt)
			}
			cells = append(cells, txt)
			hits = append(hits, hitRel{x: dpad + c*3, y: calStart + 3 + r, w: 2, h: 1, fn: func(m *model) tea.Cmd {
				d.day = time.Date(first.Year(), first.Month(), n, 0, 0, 0, 0, first.Location())
				d.fb, d.err = 0, ""
				return nil
			}})
		}
		left = append(left, strings.Join(cells, " "))
	}
	hits = append(hits,
		hitRel{x: dpad, y: calStart, w: 2, h: 1, fn: func(m *model) tea.Cmd { d.day = addMonths(d.day, -1); d.fb = 0; return nil }},
		hitRel{x: dpad + 18, y: calStart, w: 2, h: 1, fn: func(m *model) tea.Cmd { d.day = addMonths(d.day, 1); d.fb = 0; return nil }})

	// ---- right block: digital clock
	box7 := func(val string, focused bool) (string, string, string) {
		st := sPlain
		if focused {
			st = sYelB
		}
		return st.Render("┌─────┐"), st.Render("│") + sBold.Render(fit("  "+val, 5)) + st.Render("│"), st.Render("└─────┘")
	}
	ampm := "AM"
	if d.pm {
		ampm = "PM"
	}
	h1, h2, h3 := box7(fmt.Sprintf("%02d", d.h12), d.fb == 1)
	m1, m2, m3 := box7(fmt.Sprintf("%02d", d.min), d.fb == 2)
	a1, a2, a3 := box7(ampm, d.fb == 3)
	arrow := func(focused bool, ch string) string {
		if focused {
			return sYelB.Render(ch)
		}
		return ch
	}
	right := []string{
		sBold.Render("Time"),
		"",
		"   " + arrow(d.fb == 1, "▲") + "       " + arrow(d.fb == 2, "▲") + "        " + arrow(d.fb == 3, "▲"),
		h1 + " " + m1 + "  " + a1,
		h2 + sBold.Render(":") + m2 + "  " + a2,
		h3 + " " + m3 + "  " + a3,
		"   " + arrow(d.fb == 1, "▼") + "       " + arrow(d.fb == 2, "▼") + "        " + arrow(d.fb == 3, "▼"),
		"",
		sDim.Render("Minutes: 00 · 15 · 30 · 45"),
	}
	rx := dpad + leftW
	for f, bx := range []int{0, 8, 17} {
		f := f + 1
		hits = append(hits,
			hitRel{x: rx + bx + 2, y: calStart + 2, w: 3, h: 1, fn: func(m *model) tea.Cmd { d.fb = f; d.adjust(f, 1); return nil }},
			hitRel{x: rx + bx + 2, y: calStart + 6, w: 3, h: 1, fn: func(m *model) tea.Cmd { d.fb = f; d.adjust(f, -1); return nil }},
			hitRel{x: rx + bx, y: calStart + 3, w: 7, h: 3, fn: func(m *model) tea.Cmd { d.fb = f; return nil }})
	}
	for i := 0; i < len(left); i++ {
		r := ""
		if i < len(right) {
			r = right[i]
		}
		add(fit(left[i], leftW) + r)
	}
	add("")
	add(sDim.Render("Selected  ") + sBold.Render(d.value().Format("Mon 02 Jan 2006 · 03:04 PM")))
	add("")
	add(kc("Tab") + " switch  " + kc("←↑↓→") + " day  " + kc("PgUp/PgDn") + " month  " + kc("↑↓") + " time")
	if d.err != "" {
		add(sRed.Render("✗ " + d.err))
	} else {
		add("")
	}
	add("")
	fns := []func(m *model) tea.Cmd{
		func(m *model) tea.Cmd { return d.submit(m) },
		func(m *model) tea.Cmd { return d.cancel(m) },
	}
	bf := -1
	if d.fb >= 4 {
		bf = d.fb - 4
	}
	row, bh := buttonRow([]string{"OK", "Cancel"}, bf, inner, len(lines), fns)
	lines = append(lines, row, "")
	hits = append(hits, bh...)
	return frame(d.title, d.w, lines, hits)
}

// ============================================================ upcoming dialog

type arow struct {
	hdr  bool
	text string
	idx  int
}

// agendaDialog lists start / due times grouped by day.
type agendaDialog struct {
	title  string
	items  []agendaItem
	rows   []arow
	cur    int
	off    int
	fb     int // -1 = list, 0 = Show task, 1 = Close
	w      int
	onShow func(m *model, it agendaItem) tea.Cmd
}

const agendaRows = 13

func dayLabel(t time.Time) string {
	today := dayStart(time.Now())
	switch {
	case dayStart(t).Equal(today):
		return t.Format("Today · Mon 02 Jan")
	case dayStart(t).Equal(today.AddDate(0, 0, 1)):
		return t.Format("Tomorrow · Mon 02 Jan")
	}
	return t.Format("Mon 02 Jan")
}

func newAgenda(m *model, items []agendaItem, selectPath string, selectKind string) *agendaDialog {
	d := &agendaDialog{title: "Upcoming · start / due", items: items, fb: -1, w: dlgWidth(m, 76)}
	last := ""
	for i, it := range items {
		day := it.at.Format("2006-01-02")
		if day != last {
			d.rows = append(d.rows, arow{hdr: true, text: dayLabel(it.at)})
			last = day
		}
		d.rows = append(d.rows, arow{idx: i})
		if selectPath != "" && it.path == selectPath && (selectKind == "" || it.kind == selectKind) && d.cur == 0 {
			d.cur = i
		}
	}
	return d
}

func (d *agendaDialog) rowOf(idx int) int {
	for i, r := range d.rows {
		if !r.hdr && r.idx == idx {
			return i
		}
	}
	return 0
}

func (d *agendaDialog) show(m *model) tea.Cmd {
	if len(d.items) == 0 {
		return d.close(m)
	}
	m.dlg = nil
	return d.onShow(m, d.items[d.cur])
}

func (d *agendaDialog) close(m *model) tea.Cmd {
	m.dlg = nil
	return nil
}

func (d *agendaDialog) update(m *model, msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	n := len(d.items)
	switch k.String() {
	case "esc", "q":
		return d.close(m)
	case "tab":
		d.fb++
		if d.fb > 1 {
			d.fb = -1
		}
	case "shift+tab":
		d.fb--
		if d.fb < -1 {
			d.fb = 1
		}
	case "up", "k":
		if d.fb >= 0 {
			d.fb = -1
		} else if n > 0 {
			d.cur = maxInt(d.cur-1, 0)
		}
	case "down", "j":
		if d.fb == -1 && n > 0 {
			d.cur = minInt(d.cur+1, n-1)
		}
	case "pgup":
		d.cur = maxInt(d.cur-8, 0)
	case "pgdown":
		d.cur = clamp(d.cur+8, 0, maxInt(n-1, 0))
	case "home":
		d.cur = 0
	case "end":
		d.cur = maxInt(n-1, 0)
	case "left", "h":
		if d.fb >= 0 {
			d.fb = 0
		}
	case "right", "l":
		if d.fb >= 0 {
			d.fb = 1
		}
	case "enter":
		if d.fb == 1 {
			return d.close(m)
		}
		return d.show(m)
	}
	return nil
}

func (d *agendaDialog) view(m *model) dview {
	inner := d.w - 2
	text := inner - dpad - 2
	pad := strings.Repeat(" ", dpad)
	var lines []string
	var hits []hitRel
	add := func(s string) { lines = append(lines, pad+s) }
	add("")
	count := ""
	if len(d.items) > 0 {
		count = fmt.Sprintf("   (%d/%d)", d.cur+1, len(d.items))
	}
	add(sDim.Render("Next 30 days.  ") + sGreen.Render("▶") + sDim.Render(" starts  ") + sYel.Render("■") + sDim.Render(" due") + sDim.Render(count))
	add("")
	if len(d.items) == 0 {
		add(sDim.Render("Nothing starts or is due in the next 30 days."))
	}
	cr := d.rowOf(d.cur)
	d.off = windowStart(cr, d.off, agendaRows)
	for i := d.off; i < minInt(len(d.rows), d.off+agendaRows); i++ {
		r := d.rows[i]
		if r.hdr {
			add(sYelB.Underline(true).Render(r.text))
			continue
		}
		it := d.items[r.idx]
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
		sel := r.idx == d.cur
		mk := "  "
		if sel {
			mk = "> "
		}
		right := it.label
		leftPlain := mk + it.at.Format("15:04") + "  " + glyph + " " + tag + "  " + title
		avail := text - ansi.StringWidth(right) - 1
		var row string
		if sel && d.fb == -1 {
			l := ansi.Truncate(leftPlain, maxInt(avail, 1), "…")
			row = sBar.Render(fit(l+strings.Repeat(" ", maxInt(avail-ansi.StringWidth(l), 0)+1)+right, text))
		} else {
			tst := sPlain
			if it.overdue {
				tst = sRed
			}
			l := mk + sDim.Render(it.at.Format("15:04")) + "  " + gst.Render(glyph) + " " + tst.Render(tag) + "  " + tst.Render(title)
			if sel {
				l = sBold.Render(mk) + sDim.Render(it.at.Format("15:04")) + "  " + gst.Render(glyph) + " " + sBold.Render(tag+"  "+title)
			}
			l = ansi.Truncate(l, maxInt(avail, 1), "…")
			row = l + strings.Repeat(" ", maxInt(avail-ansi.StringWidth(l), 0)+1) + sDim.Render(right)
		}
		idx := r.idx
		hits = append(hits, hitRel{x: dpad, y: len(lines), w: text, h: 1, fn: func(m *model) tea.Cmd {
			if d.cur == idx && d.fb == -1 {
				return d.show(m) // second click opens the task
			}
			d.cur, d.fb = idx, -1
			return nil
		}})
		add(row)
	}
	add("")
	add(kc("↑↓") + " move   " + kc("Enter") + " show task   " + kc("Esc") + " close")
	add("")
	fns := []func(m *model) tea.Cmd{
		func(m *model) tea.Cmd { return d.show(m) },
		func(m *model) tea.Cmd { return d.close(m) },
	}
	row, bh := buttonRow([]string{"Show task", "Close"}, d.fb, inner, len(lines), fns)
	lines = append(lines, row, "")
	hits = append(hits, bh...)
	return frame(d.title, d.w, lines, hits)
}

// ============================================================ help dialog

type helpDialog struct{ w int }

func (d *helpDialog) update(m *model, msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc", "enter", "f9", "q", "space", " ":
			m.dlg = nil
		}
	}
	return nil
}

func (d *helpDialog) view(m *model) dview {
	pad := strings.Repeat(" ", dpad)
	var lines []string
	add := func(s string) { lines = append(lines, pad+s) }
	kv := func(k, v string) { add(sYelB.Render(fit(k, 18)) + v) }
	two := func(k1, v1, k2, v2 string) {
		add(sYelB.Render(fit(k1, 5)) + fit(v1, 22) + sYelB.Render(fit(k2, 5)) + v2)
	}
	head := func(t string) { add(sYelB.Underline(true).Render(t)) }
	add("")
	head("Move")
	kv("Tab / Shift+Tab", "Next / previous panel")
	kv("↑ ↓", "Move the cursor")
	kv("← →", "Collapse / expand a task")
	kv("Enter / click", "Open · edit a field · expand")
	kv("PgUp / PgDn", "Scroll details / work log")
	kv("Esc", "Close a box")
	add("")
	head("Function keys")
	two("F1", "Add label / task", "F6", "Delete label")
	two("F2", "Add sub-task", "F7", "Upcoming")
	two("F3", "Add work log", "F8", "Reload")
	two("F4", "Mark done", "F9", "This help")
	two("F5", "Mark canceled", "F10", "Quit (also Esc, q)")
	add("")
	add(sDim.Render("Task Counts is display only (no focus)."))
	add("")
	row, hits := buttonRow([]string{"Close"}, 0, d.w-2, len(lines), []func(m *model) tea.Cmd{
		func(m *model) tea.Cmd { m.dlg = nil; return nil },
	})
	lines = append(lines, row, "")
	return frame("Help · F9", d.w, lines, hits)
}
