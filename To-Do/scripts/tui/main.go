package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	alt := true // alternate screen keeps mouse clicks exactly on the cells
	dir := ""
	for _, a := range os.Args[1:] {
		switch a {
		case "--inline":
			alt = false
		case "--alt":
			alt = true
		default:
			dir = a
		}
	}
	if dir == "" {
		fmt.Println("Usage: to-do-tui [--inline] <task-directory>")
		os.Exit(1)
	}
	if _, e := os.Stat(dir); e != nil {
		fmt.Println("Task directory not found:", dir)
		os.Exit(1)
	}
	if _, e := tea.NewProgram(newModel(dir, alt)).Run(); e != nil {
		fmt.Println("Error:", e)
		os.Exit(1)
	}
}
