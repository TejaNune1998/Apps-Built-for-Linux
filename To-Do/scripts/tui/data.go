package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type item struct{ name, path string }

// entity is a task or a sub-task, open or closed.
type entity struct {
	path, dir, title, status, priority, start, due, repeat string
	label                                                  string
	isSub                                                  bool
	closed                                                 string // "", "DONE", "CANCEL"
	children                                               int    // open sub-tasks
}

func closedKind(dir string) string {
	switch {
	case strings.HasPrefix(dir, "DONE_"):
		return "DONE"
	case strings.HasPrefix(dir, "CANCEL_"):
		return "CANCEL"
	}
	return ""
}

func trimClosedPrefix(s string) string {
	s = strings.TrimPrefix(s, "DONE_")
	return strings.TrimPrefix(s, "CANCEL_")
}

func loadEntity(workDir, path string) entity {
	e := entity{path: path, dir: filepath.Base(path)}
	e.title = readField(path, "title")
	if e.title == "" {
		e.title = strings.ReplaceAll(trimClosedPrefix(e.dir), "_", " ")
	}
	e.status = readField(path, "status")
	e.priority = readField(path, "priority")
	e.start = readField(path, "start_time")
	e.due = readField(path, "due_time")
	e.repeat = readField(path, "recurrence")
	e.closed = closedKind(e.dir)
	if rel, err := filepath.Rel(workDir, path); err == nil {
		parts := strings.Split(rel, string(os.PathSeparator))
		e.label = strings.ReplaceAll(parts[0], "_", " ")
		e.isSub = len(parts) >= 3
	}
	if e.closed == "" {
		e.children = len(getDirs(path))
	}
	return e
}

// ---- rows of the Tasks panel ------------------------------------------

type rowKind int

const (
	rowTask rowKind = iota
	rowSub
	rowGroup
	rowClosed
)

type row struct {
	kind  rowKind
	e     entity
	count int // rowGroup: number of closed items
}

func (m *model) buildRows() {
	m.rows = nil
	lp := m.labelPath()
	if lp == "" {
		return
	}
	var closed []entity
	for _, t := range getDirs(lp) {
		e := loadEntity(m.workDir, t.path)
		m.rows = append(m.rows, row{kind: rowTask, e: e})
		if m.expanded[e.path] {
			for _, s := range getDirs(t.path) {
				m.rows = append(m.rows, row{kind: rowSub, e: loadEntity(m.workDir, s.path)})
			}
		}
		for _, c := range getClosedDirs(t.path) { // closed sub-tasks of open tasks
			ce := loadEntity(m.workDir, c.path)
			ce.title = e.title + " › " + ce.title
			closed = append(closed, ce)
		}
	}
	for _, c := range getClosedDirs(lp) {
		closed = append(closed, loadEntity(m.workDir, c.path))
	}
	if len(closed) > 0 {
		m.rows = append(m.rows, row{kind: rowGroup, count: len(closed)})
		if m.closedOpen {
			for _, c := range closed {
				m.rows = append(m.rows, row{kind: rowClosed, e: c})
			}
		}
	}
}

func (m *model) labelPath() string {
	if m.li < 0 || m.li >= len(m.labels) {
		return ""
	}
	return m.labels[m.li].path
}

// ---- counts ------------------------------------------------------------

type counts struct{ openTasks, openSubs, doneTasks, doneSubs int }

func labelCounts(labelPath string) counts {
	var c counts
	entries, _ := os.ReadDir(labelPath)
	for _, t := range entries {
		n := t.Name()
		if !t.IsDir() || strings.HasPrefix(n, ".") || n == "worklog" || strings.HasPrefix(n, "CANCEL_") || strings.HasPrefix(n, "DELETED_") {
			continue
		}
		if strings.HasPrefix(n, "DONE_") {
			c.doneTasks++
		} else {
			c.openTasks++
		}
		subs, _ := os.ReadDir(filepath.Join(labelPath, n))
		for _, s := range subs {
			sn := s.Name()
			if !s.IsDir() || strings.HasPrefix(sn, ".") || sn == "worklog" || strings.HasPrefix(sn, "CANCEL_") || strings.HasPrefix(sn, "DELETED_") {
				continue
			}
			if strings.HasPrefix(sn, "DONE_") {
				c.doneSubs++
			} else {
				c.openSubs++
			}
		}
	}
	return c
}

// ---- today / upcoming --------------------------------------------------

type agendaItem struct {
	at      time.Time
	kind    string // "start" or "due"
	title   string
	label   string
	isSub   bool
	path    string
	overdue bool
}

func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// agenda lists the start/due times of every open task and sub-task between
// from (inclusive) and to (exclusive), in time order.
func (m *model) agenda(from, to time.Time) []agendaItem {
	var out []agendaItem
	now := time.Now()
	add := func(e entity) {
		for _, k := range []string{"start", "due"} {
			raw := e.start
			if k == "due" {
				raw = e.due
			}
			t, err := parseTaskTime(raw)
			if err != nil || t.Before(from) || !t.Before(to) {
				continue
			}
			out = append(out, agendaItem{at: t, kind: k, title: e.title, label: e.label, isSub: e.isSub, path: e.path,
				overdue: k == "due" && t.Before(now) && e.status != "Completed"})
		}
	}
	for _, l := range getDirs(m.workDir) {
		for _, t := range getDirs(l.path) {
			add(loadEntity(m.workDir, t.path))
			for _, s := range getDirs(t.path) {
				add(loadEntity(m.workDir, s.path))
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].at.Equal(out[j].at) {
			return out[i].at.Before(out[j].at)
		}
		if out[i].kind != out[j].kind {
			return out[i].kind == "start"
		}
		return strings.ToLower(out[i].title) < strings.ToLower(out[j].title)
	})
	return out
}

// ---- work logs ---------------------------------------------------------

type logEntry struct{ file, stamp, text string }

const logHeaderPrefix = "### Work Log — "

func loadLogs(dir string) []logEntry {
	files, _ := filepath.Glob(filepath.Join(dir, "worklog", "*.md"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	var out []logEntry
	for _, f := range files {
		h := workLogHeader(f)
		stamp := strings.TrimSpace(strings.TrimPrefix(h, logHeaderPrefix))
		if stamp == "" || stamp == h {
			stamp = strings.TrimSuffix(filepath.Base(f), ".md")
		}
		out = append(out, logEntry{file: f, stamp: stamp, text: workLogDescription(f)})
	}
	return out
}

func cleanName(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.ContainsAny(s, "/_") && !strings.HasPrefix(s, ".") && !strings.HasPrefix(s, "DONE ") && !strings.HasPrefix(s, "CANCEL ") && !strings.HasPrefix(s, "DELETED ") && s != "Back" && s != "Delete Label"
}

func folderName(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), " ", "_") }
