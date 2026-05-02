// File: yaml.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: parseYAMLFile is a minimal, zero-dependency YAML parser.
// License: MIT

package parser

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// parseYAMLFile is a minimal, zero-dependency YAML parser.
// Supports a strict but practical subset:
//   - Flat key: value mappings
//   - Nested mappings via indentation (2 or 4 spaces)
//   - Block scalars are NOT supported (use JSON/TOML for complex configs)
//   - # comments
//   - Quoted and unquoted scalar values
//   - Boolean, integer, float auto-detection
//   - Lists as arrays of strings (- item)
func parseYAMLFile(path string) (Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .yaml %s: %w", path, err)
	}
	defer f.Close()

	lines := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan .yaml %s: %w", path, err)
	}

	result, _, err := parseYAMLBlock(lines, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("parse .yaml %s: %w", path, err)
	}
	if m, ok := result.(map[string]interface{}); ok {
		return Data(m), nil
	}
	return nil, fmt.Errorf("parse .yaml %s: root must be a mapping", path)
}

// parseYAMLBlock recursively parses YAML mappings starting at baseIndent.
// Returns the parsed value, the index of the next unprocessed line, and any error.
func parseYAMLBlock(lines []string, start, baseIndent int) (interface{}, int, error) {
	// Determine if this block is a mapping or a sequence
	for i := start; i < len(lines); i++ {
		raw := lines[i]
		stripped := strings.TrimLeft(raw, " \t")
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		indent := len(raw) - len(stripped)
		if indent < baseIndent {
			break
		}
		if strings.HasPrefix(stripped, "- ") || stripped == "-" {
			return parseYAMLSequence(lines, start, baseIndent)
		}
		break
	}
	return parseYAMLMapping(lines, start, baseIndent)
}

func parseYAMLMapping(lines []string, start, baseIndent int) (interface{}, int, error) {
	result := make(map[string]interface{})
	i := start

	for i < len(lines) {
		raw := lines[i]
		stripped := strings.TrimLeft(raw, " \t")

		// Skip blank lines and comments
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			i++
			continue
		}

		indent := len(raw) - len(stripped)

		// If indentation is less than base, we're done with this block
		if indent < baseIndent {
			break
		}

		// If indentation is greater than base, it belongs to parent's last key (handled below)
		if indent > baseIndent {
			i++
			continue
		}

		// Must contain a colon to be a mapping entry
		colonIdx := strings.Index(stripped, ":")
		if colonIdx < 0 {
			i++
			continue
		}

		key := strings.TrimSpace(stripped[:colonIdx])
		rest := strings.TrimSpace(stripped[colonIdx+1:])

		// Strip inline comment from rest
		if commentIdx := findInlineComment(rest); commentIdx >= 0 {
			rest = strings.TrimSpace(rest[:commentIdx])
		}

		i++

		if rest != "" {
			// Inline value on same line
			result[key] = parseYAMLScalar(rest)
		} else {
			// Value is on following lines (nested block)
			// Determine child indent
			childIndent := -1
			for j := i; j < len(lines); j++ {
				s := strings.TrimLeft(lines[j], " \t")
				if s == "" || strings.HasPrefix(s, "#") {
					continue
				}
				childIndent = len(lines[j]) - len(s)
				break
			}
			if childIndent > baseIndent {
				var val interface{}
				var err error
				val, i, err = parseYAMLBlock(lines, i, childIndent)
				if err != nil {
					return nil, i, err
				}
				result[key] = val
			} else {
				result[key] = nil
			}
		}
	}

	return result, i, nil
}

func parseYAMLSequence(lines []string, start, baseIndent int) (interface{}, int, error) {
	var items []interface{}
	i := start

	for i < len(lines) {
		raw := lines[i]
		stripped := strings.TrimLeft(raw, " \t")

		if stripped == "" || strings.HasPrefix(stripped, "#") {
			i++
			continue
		}

		indent := len(raw) - len(stripped)
		if indent < baseIndent {
			break
		}

		if strings.HasPrefix(stripped, "- ") {
			val := strings.TrimSpace(stripped[2:])
			if commentIdx := findInlineComment(val); commentIdx >= 0 {
				val = strings.TrimSpace(val[:commentIdx])
			}
			items = append(items, parseYAMLScalar(val))
			i++
		} else if stripped == "-" {
			items = append(items, nil)
			i++
		} else {
			break
		}
	}

	return items, i, nil
}

// parseYAMLScalar converts a raw YAML scalar string to an appropriate Go type.
func parseYAMLScalar(s string) interface{} {
	if s == "" || s == "~" || strings.EqualFold(s, "null") {
		return nil
	}

	// Quoted string
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}

	// Boolean
	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}

	// Integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return s
}

// findInlineComment returns the index of a YAML inline comment (" #" preceded by space).
// Returns -1 if none found. Does not strip comments inside quoted values.
func findInlineComment(s string) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if c == '#' && !inSingle && !inDouble && i > 0 && s[i-1] == ' ' {
			return i - 1
		}
	}
	return -1
}
