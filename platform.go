// File: platform.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: Package platform provides cross-platform base directory resolution for config file discovery (Windows, macOS, Linux/XDG).
// License: MIT

// Package platform provides cross-platform base directory resolution for
// config file discovery (Windows, macOS, Linux/XDG).
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BaseDirs returns the ordered list of base directories to search for config
// files on the current platform.
func BaseDirs() []string {
	if runtime.GOOS == "windows" {
		return windowsDirs()
	}
	return unixDirs()
}

// windowsDirs returns %APPDATA%, %USERPROFILE%, %LOCALAPPDATA% (in that order),
// skipping any that are unset.
func windowsDirs() []string {
	vars := []string{"APPDATA", "USERPROFILE", "LOCALAPPDATA"}
	var dirs []string
	for _, v := range vars {
		val := os.Getenv(v)
		if val != "" {
			dirs = append(dirs, val)
		}
	}
	return dirs
}

// unixDirs returns $HOME, $HOME/.config, /etc (XDG-aware, in that order).
func unixDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	dirs := []string{home}

	// Respect XDG_CONFIG_HOME if set
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		dirs = append(dirs, xdgConfig)
	} else if home != "" {
		dirs = append(dirs, filepath.Join(home, ".config"))
	}

	dirs = append(dirs, "/etc")
	return dirs
}

// BuildCandidatePaths returns an ordered, de-duplicated list of candidate
// config file paths for the given filename stem and optional sub-directory.
//
// Extensions are tried in priority order: .env, .ini, .toml, .json, .yml, .yaml
func BuildCandidatePaths(configFile, configDir string) []string {
	stem := stemOf(configFile)
	configDir = strings.Trim(configDir, `/\`)

	extensions := []string{".env", ".ini", ".toml", ".json", ".yml", ".yaml"}

	filenamesFor := func(ext string) []string {
		if ext == ".env" {
			return []string{".env"}
		}
		return []string{stem + ext}
	}

	bases := BaseDirs()
	seen := make(map[string]struct{})
	var candidates []string

	add := func(p string) {
		// Normalize to clean absolute path for de-dup
		key := filepath.Clean(p)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			candidates = append(candidates, p)
		}
	}

	for _, base := range bases {
		// With configDir sub-directory
		if configDir != "" {
			sub := filepath.Join(base, configDir)
			for _, ext := range extensions {
				for _, name := range filenamesFor(ext) {
					add(filepath.Join(sub, name))
				}
			}
		}
		// Without configDir (fallback / legacy)
		for _, ext := range extensions {
			for _, name := range filenamesFor(ext) {
				add(filepath.Join(base, name))
			}
		}
	}

	return candidates
}

// stemOf returns the filename stem (without extension).
// For dotfiles like ".env" it returns "config" as a safe fallback.
func stemOf(filename string) string {
	base := filepath.Base(filename)
	if strings.HasPrefix(base, ".") && strings.Count(base, ".") == 1 {
		// bare dotfile — use "config" as stem
		return "config"
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}
