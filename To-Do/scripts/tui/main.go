package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type item struct {
	name string
	path string
}

type model struct {
	workDir       string
	labels        []item
	tasks         []item
	worklogs      []string
	selectedLabel int
	selectedTask  int
	activePanel   int
	currentParent string

	details string
	logs    viewport.Model

	width  int
	height int
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7"))

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7AA2F7"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C0CAF5"))

	focusedBorder = lipgloss.Color("#7AA2F7")
	normalBorder  = lipgloss.Color("#565F89")
)

func initialModel(workDir string) model {
	m := model{
		workDir:     workDir,
		activePanel: 0,
		logs:        viewport.New(),
	}

	m.loadLabels()
	m.loadTasks()

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

// -------------------------
// Data helpers
// -------------------------

func isClosed(name string) bool {
	return strings.HasPrefix(name, "DONE_") ||
		strings.HasPrefix(name, "CANCEL_") ||
		strings.HasPrefix(name, "DELETED_")
}

func getDirs(parent string) []item {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}

	var result []item

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		if strings.HasPrefix(name, ".") ||
			isClosed(name) ||
			name == "worklog" {
			continue
		}

		result = append(result, item{
			name: strings.ReplaceAll(name, "_", " "),
			path: filepath.Join(parent, name),
		})
	}

	return result
}

func (m *model) loadLabels() {
	m.labels = getDirs(m.workDir)

	if m.selectedLabel >= len(m.labels) {
		m.selectedLabel = len(m.labels) - 1
	}

	if m.selectedLabel < 0 {
		m.selectedLabel = 0
	}
}

func (m *model) loadTasks() {
	m.tasks = nil
	m.details = "Select a task to view its details."
	m.worklogs = nil
	m.logs.SetContent("No work logs available.")

	if len(m.labels) == 0 {
		m.currentParent = ""
		return
	}

	if m.currentParent == "" {
		m.currentParent = m.labels[m.selectedLabel].path
	}

	m.tasks = getDirs(m.currentParent)

	if m.selectedTask >= len(m.tasks) {
		m.selectedTask = len(m.tasks) - 1
	}

	if m.selectedTask < 0 {
		m.selectedTask = 0
	}

	m.loadSelectedTask()
}

func (m *model) loadSelectedTask() {
	if len(m.tasks) == 0 {
		m.details = "No tasks available."
		m.worklogs = nil
		m.logs.SetContent("No work logs available.")
		return
	}

	taskPath := m.tasks[m.selectedTask].path

	detailsFile := filepath.Join(taskPath, "details.md")
	data, err := os.ReadFile(detailsFile)

	if err != nil {
		m.details = "No details.md available."
	} else {
		m.details = string(data)
	}

	m.loadWorklogs(taskPath)
}

func (m *model) loadWorklogs(taskPath string) {
	worklogDir := filepath.Join(taskPath, "worklog")

	entries, err := os.ReadDir(worklogDir)
	if err != nil {
		m.worklogs = nil
		m.logs.SetContent("No work logs available.")
		return
	}

	m.worklogs = nil

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			m.worklogs = append(m.worklogs,
				filepath.Join(worklogDir, entry.Name()))
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(m.worklogs)))

	var content strings.Builder

	for _, file := range m.worklogs {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		content.WriteString(string(data))
		content.WriteString("\n\n────────────────────\n\n")
	}

	if content.Len() == 0 {
		content.WriteString("No work logs available.")
	}

	m.logs.SetContent(content.String())
}

// -------------------------
// Task counts
// -------------------------

func (m model) taskCounts() string {
	var openTasks, openSubtasks int
	var completedTasks, completedSubtasks int

	filepath.WalkDir(m.workDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == m.workDir {
			return nil
		}

		rel, err := filepath.Rel(m.workDir, path)
		if err != nil {
			return nil
		}

		name := entry.Name()

		if strings.HasPrefix(name, ".") || name == "worklog" {
			return filepath.SkipDir
		}

		depth := strings.Count(rel, string(os.PathSeparator)) + 1

		if depth < 2 {
			return nil
		}

		if strings.HasPrefix(name, "CANCEL_") ||
			strings.HasPrefix(name, "DELETED_") {
			return filepath.SkipDir
		}

		isCompleted := strings.HasPrefix(name, "DONE_")

		switch {
		case depth == 2 && isCompleted:
			completedTasks++
		case depth == 2:
			openTasks++
		case depth >= 3 && isCompleted:
			completedSubtasks++
		case depth >= 3:
			openSubtasks++
		}

		return nil
	})

	return fmt.Sprintf(
		"Open\n  Tasks       %d\n  Sub-tasks   %d\n\nCompleted\n  Tasks       %d\n  Sub-tasks   %d",
		openTasks, openSubtasks, completedTasks, completedSubtasks,
	)
}

// -------------------------
// Panel helpers
// -------------------------

func panel(content string, width, height int, title string, active bool) string {
	border := normalBorder

	if active {
		border = focusedBorder
	}

	style := lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)

	return style.Render(titleStyle.Render(title) + "\n" + content)
}

func renderList(items []item, selected int, active bool) string {
	if len(items) == 0 {
		return "  No items"
	}

	var output strings.Builder

	for i, entry := range items {
		name := entry.name

		if i == selected {
			output.WriteString(selectedStyle.Render("> " + name))
		} else {
			output.WriteString(normalStyle.Render("  " + name))
		}

		output.WriteString("\n")
	}

	return output.String()
}

func (m model) renderLabels(width, height int) string {
	return panel(
		renderList(m.labels, m.selectedLabel, m.activePanel == 0),
		width, height, "Labels", m.activePanel == 0,
	)
}

func (m model) renderTasks(width, height int) string {
	return panel(
		renderList(m.tasks, m.selectedTask, m.activePanel == 1),
		width, height, "Tasks", m.activePanel == 1,
	)
}

func (m model) renderDetails(width, height int) string {
	content := strings.TrimSpace(m.details)

	return panel(
		content, width, height, "Task Details", m.activePanel == 2,
	)
}

func (m model) renderCounts(width, height int) string {
	return panel(
		m.taskCounts(), width, height, "Task Counts", m.activePanel == 3,
	)
}

func (m model) renderHelp(width, height int) string {
	content := `Tab       Switch panel
↑ / ↓     Navigate
Enter     Select / Open
Backspace Go to parent
PgUp/PgDn Scroll work logs
r         Refresh
q         Quit`

	return panel(content, width, height, "Help", m.activePanel == 4)
}

func (m model) renderWorklogs(width, height int) string {
	return panel(
		m.logs.View(), width, height, "Work Logs", m.activePanel == 5,
	)
}

// -------------------------
// Layout
// -------------------------

func (m model) View() tea.View {
	if m.width < 90 || m.height < 24 {
		return tea.NewView("Please resize terminal to at least 90 columns and 24 rows.")
	}

	topHeight := m.height * 2 / 3
	bottomHeight := m.height - topHeight

	firstWidth := m.width / 3
	secondWidth := m.width / 3
	thirdWidth := m.width - firstWidth - secondWidth

	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderLabels(firstWidth, topHeight),
		m.renderTasks(secondWidth, topHeight),
		m.renderDetails(thirdWidth, topHeight),
	)

	bottom := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderCounts(firstWidth, bottomHeight),
		m.renderHelp(secondWidth, bottomHeight),
		m.renderWorklogs(thirdWidth, bottomHeight),
	)

	view := tea.NewView(
		lipgloss.JoinVertical(lipgloss.Left, top, bottom),
	)

	view.AltScreen = true
	return view
}

// -------------------------
// Navigation
// -------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.logs.SetWidth(m.width/3 - 6)
		m.logs.SetHeight(m.height/3 - 5)

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.activePanel = (m.activePanel + 1) % 6

		case "shift+tab":
			m.activePanel--
			if m.activePanel < 0 {
				m.activePanel = 5
			}

		case "left":
			m.activePanel--
			if m.activePanel < 0 {
				m.activePanel = 5
			}

		case "right":
			m.activePanel = (m.activePanel + 1) % 6

		case "up", "k":
			switch m.activePanel {
			case 0:
				if m.selectedLabel > 0 {
					m.selectedLabel--
					m.currentParent = ""
					m.selectedTask = 0
					m.loadTasks()
				}
			case 1:
				if m.selectedTask > 0 {
					m.selectedTask--
					m.loadSelectedTask()
				}
			case 5:
				var cmd tea.Cmd
				m.logs, cmd = m.logs.Update(msg)
				return m, cmd
			}

		case "down", "j":
			switch m.activePanel {
			case 0:
				if m.selectedLabel < len(m.labels)-1 {
					m.selectedLabel++
					m.currentParent = ""
					m.selectedTask = 0
					m.loadTasks()
				}
			case 1:
				if m.selectedTask < len(m.tasks)-1 {
					m.selectedTask++
					m.loadSelectedTask()
				}
			case 5:
				var cmd tea.Cmd
				m.logs, cmd = m.logs.Update(msg)
				return m, cmd
			}

		case "enter":
			switch m.activePanel {
			case 0:
				if len(m.labels) > 0 {
					m.currentParent = m.labels[m.selectedLabel].path
					m.selectedTask = 0
					m.loadTasks()
					m.activePanel = 1
				}
			case 1:
				if len(m.tasks) > 0 {
					children := getDirs(m.tasks[m.selectedTask].path)

					if len(children) > 0 {
						m.currentParent = m.tasks[m.selectedTask].path
						m.selectedTask = 0
						m.loadTasks()
					}
				}
			}

		case "backspace":
			if m.currentParent != "" && len(m.labels) > 0 {
				labelPath := m.labels[m.selectedLabel].path

				if m.currentParent != labelPath {
					m.currentParent = filepath.Dir(m.currentParent)
					m.selectedTask = 0
					m.loadTasks()
					m.activePanel = 1
				}
			}

		case "r":
			m.loadLabels()
			m.currentParent = ""
			m.selectedTask = 0
			m.loadTasks()

		case "pgdown", "pgup":
			if m.activePanel == 5 {
				var cmd tea.Cmd
				m.logs, cmd = m.logs.Update(msg)
				return m, cmd
			}
		}
	}

	return m, nil
}

// -------------------------
// Main
// -------------------------

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: to-do-tui <task-directory>")
		os.Exit(1)
	}

	workDir := os.Args[1]

	if _, err := os.Stat(workDir); err != nil {
		fmt.Println("Task directory not found:", workDir)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(workDir))

	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
