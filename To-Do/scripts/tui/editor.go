package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// editor is a small text editor used by the input and text dialogs.
type editor struct {
	r     []rune
	pos   int
	multi bool
	top   int
}

func newEditor(s string, multi bool) *editor {
	r := []rune(s)
	return &editor{r: r, pos: len(r), multi: multi}
}

func (e *editor) value() string { return string(e.r) }

func runeWidth(r rune) int {
	if w := ansi.StringWidth(string(r)); w > 0 {
		return w
	}
	return 1
}

func (e *editor) insert(s string) {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "  ")
	if !e.multi {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	var rs []rune
	for _, r := range s {
		if r == '\n' || r >= 32 && r != 127 {
			rs = append(rs, r)
		}
	}
	nr := make([]rune, 0, len(e.r)+len(rs))
	nr = append(nr, e.r[:e.pos]...)
	nr = append(nr, rs...)
	nr = append(nr, e.r[e.pos:]...)
	e.r = nr
	e.pos += len(rs)
}

type vline struct {
	s, e int
	hard bool
}

func (e *editor) vlines(w int) []vline {
	w = maxInt(w, 1)
	var out []vline
	s, col := 0, 0
	for i := 0; i < len(e.r); i++ {
		if e.r[i] == '\n' {
			out = append(out, vline{s, i, true})
			s, col = i+1, 0
			continue
		}
		cw := runeWidth(e.r[i])
		if col+cw > w {
			out = append(out, vline{s, i, false})
			s, col = i, 0
		}
		col += cw
	}
	return append(out, vline{s, len(e.r), true})
}

func (e *editor) cursorLine(vl []vline) (int, int) {
	for i, l := range vl {
		if e.pos >= l.s && (e.pos < l.e || (e.pos == l.e && l.hard)) {
			return i, e.pos - l.s
		}
	}
	last := len(vl) - 1
	return last, e.pos - vl[last].s
}

func (e *editor) del(from, to int) {
	if from < 0 {
		from = 0
	}
	if to > len(e.r) {
		to = len(e.r)
	}
	if from >= to {
		return
	}
	e.r = append(append([]rune{}, e.r[:from]...), e.r[to:]...)
	e.pos = from
}

// key handles an editing key; it returns false if the key is not an editing key.
func (e *editor) key(k tea.KeyPressMsg, w int) bool {
	vl := e.vlines(w)
	ci, cc := e.cursorLine(vl)
	switch k.String() {
	case "left":
		if e.pos > 0 {
			e.pos--
		}
	case "right":
		if e.pos < len(e.r) {
			e.pos++
		}
	case "home", "ctrl+a":
		e.pos = vl[ci].s
	case "end", "ctrl+e":
		e.pos = vl[ci].e
	case "up":
		if ci > 0 {
			e.pos = minInt(vl[ci-1].s+cc, vl[ci-1].e)
		}
	case "down":
		if ci < len(vl)-1 {
			e.pos = minInt(vl[ci+1].s+cc, vl[ci+1].e)
		}
	case "backspace":
		e.del(e.pos-1, e.pos)
	case "delete", "ctrl+d":
		e.del(e.pos, e.pos+1)
	case "ctrl+u":
		e.del(vl[ci].s, e.pos)
	case "ctrl+k":
		e.del(e.pos, vl[ci].e)
	case "ctrl+w":
		i := e.pos
		for i > 0 && e.r[i-1] == ' ' {
			i--
		}
		for i > 0 && e.r[i-1] != ' ' && e.r[i-1] != '\n' {
			i--
		}
		e.del(i, e.pos)
	case "enter":
		if !e.multi {
			return false
		}
		e.insert("\n")
	default:
		if k.Text != "" {
			e.insert(k.Text)
			return true
		}
		return false
	}
	return true
}

// view returns h visual lines of width w; the cursor is drawn as a block.
func (e *editor) view(w, h int, cursor bool) []string {
	vl := e.vlines(w)
	ci, cc := e.cursorLine(vl)
	e.top = windowStart(ci, e.top, h)
	out := make([]string, 0, h)
	for i := e.top; i < e.top+h; i++ {
		if i >= len(vl) {
			out = append(out, "")
			continue
		}
		seg := e.r[vl[i].s:vl[i].e]
		if !cursor || i != ci {
			out = append(out, string(seg))
			continue
		}
		left, ch, right := string(seg[:minInt(cc, len(seg))]), " ", ""
		if cc < len(seg) {
			ch, right = string(seg[cc]), string(seg[cc+1:])
		}
		out = append(out, left+sTitle.Render(ch)+right)
	}
	return out
}
