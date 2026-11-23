package main

import (
	"os"
	"strings"
)

func workLogDescription(file string) string {
	b, _ := os.ReadFile(file)
	p := strings.SplitN(string(b), "\n", 3)
	if len(p) < 3 {
		return ""
	}
	return strings.TrimSpace(p[2])
}
func workLogHeader(file string) string {
	b, _ := os.ReadFile(file)
	return strings.SplitN(string(b), "\n", 2)[0]
}
