package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// mdLine is one rendered line of details.md.
type mdLine struct {
	plain string // text without colours (used for the highlight bar)
	text  string // coloured text
	field string // editable field this line belongs to ("" = read only)
}

// renderMD renders details.md the way a markdown viewer would, but without
// the leading # of headings: the title is a bar, "Description" is underlined,
// work log headings are cyan, "**bold**" labels are bold, "- " are bullets.
func renderMD(md string, w int) []mdLine {
	var out []mdLine
	section := ""
	add := func(plain, text, field string) { out = append(out, mdLine{plain: plain, text: text, field: field}) }
	for _, ln := range strings.Split(strings.TrimRight(md, "\n"), "\n") {
		switch {
		case strings.HasPrefix(ln, "# "):
			section = "title"
			t := strings.TrimPrefix(strings.TrimPrefix(ln, "# "), "Title: ")
			for _, p := range wrapPlain(t, w-2) {
				add(" "+p+" ", sH1.Render(" "+p+" "), "title")
			}
		case strings.HasPrefix(ln, "### "):
			section = "worklog"
			t := strings.TrimPrefix(ln, "### ")
			add(t, sCyanB.Render(t), "")
		case strings.HasPrefix(ln, "## "):
			t := strings.TrimPrefix(ln, "## ")
			field := ""
			switch strings.ToLower(t) {
			case "description":
				section, field = "description", "description"
			case "alerts":
				section, field = "alerts", "alerts"
			default:
				section = "history"
			}
			add(t, sYelB.Underline(true).Render(t), field)
		case ln == "===":
			section = "after"
			add(ln, sDim.Render(ln), "")
		case ln == "---":
			r := strings.Repeat("─", maxInt(w-2, 4))
			add(r, sDim.Render(r), "")
		case strings.TrimSpace(ln) == "":
			add("", "", "")
		case strings.HasPrefix(ln, "- ") && section != "description" && section != "worklog":
			out = append(out, mdBullet(strings.TrimPrefix(ln, "- "), section))
		default:
			field := ""
			if section == "description" {
				field = "description"
			}
			for _, p := range wrapPlain(ln, w) {
				add(p, p, field)
			}
		}
	}
	return out
}

func mdBullet(body, section string) mdLine {
	label, rest := "", body
	if strings.HasPrefix(body, "**") {
		if j := strings.Index(body[2:], "**"); j >= 0 {
			label, rest = body[2:2+j], body[2+j+2:]
		}
	}
	field := ""
	switch section {
	case "alerts":
		field = "alerts"
	case "history":
	default:
		switch label {
		case "Start Time:":
			field = "start_time"
		case "Due Time:":
			field = "due_time"
		case "Priority:":
			field = "priority"
		case "Status:":
			field = "status"
		case "Repeat:":
			field = "recurrence"
		}
	}
	plain := "• " + label + rest
	text := "• " + sBold.Render(label) + rest
	if label == "" {
		text = "• " + rest
	}
	return mdLine{plain: plain, text: text, field: field}
}

// fieldStops lists the first line of every editable field, in order.
func fieldStops(lines []mdLine) []int {
	seen := map[string]bool{}
	var stops []int
	for i, l := range lines {
		if l.field != "" && !seen[l.field] {
			seen[l.field] = true
			stops = append(stops, i)
		}
	}
	return stops
}

var _ = lipgloss.NewStyle
