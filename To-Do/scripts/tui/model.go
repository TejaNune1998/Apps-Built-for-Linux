package main

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type item struct{ name, path string }

type model struct {
	workDir                                  string
	labels, tasks                            []item
	worklogs                                 []string
	selectedLabel, selectedTask, selectedLog int
	activePanel                              int
	currentParent                            string
	details                                  string
	logs                                     viewport.Model
	width, height                            int
	mode                                     string // menu, input, confirm, viewlog
	menuTitle                                string
	menuItems                                []string
	menuIndex                                int
	inputTitle, inputValue, inputField       string
	inputStep                                int
	form                                     map[string]string
	formFields                               []string
	formTarget                               string
	message                                  string
	returnMode                               string
}

func initialModel(workDir string) model {
	m := model{workDir: workDir, logs: viewport.New(), activePanel: 0, form: map[string]string{}}
	m.loadLabels()
	m.loadTasks()
	return m
}
func (m model) Init() tea.Cmd { return nil }
func (m *model) refresh() {
	m.loadLabels()
	if m.currentParent != "" {
		if _, err := os.Stat(m.currentParent); err != nil {
			m.currentParent = ""
		}
	}
	m.loadTasks()
}
func (m *model) selectedPath() string {
	if len(m.tasks) == 0 || m.selectedTask < 0 || m.selectedTask >= len(m.tasks) {
		return ""
	}
	return m.tasks[m.selectedTask].path
}
func (m *model) selectedLabelPath() string {
	if len(m.labels) == 0 || m.selectedLabel < 0 || m.selectedLabel >= len(m.labels) {
		return ""
	}
	return m.labels[m.selectedLabel].path
}
func (m *model) openMenu(title string, choices ...string) {
	m.mode = "menu"
	m.menuTitle = title
	m.menuItems = choices
	m.menuIndex = 0
}
func (m *model) startInput(title, field, value string) {
	m.mode = "input"
	m.inputTitle = title
	m.inputField = field
	m.inputValue = value
}
func cleanName(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.ContainsAny(s, "/_") && !strings.HasPrefix(s, ".") && !strings.HasPrefix(s, "DONE ") && !strings.HasPrefix(s, "CANCEL ") && !strings.HasPrefix(s, "DELETED ") && s != "Back" && s != "Delete Label"
}
func folderName(s string) string            { return strings.ReplaceAll(strings.TrimSpace(s), " ", "_") }
func (m *model) setMessage(s string)        { m.message = s }
func (m *model) selectedEntityPath() string { return m.selectedPath() }
func (m *model) isTopLevelTask() bool {
	p := m.selectedPath()
	return p != "" && filepath.Dir(filepath.Dir(p)) == m.workDir
}
