// File: ini.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: parseIniFile is a minimal, zero-dependency INI parser.
// License: MIT

package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// parseIniFile is a minimal, zero-dependency INI parser.
// Supports:
//   - [section] headers
//   - key = value  and  key: value
//   - # and ; line comments
//   - inline comments (# or ; after value)
//   - multi-word values
//   - DEFAULT section
func parseIniFile(path string) (Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .ini %s: %w", path, err)
	}
	defer f.Close()

	data := make(Data)
	currentSection := "DEFAULT"
	sectionMap := make(map[string]interface{})
	data[currentSection] = sectionMap

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comment-only lines
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}

		// Section header
		if line[0] == '[' {
			end := strings.Index(line, "]")
			if end < 0 {
				continue
			}
			currentSection = strings.TrimSpace(line[1:end])
			if _, ok := data[currentSection]; !ok {
				m := make(map[string]interface{})
				data[currentSection] = m
				sectionMap = m
			} else {
				sectionMap = data[currentSection].(map[string]interface{})
			}
			continue
		}

		// key = value  or  key: value
		var key, val string
		if idx := strings.IndexAny(line, "=:"); idx >= 0 {
			key = strings.TrimSpace(line[:idx])
			val = strings.TrimSpace(line[idx+1:])
		} else {
			// Boolean key (no value)
			key = line
			val = "true"
		}

		// Strip inline comment
		for _, commentChar := range []string{" #", " ;"} {
			if ci := strings.Index(val, commentChar); ci >= 0 {
				val = strings.TrimSpace(val[:ci])
				break
			}
		}

		// Strip surrounding quotes
		if len(val) >= 2 && val[0] == val[len(val)-1] && (val[0] == '"' || val[0] == '\'') {
			val = val[1 : len(val)-1]
		}

		if key != "" {
			sectionMap[key] = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan .ini %s: %w", path, err)
	}

	// Remove empty DEFAULT section
	if m, ok := data["DEFAULT"].(map[string]interface{}); ok && len(m) == 0 {
		delete(data, "DEFAULT")
	}

	return data, nil
}
