package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Use only ANSI 16-colour indices so the TUI follows the user's terminal theme.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	normalStyle   = lipgloss.NewStyle()
	countStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	dangerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	focusedBorder = lipgloss.Color("11")
	normalBorder  = lipgloss.Color("7")
)

// panel draws its own frame so the title is part of the top border, as in
// classic terminal applications. width and height include the frame.
func panel(content string, width, height int, title string, active bool) string {
	if width < 8 {
		width = 8
	}
	if height < 4 {
		height = 4
	}

	left, right, horizontal, vertical := "┌", "┐", "─", "│"
	bottomLeft, bottomRight := "└", "┘"
	borderStyle := lipgloss.NewStyle().Foreground(normalBorder)
	if active {
		left, right, horizontal, vertical = "╔", "╗", "═", "║"
		bottomLeft, bottomRight = "╚", "╝"
		borderStyle = lipgloss.NewStyle().Foreground(focusedBorder)
	}

	innerWidth := width - 2
	label := "┤ " + title + " ├"
	if active {
		label = "╡ " + title + " ╞"
	}
	labelWidth := lipgloss.Width(label)
	if labelWidth > innerWidth-1 {
		label = lipgloss.NewStyle().MaxWidth(innerWidth - 1).Render(label)
		labelWidth = lipgloss.Width(label)
	}
	top := left + horizontal + label + strings.Repeat(horizontal, maxInt(0, innerWidth-1-labelWidth)) + right
	bottom := bottomLeft + strings.Repeat(horizontal, innerWidth) + bottomRight

	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	bodyHeight := height - 2
	if len(lines) > bodyHeight {
		lines = lines[:bodyHeight]
	}
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > innerWidth-2 {
			line = lipgloss.NewStyle().MaxWidth(innerWidth - 2).Render(line)
			lineWidth = lipgloss.Width(line)
		}
		lines[i] = vertical + " " + line + strings.Repeat(" ", maxInt(0, innerWidth-2-lineWidth)) + " " + vertical
	}
	return borderStyle.Render(top) + "\n" + strings.Join(lines, "\n") + "\n" + borderStyle.Render(bottom)
}

func renderList(items []item, selected int) string {
	if len(items) == 0 {
		return "  No items"
	}
	var out strings.Builder
	for i, x := range items {
		line := "  " + x.name
		if i == selected {
			line = "> " + x.name
			out.WriteString(selectedStyle.Render(line))
		} else {
			out.WriteString(line)
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func addScrollbar(content string, width, height int, percent float64) string {
	if width < 2 {
		return content
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	marker := int(percent * float64(maxInt(0, height-1)))
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > width-1 {
			line = lipgloss.NewStyle().MaxWidth(width - 1).Render(line)
			lineWidth = lipgloss.Width(line)
		}
		glyph := "░"
		if i == 0 {
			glyph = "▲"
		} else if i == height-1 {
			glyph = "▼"
		} else if i == marker {
			glyph = "█"
		}
		lines[i] = line + strings.Repeat(" ", maxInt(0, width-1-lineWidth)) + glyph
	}
	return strings.Join(lines, "\n")
}
