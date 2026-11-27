package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Only the terminal's own 16 ANSI colours are used, so the TUI follows the
// colour theme of the terminal.
var (
	cYellow color.Color = lipgloss.Color("3")
	cWhite  color.Color = lipgloss.Color("7")
	cGrey   color.Color = lipgloss.Color("8")
	cRed    color.Color = lipgloss.Color("1")
	cGreen  color.Color = lipgloss.Color("2")
	cCyan   color.Color = lipgloss.Color("6")
	cBlue   color.Color = lipgloss.Color("4")
	cBlack  color.Color = lipgloss.Color("0")
	cBright color.Color = lipgloss.Color("15")
)

func fgc(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

var (
	sPlain = lipgloss.NewStyle()
	sBold  = lipgloss.NewStyle().Bold(true)
	sDim   = fgc(cGrey)
	sYel   = fgc(cYellow)
	sYelB  = fgc(cYellow).Bold(true)
	sCyan  = fgc(cCyan)
	sCyanB = fgc(cCyan).Bold(true)
	sRed   = fgc(cRed)
	sRedB  = fgc(cRed).Bold(true)
	sGreen = fgc(cGreen)
	sBar   = lipgloss.NewStyle().Foreground(cBlack).Background(cCyan).Bold(true) // highlight bar
	sTitle = lipgloss.NewStyle().Reverse(true)                                   // title bar
	sH1    = lipgloss.NewStyle().Foreground(cBright).Background(cBlue).Bold(true)
	sKey   = lipgloss.NewStyle().Foreground(cBlack).Background(cWhite) // key caps in hints
	sBtnOn = lipgloss.NewStyle().Foreground(cBlack).Background(cYellow).Bold(true)
)

// fit cuts or pads s to exactly w columns.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sbar describes a scrollbar drawn in the right border (▲ ░ █ ▼).
type sbar struct{ total, visible, offset int }

// box draws a retro panel: double border when focused, single otherwise,
// title in the top border, optional footer in the bottom border.
func box(title, footer string, lines []string, w, h int, focused, passive bool, sb *sbar) []string {
	bc := cWhite
	switch {
	case focused:
		bc = cYellow
	case passive:
		bc = cGrey
	}
	bs := fgc(bc)
	ts := sPlain
	if focused {
		ts = sYelB
	} else if passive {
		ts = sDim
	}
	tl, hz, tr, vt, bl, br, lt, rt := "┌", "─", "┐", "│", "└", "┘", "┤", "├"
	if focused {
		tl, hz, tr, vt, bl, br, lt, rt = "╔", "═", "╗", "║", "╚", "╝", "╡", "╞"
	}
	iw, ih := w-2, h-2
	seg := func(text string, avail int) string {
		if text == "" || avail < 6 {
			return ""
		}
		text = ansi.Truncate(text, avail-4, "…")
		return bs.Render(lt) + ts.Render(" "+text+" ") + bs.Render(rt)
	}
	t := seg(title, iw-1)
	top := bs.Render(tl+hz) + t + bs.Render(strings.Repeat(hz, maxInt(w-3-ansi.StringWidth(t), 0))+tr)
	f := seg(footer, iw-1)
	bottom := bs.Render(bl+strings.Repeat(hz, maxInt(w-3-ansi.StringWidth(f), 0))) + f + bs.Render(hz+br)

	right := make([]string, ih)
	for j := range right {
		right[j] = bs.Render(vt)
	}
	if sb != nil && sb.total > sb.visible && ih >= 4 {
		ac := cWhite
		if focused {
			ac = cYellow
		}
		track := ih - 2
		thumb := maxInt(1, track*sb.visible/sb.total)
		maxOff := sb.total - sb.visible
		top0 := 0
		if maxOff > 0 {
			top0 = (track - thumb) * clamp(sb.offset, 0, maxOff) / maxOff
		}
		right[0] = fgc(ac).Render("▲")
		right[ih-1] = fgc(ac).Render("▼")
		for j := 0; j < track; j++ {
			if j >= top0 && j < top0+thumb {
				right[1+j] = fgc(ac).Render("█")
			} else {
				right[1+j] = sDim.Render("░")
			}
		}
	}
	out := make([]string, 0, h)
	out = append(out, top)
	for j := 0; j < ih; j++ {
		line := ""
		if j < len(lines) {
			line = lines[j]
		}
		out = append(out, bs.Render(vt)+fit(line, iw)+"\x1b[0m"+right[j])
	}
	out = append(out, bottom)
	return out
}

// seg is a piece of styled text.
type seg struct {
	s  string
	st lipgloss.Style
}

func plain(segs []seg) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.s)
	}
	return b.String()
}

func styled(segs []seg) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.st.Render(s.s))
	}
	return b.String()
}

// listRow draws one row of a list: "> text ........ right". A selected row in
// a focused panel becomes a full-width highlight bar.
func listRow(w int, marker string, segs []seg, right string, rightSt lipgloss.Style, selected, focused bool) string {
	avail := w - 2
	if right != "" {
		avail -= ansi.StringWidth(right) + 1
	}
	left := plain(segs)
	if selected && focused {
		left = ansi.Truncate(left, maxInt(avail, 1), "…")
		pad := strings.Repeat(" ", maxInt(avail-ansi.StringWidth(left), 0)+boolInt(right != ""))
		return sBar.Render(fit(marker+" "+left+pad+right, w))
	}
	l := ansi.Truncate(styled(segs), maxInt(avail, 1), "…")
	pad := strings.Repeat(" ", maxInt(avail-ansi.StringWidth(l), 0)+boolInt(right != ""))
	mk := marker + " "
	if selected {
		mk = sYelB.Render(marker) + " "
		var b strings.Builder
		for _, sg := range segs { // keep the colours, just make the selected row bold
			b.WriteString(sg.st.Bold(true).Render(sg.s))
		}
		l = ansi.Truncate(b.String(), maxInt(avail, 1), "…")
		pad = strings.Repeat(" ", maxInt(avail-ansi.StringWidth(l), 0)+boolInt(right != ""))
	}
	return mk + l + pad + rightSt.Render(right)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// wrapPlain word-wraps plain text to w columns.
func wrapPlain(s string, w int) []string {
	if w < 4 {
		w = 4
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range strings.Fields(para) {
			for ansi.StringWidth(word) > w {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				cut := ansi.Truncate(word, w, "")
				out = append(out, cut)
				word = strings.TrimPrefix(word, cut)
			}
			switch {
			case line == "":
				line = word
			case ansi.StringWidth(line)+1+ansi.StringWidth(word) <= w:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		out = append(out, line)
	}
	return out
}

// windowStart keeps a cursor visible inside a window of `height` rows.
func windowStart(cursor, off, height int) int {
	if cursor < off {
		off = cursor
	}
	if cursor >= off+height {
		off = cursor - height + 1
	}
	return maxInt(off, 0)
}
