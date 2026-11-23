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
	fmt.Fprintf(&b, "# Title: %s\n\n## Description\n%s\n", title, desc)
	if latest != "" {
		data, _ := os.ReadFile(latest)
		b.WriteString("\n" + string(data) + "\n")
	}
	fmt.Fprintf(&b, "===\n---\n\n- **Start Time:** %s\n- **Due Time:** %s\n- **Priority:** %s\n- **Status:** %s\n- **Repeat:** %s\n\n## Alerts\n", readField(dir, "start_time"), readField(dir, "due_time"), readField(dir, "priority"), readField(dir, "status"), displayRecurrence(readField(dir, "recurrence")))
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
		m.currentParent = m.selectedLabelPath()
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
	p := m.selectedPath()
	if p == "" {
		m.details = "No tasks available."
		m.worklogs = nil
		m.logs.SetContent("No work logs available.")
		return
	}
	_ = buildDetails(p)
	b, e := os.ReadFile(filepath.Join(p, "details.md"))
	if e != nil {
		m.details = "No details.md available."
	} else {
		m.details = string(b)
	}
	m.loadWorklogs(p)
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
