// File: configget.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: Package configget provides cross-platform configuration file discovery,
// License: MIT

// Package configget provides cross-platform configuration file discovery,
// loading, and value retrieval.
//
// It supports .env, .ini, .toml, .json, .yml/.yaml formats and searches
// standard platform-specific locations (XDG on Linux/macOS, %APPDATA% on
// Windows). Environment variables always take precedence over file values.
//
// # Quick start
//
//	// Functional API
//	path, err := configget.GetConfigFile(".env", "myapp", configget.Options{Create: true})
//	data, err := configget.LoadConfig("myapp.toml", "myapp", configget.Options{})
//	host, err := configget.GetString("DB_HOST", "myapp.toml", "myapp", configget.GetOptions{Default: "localhost"})
//	port, err := configget.GetInt("DB_PORT", "myapp.toml", "myapp", configget.GetOptions{Default: 5432})
//
//	// Object-oriented API
//	cfg := configget.New("myapp.toml", "myapp", configget.Options{})
//	host := cfg.String("DB_HOST", "localhost")
//	port := cfg.Int("DB_PORT", 5432)
//	debug := cfg.Bool("DEBUG", false)
package configget

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cumulus13/config-get/internal/cast"
	"github.com/cumulus13/config-get/internal/parser"
	"github.com/cumulus13/config-get/internal/platform"
)

const Version = "1.0.0"

// SupportedFormats lists the config file extensions supported, in priority order.
var SupportedFormats = []string{".env", ".ini", ".toml", ".json", ".yml", ".yaml"}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

// Options controls the behavior of config file discovery and loading.
type Options struct {
	// Create the parent directory of the resolved config file if it doesn't exist.
	// Default: true
	Create bool

	// Extensions restricts the search to these extensions only (e.g. []string{".toml", ".ini"}).
	// An empty slice means all supported formats are tried.
	Extensions []string

	// Strict causes LoadConfig to return an error when no config file is found,
	// rather than silently returning an empty map.
	Strict bool
}

// DefaultOptions returns a sensible set of defaults.
func DefaultOptions() Options {
	return Options{Create: true}
}

// GetOptions controls single-key retrieval.
type GetOptions struct {
	// Section navigates into a nested section (for .ini / .toml / .json / .yaml).
	Section string

	// Default is returned when the key is absent from both env and config file.
	Default interface{}

	// Extensions restricts the config file search to these extensions.
	Extensions []string
}

// ---------------------------------------------------------------------------
// GetConfigFile
// ---------------------------------------------------------------------------

// GetConfigFile discovers the first existing config file matching configFile
// and configDir in the standard platform locations. If none exists, it returns
// the preferred default path (first candidate) and optionally creates its
// parent directory.
func GetConfigFile(configFile, configDir string, opts Options) (string, error) {
	if configFile == "" {
		configFile = ".env"
	}

	candidates := platform.BuildCandidatePaths(configFile, configDir)

	if len(opts.Extensions) > 0 {
		exts := normalizeExtensions(opts.Extensions)
		filtered := candidates[:0]
		for _, c := range candidates {
			ext := strings.ToLower(filepath.Ext(c))
			if ext == "" && strings.HasPrefix(filepath.Base(c), ".") {
				ext = ".env"
			}
			if _, ok := exts[ext]; ok {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}

	// Find first existing file
	for _, c := range candidates {
		if isFile(c) {
			slog.Debug("config-get: found config", "path", c)
			return c, nil
		}
	}

	// No file found → use first candidate as default
	var chosen string
	if len(candidates) > 0 {
		chosen = candidates[0]
	} else {
		chosen = configFile
	}
	slog.Debug("config-get: no config found, defaulting", "path", chosen)

	if opts.Create {
		dir := filepath.Dir(chosen)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Warn("config-get: could not create directory", "dir", dir, "err", err)
		} else {
			slog.Debug("config-get: created directory", "dir", dir)
		}
	}

	return chosen, nil
}

// ---------------------------------------------------------------------------
// LoadConfig
// ---------------------------------------------------------------------------

// LoadConfig discovers and parses a config file, returning its data.
// Returns an empty map (and optionally an error when Strict=true) if no file exists.
func LoadConfig(configFile, configDir string, opts Options) (parser.Data, error) {
	path, err := GetConfigFile(configFile, configDir, opts)
	if err != nil {
		return nil, err
	}

	if !isFile(path) {
		if opts.Strict {
			return nil, &ConfigFileNotFoundError{
				ConfigFile: configFile,
				ConfigDir:  configDir,
			}
		}
		slog.Debug("config-get: config file does not exist yet, returning empty data")
		return parser.Data{}, nil
	}

	data, err := parser.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("config-get: %w", err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Get* — typed functional helpers
// ---------------------------------------------------------------------------

// GetString retrieves a string value. Env vars take precedence over file values.
func GetString(key, configFile, configDir string, gopts GetOptions) (string, error) {
	if v, ok := os.LookupEnv(key); ok {
		return v, nil
	}
	raw, err := lookupRaw(key, configFile, configDir, gopts)
	if err != nil {
		return "", err
	}
	if raw == nil {
		if s, ok := gopts.Default.(string); ok {
			return s, nil
		}
		return "", nil
	}
	return cast.ToString(raw)
}

// GetInt retrieves an int64 value.
func GetInt(key, configFile, configDir string, gopts GetOptions) (int64, error) {
	if v, ok := os.LookupEnv(key); ok {
		return cast.ToInt(v)
	}
	raw, err := lookupRaw(key, configFile, configDir, gopts)
	if err != nil {
		return 0, err
	}
	if raw == nil {
		if i, ok := gopts.Default.(int); ok {
			return int64(i), nil
		}
		if i, ok := gopts.Default.(int64); ok {
			return i, nil
		}
		return 0, nil
	}
	return cast.ToInt(raw)
}

// GetFloat retrieves a float64 value.
func GetFloat(key, configFile, configDir string, gopts GetOptions) (float64, error) {
	if v, ok := os.LookupEnv(key); ok {
		return cast.ToFloat(v)
	}
	raw, err := lookupRaw(key, configFile, configDir, gopts)
	if err != nil {
		return 0, err
	}
	if raw == nil {
		if f, ok := gopts.Default.(float64); ok {
			return f, nil
		}
		return 0, nil
	}
	return cast.ToFloat(raw)
}

// GetBool retrieves a bool value.
// Truthy strings: "1", "true", "yes", "on". Falsy: "0", "false", "no", "off".
func GetBool(key, configFile, configDir string, gopts GetOptions) (bool, error) {
	if v, ok := os.LookupEnv(key); ok {
		return cast.ToBool(v)
	}
	raw, err := lookupRaw(key, configFile, configDir, gopts)
	if err != nil {
		return false, err
	}
	if raw == nil {
		if b, ok := gopts.Default.(bool); ok {
			return b, nil
		}
		return false, nil
	}
	return cast.ToBool(raw)
}

// GetRaw retrieves the raw (untyped) value for a key.
func GetRaw(key, configFile, configDir string, gopts GetOptions) (interface{}, error) {
	if v, ok := os.LookupEnv(key); ok {
		return v, nil
	}
	raw, err := lookupRaw(key, configFile, configDir, gopts)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return gopts.Default, nil
	}
	return raw, nil
}

// lookupRaw loads the config and returns the value for key (nil when absent).
func lookupRaw(key, configFile, configDir string, gopts GetOptions) (interface{}, error) {
	opts := Options{
		Create:     false,
		Extensions: gopts.Extensions,
	}
	data, err := LoadConfig(configFile, configDir, opts)
	if err != nil {
		return nil, err
	}

	// Navigate into section if requested
	src := map[string]interface{}(data)
	if gopts.Section != "" {
		if sec, ok := data[gopts.Section]; ok {
			if m, ok := sec.(map[string]interface{}); ok {
				src = m
			}
		}
	}

	v, ok := src[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// ConfigGet — object-oriented API
// ---------------------------------------------------------------------------

// ConfigGet is the object-oriented interface for config-get.
// It lazily discovers and caches the config file on first access.
// Safe for concurrent use.
type ConfigGet struct {
	configFile string
	configDir  string
	section    string
	opts       Options
	autoReload bool

	mu   sync.RWMutex
	path string
	data parser.Data
}

// New creates a new ConfigGet instance. The config file is discovered and
// parsed lazily on the first access.
func New(configFile, configDir string, opts Options) *ConfigGet {
	if configFile == "" {
		configFile = ".env"
	}
	return &ConfigGet{
		configFile: configFile,
		configDir:  configDir,
		opts:       opts,
	}
}

// WithSection returns a new ConfigGet that restricts all Get* calls to the
// given section (for INI / TOML / JSON / YAML nested configs).
func (c *ConfigGet) WithSection(section string) *ConfigGet {
	clone := *c
	clone.section = section
	clone.data = nil
	clone.path = ""
	return &clone
}

// WithAutoReload returns a new ConfigGet that re-reads the file on every access.
func (c *ConfigGet) WithAutoReload() *ConfigGet {
	clone := *c
	clone.autoReload = true
	clone.data = nil
	clone.path = ""
	return &clone
}

// Path returns the resolved path of the discovered config file.
func (c *ConfigGet) Path() (string, error) {
	if err := c.ensureLoaded(); err != nil {
		return "", err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.path, nil
}

// Data returns a copy of the full parsed config.
func (c *ConfigGet) Data() (parser.Data, error) {
	if err := c.ensureLoaded(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(parser.Data, len(c.data))
	for k, v := range c.data {
		out[k] = v
	}
	return out, nil
}

// Reload forces the config file to be re-read from disk.
func (c *ConfigGet) Reload() error {
	c.mu.Lock()
	c.data = nil
	c.path = ""
	c.mu.Unlock()
	return c.ensureLoaded()
}

// Has reports whether key is present in the (possibly sectioned) config.
func (c *ConfigGet) Has(key string) bool {
	if err := c.ensureLoaded(); err != nil {
		return false
	}
	if _, ok := os.LookupEnv(key); ok {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.data[key]
	return ok
}

// String returns the string value for key, or defaultVal if absent.
func (c *ConfigGet) String(key, defaultVal string) string {
	v, err := c.get(key)
	if err != nil || v == nil {
		return defaultVal
	}
	s, err := cast.ToString(v)
	if err != nil {
		return defaultVal
	}
	return s
}

// Int returns the int64 value for key, or defaultVal if absent.
func (c *ConfigGet) Int(key string, defaultVal int64) int64 {
	v, err := c.get(key)
	if err != nil || v == nil {
		return defaultVal
	}
	i, err := cast.ToInt(v)
	if err != nil {
		return defaultVal
	}
	return i
}

// Float returns the float64 value for key, or defaultVal if absent.
func (c *ConfigGet) Float(key string, defaultVal float64) float64 {
	v, err := c.get(key)
	if err != nil || v == nil {
		return defaultVal
	}
	f, err := cast.ToFloat(v)
	if err != nil {
		return defaultVal
	}
	return f
}

// Bool returns the bool value for key, or defaultVal if absent.
func (c *ConfigGet) Bool(key string, defaultVal bool) bool {
	v, err := c.get(key)
	if err != nil || v == nil {
		return defaultVal
	}
	b, err := cast.ToBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

// MustString returns the string value or panics if the key is absent.
func (c *ConfigGet) MustString(key string) string {
	v, err := c.get(key)
	if err != nil {
		panic(fmt.Sprintf("config-get: %v", err))
	}
	if v == nil {
		panic(fmt.Sprintf("config-get: required key %q not found", key))
	}
	s, err := cast.ToString(v)
	if err != nil {
		panic(fmt.Sprintf("config-get: %v", err))
	}
	return s
}

// MustInt returns the int64 value or panics if the key is absent or invalid.
func (c *ConfigGet) MustInt(key string) int64 {
	v, err := c.get(key)
	if err != nil {
		panic(fmt.Sprintf("config-get: %v", err))
	}
	if v == nil {
		panic(fmt.Sprintf("config-get: required key %q not found", key))
	}
	i, err := cast.ToInt(v)
	if err != nil {
		panic(fmt.Sprintf("config-get: %v", err))
	}
	return i
}

// get retrieves the raw value for key, checking env vars first.
func (c *ConfigGet) get(key string) (interface{}, error) {
	if ev, ok := os.LookupEnv(key); ok {
		return ev, nil
	}
	if err := c.ensureLoaded(); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}

// ensureLoaded discovers and parses the config file if not already done.
func (c *ConfigGet) ensureLoaded() error {
	c.mu.RLock()
	loaded := c.data != nil && !c.autoReload
	c.mu.RUnlock()
	if loaded {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-checked locking
	if c.data != nil && !c.autoReload {
		return nil
	}

	path, err := GetConfigFile(c.configFile, c.configDir, c.opts)
	if err != nil {
		return err
	}
	c.path = path

	if !isFile(path) {
		c.data = make(parser.Data)
		return nil
	}

	data, err := parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("config-get: %w", err)
	}

	// Navigate into section if configured
	if c.section != "" {
		if sec, ok := data[c.section]; ok {
			if m, ok := sec.(map[string]interface{}); ok {
				c.data = parser.Data(m)
				return nil
			}
		}
		c.data = make(parser.Data)
		return nil
	}

	c.data = data
	return nil
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ConfigFileNotFoundError is returned when Strict=true and no config file exists.
type ConfigFileNotFoundError struct {
	ConfigFile string
	ConfigDir  string
}

func (e *ConfigFileNotFoundError) Error() string {
	return fmt.Sprintf(
		"config-get: no configuration file found for %q (config_dir=%q)",
		e.ConfigFile, e.ConfigDir,
	)
}

// UnsupportedFormatError is returned when the file extension is not recognized.
type UnsupportedFormatError struct {
	Ext string
}

func (e *UnsupportedFormatError) Error() string {
	return fmt.Sprintf(
		"config-get: unsupported config format %q (supported: %s)",
		e.Ext, strings.Join(SupportedFormats, " "),
	)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

func normalizeExtensions(exts []string) map[string]struct{} {
	m := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		m[strings.ToLower(e)] = struct{}{}
	}
	return m
}
