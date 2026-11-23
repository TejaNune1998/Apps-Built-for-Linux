package main

import "charm.land/lipgloss/v2"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7AA2F7"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7AA2F7"))
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0CAF5"))
	focusedBorder = lipgloss.Color("#7AA2F7")
	normalBorder  = lipgloss.Color("#565F89")
)

func panel(content string, width, height int, title string, active bool) string {
	if width < 10 {
		width = 10
	}
	if height < 4 {
		height = 4
	}
	border := normalBorder
	if active {
		border = focusedBorder
	}
	return lipgloss.NewStyle().Width(width-2).Height(height-2).Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Render(titleStyle.Render(title) + "\n" + content)
}
func renderList(items []item, selected int) string {
	if len(items) == 0 {
		return "  No items"
	}
	out := ""
	for i, x := range items {
		if i == selected {
			out += selectedStyle.Render("> " + x.name)
		} else {
			out += normalStyle.Render("  " + x.name)
		}
		out += "\n"
	}
	return out
}
