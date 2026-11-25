package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func isClosed(name string) bool {
	return strings.HasPrefix(name, "DONE_") || strings.HasPrefix(name, "CANCEL_") || strings.HasPrefix(name, "DELETED_")
}
func getDirs(parent string) []item {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	out := []item{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, ".") || isClosed(n) || n == "worklog" {
			continue
		}
		out = append(out, item{name: strings.ReplaceAll(n, "_", " "), path: filepath.Join(parent, n)})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].name) < strings.ToLower(out[j].name) })
	return out
}

// decorateItems reads task metadata once when a list is loaded. View() then
// renders cached values instead of repeatedly touching the filesystem.
func decorateItems(items []item) []item {
	for i := range items {
		p := items[i].path
		items[i].status = readField(p, "status")
		items[i].dueTime = readField(p, "due_time")
		items[i].recurrence = readField(p, "recurrence")
		items[i].childCount = len(getDirs(p))
	}
	return items
}

func readField(dir, name string) string {
	b, _ := os.ReadFile(filepath.Join(dir, ".tmp", name))
	return strings.TrimSpace(string(b))
}
func writeField(dir, name, value string) error {
	p := filepath.Join(dir, ".tmp", name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(value+"\n"), 0644)
}
func sortAlerts(file string) {
	b, e := os.ReadFile(file)
	if e != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	sort.Strings(lines)
	_ = os.WriteFile(file, []byte(strings.Join(lines, "\n")+func() string {
		if len(lines) > 0 {
			return "\n"
		}
		return ""
	}()), 0644)
}
func displayRecurrence(s string) string {
	if s == "" {
		return "None"
	}
	if strings.HasPrefix(s, "1 ") {
		return "Every " + strings.TrimPrefix(s, "1 ")
	}
	return "Every " + s
}
func displayTime(s string) string {
	t, e := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if e != nil {
		return s
	}
	return t.Format("2006-01-02 03:04 PM")
}
func buildDetails(dir string) error {
	tmp := filepath.Join(dir, ".tmp")
	title := readField(dir, "title")
	desc := readField(dir, "description")
	latest := ""
	logs, _ := filepath.Glob(filepath.Join(dir, "worklog", "*.md"))
	sort.Strings(logs)
	if len(logs) > 0 {
		latest = logs[len(logs)-1]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n## Description\n%s\n", title, desc)
	if latest != "" {
		data, _ := os.ReadFile(latest)
		b.WriteString("\n" + string(data) + "\n")
	}
	fmt.Fprintf(&b, "- **Start Time:** %s\n- **Due Time:** %s\n- **Priority:** %s\n- **Status:** %s\n- **Repeat:** %s\n\n## Alerts\n", readField(dir, "start_time"), readField(dir, "due_time"), readField(dir, "priority"), readField(dir, "status"), displayRecurrence(readField(dir, "recurrence")))
	alerts, _ := os.ReadFile(filepath.Join(tmp, "alerts"))
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(alerts)), "\n") {
		if line == "" {
			continue
		}
		p := strings.SplitN(line, "|", 2)
		desc := ""
		if len(p) > 1 {
			desc = p[1]
		}
		fmt.Fprintf(&b, "- **%s** - %s\n", displayTime(p[0]), desc)
		found = true
	}
	if !found {
		b.WriteString("- No alerts configured.\n")
	}
	b.WriteString("\n## Alert History\n")
	hist, _ := os.ReadFile(filepath.Join(tmp, "alert_history"))
	if strings.TrimSpace(string(hist)) == "" {
		b.WriteString("- No alerts sent yet.\n")
	} else {
		for _, line := range strings.Split(strings.TrimSpace(string(hist)), "\n") {
			fmt.Fprintf(&b, "- %s\n", displayTime(line))
		}
	}
	return os.WriteFile(filepath.Join(dir, "details.md"), []byte(b.String()), 0644)
}
func (m *model) loadLabels() {
	m.labels = decorateItems(getDirs(m.workDir))
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
	m.currentParent = m.selectedLabelPath()
	m.flattenOpen(m.currentParent, 0)
	closed := getClosedDirs(m.currentParent)
	if len(closed) > 0 {
		m.tasks = append(m.tasks, item{name: fmt.Sprintf("Completed & Canceled (%d)", len(closed)), group: true, expanded: m.closedExpanded})
		if m.closedExpanded {
			for _, x := range closed {
				x.depth = 0
				m.tasks = append(m.tasks, x)
			}
		}
	}
	if m.selectedTask >= len(m.tasks) {
		m.selectedTask = len(m.tasks) - 1
	}
	if m.selectedTask < 0 {
		m.selectedTask = 0
	}
	m.loadSelectedTask()
}
func (m *model) loadSelectedTask() {
	p := m.selectedPath()
	if p == "" {
		m.details = "No tasks available."
		m.worklogs = nil
		m.logs.SetContent("No work logs available.")
		m.refreshDetails(m.width * 50 / 112)
		return
	}
	if !m.tasks[m.selectedTask].closed {
		_ = buildDetails(p)
	}
	b, e := os.ReadFile(filepath.Join(p, "details.md"))
	if e != nil {
		m.details = "No details.md available."
	} else {
		m.details = string(b)
	}
	m.loadWorklogs(p)
	m.refreshDetails(m.width * 50 / 112)
}
func (m *model) loadWorklogs(dir string) {
	files, _ := filepath.Glob(filepath.Join(dir, "worklog", "*.md"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	m.worklogs = files
	var b strings.Builder
	for _, f := range files {
		data, e := os.ReadFile(f)
		if e == nil {
			b.Write(data)
			b.WriteString("\n\n────────────────────\n\n")
		}
	}
	if b.Len() == 0 {
		b.WriteString("No work logs available.")
	}
	m.logs.SetContent(b.String())
}
func (m model) taskCounts() string {
	var ot, osub, ct, csub int
	_ = filepath.WalkDir(m.workDir, func(path string, e os.DirEntry, err error) error {
		if err != nil || !e.IsDir() || path == m.workDir {
			return nil
		}
		rel, _ := filepath.Rel(m.workDir, path)
		n := e.Name()
		if strings.HasPrefix(n, ".") || n == "worklog" {
			return filepath.SkipDir
		}
		depth := strings.Count(rel, string(os.PathSeparator)) + 1
		if depth < 2 {
			return nil
		}
		if strings.HasPrefix(n, "CANCEL_") || strings.HasPrefix(n, "DELETED_") {
			return filepath.SkipDir
		}
		done := strings.HasPrefix(n, "DONE_")
		switch {
		case depth == 2 && done:
			ct++
		case depth == 2:
			ot++
		case depth >= 3 && done:
			csub++
		case depth >= 3:
			osub++
		}
		return nil
	})
	return fmt.Sprintf("Open\n  Tasks       %d\n  Sub-tasks   %d\n\nCompleted\n  Tasks       %d\n  Sub-tasks   %d", ot, osub, ct, csub)
}

// getClosedDirs returns completed/canceled task folders without making them editable.
func getClosedDirs(parent string) []item {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var out []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		prefix := ""
		if strings.HasPrefix(n, "DONE_") {
			prefix = "DONE"
		} else if strings.HasPrefix(n, "CANCEL_") {
			prefix = "CANCEL"
		} else {
			continue
		}
		out = append(out, item{name: strings.ReplaceAll(strings.TrimPrefix(n, prefix+"_"), "_", " "), path: filepath.Join(parent, n), status: readField(filepath.Join(parent, n), "status"), dueTime: readField(filepath.Join(parent, n), "due_time"), closed: true, closedPrefix: prefix, childCount: len(getDirs(filepath.Join(parent, n)))})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].name) < strings.ToLower(out[j].name) })
	return out
}

func (m *model) flattenOpen(parent string, depth int) {
	for _, x := range decorateItems(getDirs(parent)) {
		x.depth = depth
		x.expanded = m.expanded[x.path]
		m.tasks = append(m.tasks, x)
		if x.expanded {
			m.flattenOpen(x.path, depth+1)
		}
	}
}

func walkEntities(root string, fn func(string)) {
	for _, x := range getDirs(root) {
		fn(x.path)
		walkEntities(x.path, fn)
	}
}

type scheduleItem struct {
	when                     time.Time
	kind, title, label, path string
	subtask                  bool
}

func (m *model) loadToday() {
	m.todayItems = nil
	m.upcomingItems = nil
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	for _, label := range m.labels {
		walkEntities(label.path, func(p string) {
			for _, f := range []struct{ name, kind string }{{"start_time", "Start"}, {"due_time", "Due"}} {
				t, err := parseTaskTime(readField(p, f.name))
				if err != nil {
					continue
				}
				if f.kind == "Due" && t.Before(now) || (!t.Before(start) && t.Before(end)) {
					labelName := filepath.Base(label.path)
					rel, _ := filepath.Rel(label.path, p)
					sub := strings.Contains(rel, string(os.PathSeparator))
					x := scheduleItem{when: t, kind: f.kind, title: strings.ReplaceAll(filepath.Base(p), "_", " "), label: strings.ReplaceAll(labelName, "_", " "), path: p, subtask: sub}
					if !t.Before(start) && t.Before(end) {
						m.todayItems = append(m.todayItems, x)
					}
					if !t.Before(start) && t.Before(start.AddDate(0, 0, 30)) {
						m.upcomingItems = append(m.upcomingItems, x)
					}
				}
			}
		})
	}
	sort.Slice(m.todayItems, func(i, j int) bool { return m.todayItems[i].when.Before(m.todayItems[j].when) })
	sort.Slice(m.upcomingItems, func(i, j int) bool { return m.upcomingItems[i].when.Before(m.upcomingItems[j].when) })
	if m.selectedToday >= len(m.todayItems) {
		m.selectedToday = len(m.todayItems) - 1
	}
	if m.selectedToday < 0 {
		m.selectedToday = 0
	}
}
