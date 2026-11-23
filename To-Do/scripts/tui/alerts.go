package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func makeAlert(dir, kind, offset, custom string) (string, error) {
	start, e := parseTaskTime(readField(dir, "start_time"))
	if e != nil {
		return "", e
	}
	due, e := parseTaskTime(readField(dir, "due_time"))
	if e != nil {
		return "", e
	}
	var at time.Time
	var desc string
	switch kind {
	case "At Start Time":
		at = start
		desc = "At start time"
	case "At Due Time":
		at = due
		desc = "At due time"
	case "Before Start Time":
		d, ok := offsetSeconds(offset)
		if !ok {
			return "", fmt.Errorf("invalid offset")
		}
		at = start.Add(-d)
		desc = offset + " before start"
	case "Before Due Time":
		d, ok := offsetSeconds(offset)
		if !ok {
			return "", fmt.Errorf("invalid offset")
		}
		at = due.Add(-d)
		desc = offset + " before due"
	case "Custom":
		at, e = parseTaskTime(custom)
		if e != nil {
			return "", e
		}
		desc = "Custom alert"
	default:
		return "", fmt.Errorf("unknown alert type")
	}
	return at.Format("2006-01-02 15:04") + "|" + desc, nil
}
func readAlerts(dir string) []string {
	b, _ := os.ReadFile(filepath.Join(dir, ".tmp", "alerts"))
	var a []string
	for _, s := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if s != "" {
			a = append(a, s)
		}
	}
	return a
}
func addAlerts(dir string, alerts []string) error {
	all := append(readAlerts(dir), alerts...)
	sort.Strings(all)
	return writeAlerts(dir, all)
}
func removeAlert(dir, line string) error {
	all := readAlerts(dir)
	var keep []string
	for _, a := range all {
		if a != line {
			keep = append(keep, a)
		}
	}
	if e := os.WriteFile(filepath.Join(dir, ".tmp", "alerts"), []byte(strings.Join(keep, "\n")+func() string {
		if len(keep) > 0 {
			return "\n"
		}
		return ""
	}()), 0644); e != nil {
		return e
	}
	tmpl := filepath.Join(dir, ".tmp", "alerts_template")
	b, _ := os.ReadFile(tmpl)
	var next []string
	for _, a := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if a != "" && a != line {
			next = append(next, a)
		}
	}
	_ = os.WriteFile(tmpl, []byte(strings.Join(next, "\n")+func() string {
		if len(next) > 0 {
			return "\n"
		}
		return ""
	}()), 0644)
	return buildDetails(dir)
}
