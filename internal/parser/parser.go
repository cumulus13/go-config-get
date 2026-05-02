// File: parser.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: Package parser provides parsers for all supported config file formats:
// License: MIT

// Package parser provides parsers for all supported config file formats:
// .env, .ini, .toml, .json, .yml/.yaml
//
// Only TOML requires an external dependency (github.com/BurntSushi/toml).
// All other formats are parsed with zero external dependencies.
package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Data is the parsed representation of a config file.
// Top-level keys map to string values (flat .env) or nested
// map[string]interface{} (sectioned formats like .ini, .toml, .json, .yaml).
type Data map[string]interface{}

// ParseFile dispatches to the correct parser based on the file extension.
func ParseFile(path string) (Data, error) {
	ext := strings.ToLower(filepath.Ext(path))
	base := filepath.Base(path)

	// Bare dotfiles like ".env"
	if ext == "" && strings.HasPrefix(base, ".") {
		ext = ".env"
	}

	switch ext {
	case ".env":
		return ParseEnv(path)
	case ".ini":
		return ParseIni(path)
	case ".toml":
		return ParseToml(path)
	case ".json":
		return ParseJSON(path)
	case ".yml", ".yaml":
		return ParseYAML(path)
	default:
		return nil, fmt.Errorf("unsupported config format %q (supported: .env .ini .toml .json .yml .yaml)", ext)
	}
}

// ParseEnv parses a KEY=VALUE dotenv file.
// Lines beginning with '#' are comments. Surrounding quotes are stripped.
func ParseEnv(path string) (Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .env %s: %w", path, err)
	}
	defer f.Close()

	data := make(Data)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue // no '=' → skip
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Strip optional surrounding quotes (single or double)
		if len(val) >= 2 && val[0] == val[len(val)-1] && (val[0] == '"' || val[0] == '\'') {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			data[key] = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan .env %s: %w", path, err)
	}
	return data, nil
}

// ParseIni parses a .ini file into a nested Data map using the built-in parser.
func ParseIni(path string) (Data, error) {
	return parseIniFile(path)
}

// ParseToml parses a .toml file using github.com/BurntSushi/toml.
func ParseToml(path string) (Data, error) {
	var raw map[string]interface{}
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("parse .toml %s: %w", path, err)
	}
	return Data(raw), nil
}

// ParseJSON parses a .json file. The root value must be a JSON object.
func ParseJSON(path string) (Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .json %s: %w", path, err)
	}
	defer f.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse .json %s: %w", path, err)
	}
	return Data(raw), nil
}

// ParseYAML parses a .yml / .yaml file using the built-in YAML parser.
func ParseYAML(path string) (Data, error) {
	return parseYAMLFile(path)
}
