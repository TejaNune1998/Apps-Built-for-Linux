package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func createEntity(parent, title, description, start, due, priority, status, repeat string, alerts []string) error {
	if !cleanName(title) {
		return fmt.Errorf("invalid name")
	}
	dir := filepath.Join(parent, folderName(title))
	if _, e := os.Stat(dir); e == nil {
		return fmt.Errorf("already exists")
	}
	if e := os.MkdirAll(filepath.Join(dir, ".tmp"), 0755); e != nil {
		return e
	}
	for k, v := range map[string]string{"title": title, "description": description, "start_time": start, "due_time": due, "priority": priority, "status": status, "recurrence": repeat} {
		if e := writeField(dir, k, v); e != nil {
			return e
		}
	}
	for _, f := range []string{"alerts", "alerts_template", "alert_history"} {
		_ = os.WriteFile(filepath.Join(dir, ".tmp", f), nil, 0644)
	}
	sort.Strings(alerts)
	data := strings.Join(alerts, "\n")
	if len(alerts) > 0 {
		data += "\n"
	}
	_ = os.WriteFile(filepath.Join(dir, ".tmp", "alerts"), []byte(data), 0644)
	_ = os.WriteFile(filepath.Join(dir, ".tmp", "alerts_template"), []byte(data), 0644)
	return buildDetails(dir)
}
func addWorkLog(dir, description string) error {
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("description cannot be empty")
	}
	wd := filepath.Join(dir, "worklog")
	if e := os.MkdirAll(wd, 0755); e != nil {
		return e
	}
	now := time.Now()
	stamp := now.Format("2006-01-02_15-04-05")
	file := filepath.Join(wd, stamp+".md")
	for i := 1; ; i++ {
		if _, e := os.Stat(file); os.IsNotExist(e) {
			break
		}
		file = filepath.Join(wd, fmt.Sprintf("%s_%d.md", stamp, i))
	}
	content := fmt.Sprintf("### Work Log — %s\n\n%s\n", now.Format("2006-01-02 15:04"), description)
	if e := os.WriteFile(file, []byte(content), 0644); e != nil {
		return e
	}
	return buildDetails(dir)
}
func closeTree(dir, prefix, status string) error {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "worklog" || isClosed(entry.Name()) {
			continue
		}
		if e := closeTree(filepath.Join(dir, entry.Name()), prefix, status); e != nil {
			return e
		}
	}
	_ = writeField(dir, "status", status)
	_ = buildDetails(dir)
	target := prefix + "_" + filepath.Base(dir)
	dest := filepath.Join(filepath.Dir(dir), target)
	if _, e := os.Stat(dest); e == nil {
		dest += fmt.Sprintf("_%d", time.Now().UnixNano())
	}
	return os.Rename(dir, dest)
}
func closeEntity(dir, prefix, status string, addLog, description string) error {
	if addLog == "Yes" {
		if e := addWorkLog(dir, description); e != nil {
			return e
		}
	}
	repeat := readField(dir, "recurrence")
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)
	var snap string
	if prefix == "DONE" && repeat != "" {
		snap = filepath.Join(os.TempDir(), fmt.Sprintf("todo-snapshot-%d", time.Now().UnixNano()))
		if e := copyDir(filepath.Join(dir, ".tmp"), snap); e != nil {
			return e
		}
	}
	if e := closeTree(dir, prefix, status); e != nil {
		return e
	}
	if snap != "" {
		defer os.RemoveAll(snap)
		return nextOccurrence(snap, filepath.Join(parent, base))
	}
	return nil
}
func copyDir(src, dst string) error {
	entries, e := os.ReadDir(src)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(dst, 0755); e != nil {
		return e
	}
	for _, x := range entries {
		a, b := filepath.Join(src, x.Name()), filepath.Join(dst, x.Name())
		if x.IsDir() {
			if e = copyDir(a, b); e != nil {
				return e
			}
		} else {
			data, er := os.ReadFile(a)
			if er != nil {
				return er
			}
			if er = os.WriteFile(b, data, 0644); er != nil {
				return er
			}
		}
	}
	return nil
}
func deleteLabel(dir string) error {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	for _, x := range entries {
		if x.IsDir() && !strings.HasPrefix(x.Name(), ".") && !isClosed(x.Name()) && x.Name() != "worklog" {
			if e = closeTree(filepath.Join(dir, x.Name()), "CANCEL", "Canceled"); e != nil {
				return e
			}
		}
	}
	dest := filepath.Join(filepath.Dir(dir), "DELETED_"+filepath.Base(dir))
	if _, e = os.Stat(dest); e == nil {
		dest += fmt.Sprintf("_%d", time.Now().UnixNano())
	}
	return os.Rename(dir, dest)
}
func editWorkLog(file, description string) error {
	old, e := os.ReadFile(file)
	if e != nil {
		return e
	}
	lines := strings.SplitN(string(old), "\n", 3)
	header := strings.TrimSpace(lines[0])
	return os.WriteFile(file, []byte(header+"\n\n"+description+"\n"), 0644)
}
func saveEntityFields(dir string, fields map[string]string) error {
	oldTitle := readField(dir, "title")
	newTitle := strings.TrimSpace(fields["title"])
	if newTitle == "" {
		newTitle = oldTitle
	}
	if !cleanName(newTitle) {
		return fmt.Errorf("invalid title")
	}
	dest := filepath.Join(filepath.Dir(dir), folderName(newTitle))
	if dest != dir {
		if _, e := os.Stat(dest); e == nil {
			return fmt.Errorf("a task with that title already exists")
		}
	}
	for k, v := range fields {
		if k == "title" || k == "description" || k == "start_time" || k == "due_time" || k == "priority" || k == "status" || k == "recurrence" {
			if e := writeField(dir, k, v); e != nil {
				return e
			}
		}
	}
	if dest != dir {
		if e := os.Rename(dir, dest); e != nil {
			return e
		}
		dir = dest
	}
	return buildDetails(dir)
}
func writeAlerts(dir string, alerts []string) error {
	sort.Strings(alerts)
	data := strings.Join(alerts, "\n")
	if len(alerts) > 0 {
		data += "\n"
	}
	if e := os.WriteFile(filepath.Join(dir, ".tmp", "alerts"), []byte(data), 0644); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(dir, ".tmp", "alerts_template"), []byte(data), 0644); e != nil {
		return e
	}
	return buildDetails(dir)
}
