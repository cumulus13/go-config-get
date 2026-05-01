// File: main.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: config-get CLI — cross-platform configuration file discovery and retrieval.
// License: MIT

// config-get CLI — cross-platform configuration file discovery and retrieval.
//
// Usage:
//
//	config-get [flags] [KEY]
//
// Examples:
//
//	config-get --locate --config myapp.toml --dir myapp
//	config-get DB_HOST --config myapp.toml --dir myapp --default localhost
//	config-get PORT --cast int --default 8080
//	config-get --dump --config myapp.toml
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/cumulus13/go-config-get/configget"
)

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("config-get", flag.ContinueOnError)

	configFile := fs.String("config", ".env", "Config filename / stem to search for")
	configDir := fs.String("dir", "", "Subdirectory within the platform config root")
	section := fs.String("section", "", "Section name (for INI/TOML/JSON/YAML nested configs)")
	defaultVal := fs.String("default", "", "Default value when the key is not found")
	castType := fs.String("cast", "", "Cast type: string | int | float | bool")
	locate := fs.Bool("locate", false, "Print the resolved config file path and exit")
	dump := fs.Bool("dump", false, "Print the full parsed config as JSON and exit")
	extFlag := fs.String("ext", "", "Comma-separated extensions to restrict search (e.g. .toml,.ini)")
	noCreate := fs.Bool("no-create", false, "Do not create config directory if it doesn't exist")
	verbose := fs.Bool("v", false, "Enable debug logging")
	version := fs.Bool("version", false, "Print version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if *verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	if *version {
		fmt.Printf("config-get %s\n", configget.Version)
		return 0
	}

	var exts []string
	if *extFlag != "" {
		for _, e := range strings.Split(*extFlag, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				exts = append(exts, e)
			}
		}
	}

	opts := configget.Options{
		Create:     !*noCreate,
		Extensions: exts,
	}

	// --locate
	if *locate {
		path, err := configget.GetConfigFile(*configFile, *configDir, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		fmt.Println(path)
		return 0
	}

	// --dump
	if *dump {
		data, err := configget.LoadConfig(*configFile, *configDir, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		return 0
	}

	// KEY retrieval
	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return 2
	}
	key := args[0]

	gopts := configget.GetOptions{
		Section:    *section,
		Default:    *defaultVal,
		Extensions: exts,
	}

	switch strings.ToLower(*castType) {
	case "int":
		v, err := configget.GetInt(key, *configFile, *configDir, gopts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		fmt.Println(v)

	case "float":
		v, err := configget.GetFloat(key, *configFile, *configDir, gopts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		fmt.Println(v)

	case "bool":
		v, err := configget.GetBool(key, *configFile, *configDir, gopts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		fmt.Println(v)

	default: // "string" or empty
		v, err := configget.GetString(key, *configFile, *configDir, gopts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config-get:", err)
			return 1
		}
		if v == "" && *defaultVal != "" {
			fmt.Println(*defaultVal)
			return 0
		}
		if v == "" {
			return 1
		}
		fmt.Println(v)
	}

	return 0
}
