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

// getClosedDirs lists DONE_ / CANCEL_ folders (not DELETED_ ones) inside parent.
func getClosedDirs(parent string) []item {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	out := []item{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() && (strings.HasPrefix(n, "DONE_") || strings.HasPrefix(n, "CANCEL_")) {
			out = append(out, item{name: n, path: filepath.Join(parent, n)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].name) < strings.ToLower(out[j].name) })
	return out
}
