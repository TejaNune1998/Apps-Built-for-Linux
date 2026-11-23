package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func parseStep(step string) (int, string, bool) {
	p := strings.Fields(step)
	if len(p) != 2 {
		return 0, "", false
	}
	n, e := strconv.Atoi(p[0])
	if e != nil || n < 1 {
		return 0, "", false
	}
	u := strings.TrimSuffix(p[1], "s")
	switch u {
	case "day", "week", "month", "year":
		return n, u, true
	}
	return 0, "", false
}
func addStep(t time.Time, n int, u string) time.Time {
	switch u {
	case "day":
		return t.AddDate(0, 0, n)
	case "week":
		return t.AddDate(0, 0, 7*n)
	case "month":
		return t.AddDate(0, n, 0)
	default:
		return t.AddDate(n, 0, 0)
	}
}
func offsetSeconds(s string) (time.Duration, bool) {
	p := strings.Fields(s)
	if len(p) != 2 {
		return 0, false
	}
	n, e := strconv.Atoi(p[0])
	if e != nil {
		return 0, false
	}
	switch p[1] {
	case "minute", "minutes":
		return time.Duration(n) * time.Minute, true
	case "hour", "hours":
		return time.Duration(n) * time.Hour, true
	case "day", "days":
		return time.Duration(n) * 24 * time.Hour, true
	case "week", "weeks":
		return time.Duration(n) * 7 * 24 * time.Hour, true
	}
	return 0, false
}
func parseTaskTime(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", s, time.Local)
}
func nextOccurrence(snap, dir string) error {
	tmp := snap
	stepBytes, _ := os.ReadFile(filepath.Join(snap, "recurrence"))
	step := strings.TrimSpace(string(stepBytes))
	n, u, ok := parseStep(step)
	if !ok {
		return fmt.Errorf("invalid recurrence: %s", step)
	}
	startBytes, _ := os.ReadFile(filepath.Join(snap, "start_time"))
	start, e := parseTaskTime(strings.TrimSpace(string(startBytes)))
	if e != nil {
		return e
	}
	dueBytes, _ := os.ReadFile(filepath.Join(snap, "due_time"))
	due, e := parseTaskTime(strings.TrimSpace(string(dueBytes)))
	if e != nil {
		return e
	}
	now := time.Now()
	ns, nd := addStep(start, n, u), addStep(due, n, u)
	for !ns.After(now) {
		ns = addStep(ns, n, u)
		nd = addStep(nd, n, u)
	}
	newDir := dir
	if _, e = os.Stat(newDir); e == nil {
		return fmt.Errorf("next occurrence path already exists")
	}
	if e = os.MkdirAll(filepath.Join(newDir, ".tmp"), 0755); e != nil {
		return e
	}
	for _, f := range []string{"title", "description", "priority", "recurrence"} {
		b, e := os.ReadFile(filepath.Join(tmp, f))
		if e == nil {
			_ = os.WriteFile(filepath.Join(newDir, ".tmp", f), b, 0644)
		}
	}
	_ = writeField(newDir, "start_time", ns.Format("2006-01-02 15:04"))
	_ = writeField(newDir, "due_time", nd.Format("2006-01-02 15:04"))
	_ = writeField(newDir, "status", "Todo")
	for _, f := range []string{"alerts", "alerts_template", "alert_history"} {
		_ = os.WriteFile(filepath.Join(newDir, ".tmp", f), nil, 0644)
	}
	data, _ := os.ReadFile(filepath.Join(tmp, "alerts_template"))
	var templates, active []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		p := strings.SplitN(line, "|", 2)
		if len(p) != 2 || p[0] == "" {
			continue
		}
		desc := p[1]
		var at time.Time
		switch {
		case desc == "At start time":
			at = ns
		case desc == "At due time":
			at = nd
		case strings.HasSuffix(desc, " before start"):
			off, ok := offsetSeconds(strings.TrimSuffix(desc, " before start"))
			if !ok {
				continue
			}
			at = ns.Add(-off)
		case strings.HasSuffix(desc, " before due"):
			off, ok := offsetSeconds(strings.TrimSuffix(desc, " before due"))
			if !ok {
				continue
			}
			at = nd.Add(-off)
		default:
			old, er := parseTaskTime(p[0])
			if er != nil {
				continue
			}
			at = addStep(old, n, u)
		}
		line = at.Format("2006-01-02 15:04") + "|" + desc
		templates = append(templates, line)
		if at.After(now) {
			active = append(active, line)
		}
	}
	_ = os.WriteFile(filepath.Join(newDir, ".tmp", "alerts_template"), []byte(strings.Join(templates, "\n")+func() string {
		if len(templates) > 0 {
			return "\n"
		}
		return ""
	}()), 0644)
	_ = os.WriteFile(filepath.Join(newDir, ".tmp", "alerts"), []byte(strings.Join(active, "\n")+func() string {
		if len(active) > 0 {
			return "\n"
		}
		return ""
	}()), 0644)
	return buildDetails(newDir)
}
