// File: configget_test.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-04-27
// Description: 
// License: MIT

package configget_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cumulus13/config-get/configget"
	"github.com/cumulus13/config-get/internal/cast"
	"github.com/cumulus13/config-get/internal/parser"
	"github.com/cumulus13/config-get/internal/platform"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func tempEnvFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	return writeFile(t, filepath.Join(dir, ".env"), content)
}

// ---------------------------------------------------------------------------
// platform tests
// ---------------------------------------------------------------------------

func TestBuildCandidatePaths_NoDuplicates(t *testing.T) {
	paths := platform.BuildCandidatePaths(".env", "myapp")
	seen := map[string]bool{}
	for _, p := range paths {
		key := filepath.Clean(p)
		if seen[key] {
			t.Errorf("duplicate candidate: %s", p)
		}
		seen[key] = true
	}
}

func TestBuildCandidatePaths_ContainsEnvVariant(t *testing.T) {
	paths := platform.BuildCandidatePaths("myapp.ini", "myapp")
	found := false
	for _, p := range paths {
		if filepath.Base(p) == ".env" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected .env variant in candidates")
	}
}

func TestBuildCandidatePaths_ContainsStemVariants(t *testing.T) {
	paths := platform.BuildCandidatePaths("myapp.toml", "")
	names := map[string]bool{}
	for _, p := range paths {
		names[filepath.Base(p)] = true
	}
	for _, want := range []string{"myapp.toml", "myapp.ini", "myapp.json"} {
		if !names[want] {
			t.Errorf("expected %q in candidate names", want)
		}
	}
}

func TestBuildCandidatePaths_ConfigDirAppears(t *testing.T) {
	paths := platform.BuildCandidatePaths("app.env", "myservice")
	found := false
	for _, p := range paths {
		if strings.Contains(p, "myservice") {
			found = true
			break
		}
	}
	if !found {
		t.Error("config_dir 'myservice' not found in any candidate path")
	}
}

func TestBuildCandidatePaths_NotEmpty(t *testing.T) {
	paths := platform.BuildCandidatePaths(".env", "")
	if len(paths) == 0 {
		t.Error("expected non-empty candidate list")
	}
}

// ---------------------------------------------------------------------------
// cast tests
// ---------------------------------------------------------------------------

func TestCastToString(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{"hello", "hello"},
		{42, "42"},
		{3.14, "3.14"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
	}
	for _, c := range cases {
		got, err := cast.ToString(c.in)
		if err != nil {
			t.Errorf("ToString(%v): unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ToString(%v) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestCastToInt(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int64
	}{
		{"42", 42},
		{"0", 0},
		{"-7", -7},
		{int(10), 10},
		{float64(3.9), 3},
		{true, 1},
		{false, 0},
	}
	for _, c := range cases {
		got, err := cast.ToInt(c.in)
		if err != nil {
			t.Errorf("ToInt(%v): unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ToInt(%v) = %d; want %d", c.in, got, c.want)
		}
	}
}

func TestCastToInt_Error(t *testing.T) {
	_, err := cast.ToInt("not-a-number")
	if err == nil {
		t.Error("expected error for non-numeric string")
	}
}

func TestCastToBool(t *testing.T) {
	truthy := []interface{}{"1", "true", "yes", "on", "TRUE", "YES", true, 1}
	for _, v := range truthy {
		got, err := cast.ToBool(v)
		if err != nil {
			t.Errorf("ToBool(%v): unexpected error: %v", v, err)
		}
		if !got {
			t.Errorf("ToBool(%v) = false; want true", v)
		}
	}

	falsy := []interface{}{"0", "false", "no", "off", "", false, 0}
	for _, v := range falsy {
		got, err := cast.ToBool(v)
		if err != nil {
			t.Errorf("ToBool(%v): unexpected error: %v", v, err)
		}
		if got {
			t.Errorf("ToBool(%v) = true; want false", v)
		}
	}
}

// ---------------------------------------------------------------------------
// parser tests
// ---------------------------------------------------------------------------

func TestParseEnv_Basic(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, ".env"), "KEY=value\nANOTHER=123\n")
	data, err := parser.ParseEnv(path)
	if err != nil {
		t.Fatalf("ParseEnv: %v", err)
	}
	assertEqual(t, "value", data["KEY"])
	assertEqual(t, "123", data["ANOTHER"])
}

func TestParseEnv_CommentsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, ".env"), "# comment\nKEY=val\n")
	data, err := parser.ParseEnv(path)
	if err != nil {
		t.Fatalf("ParseEnv: %v", err)
	}
	if _, ok := data["# comment"]; ok {
		t.Error("comment should not be a key")
	}
	assertEqual(t, "val", data["KEY"])
}

func TestParseEnv_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, ".env"), `A="hello world"`+"\n"+`B='single'`+"\n")
	data, err := parser.ParseEnv(path)
	if err != nil {
		t.Fatalf("ParseEnv: %v", err)
	}
	assertEqual(t, "hello world", data["A"])
	assertEqual(t, "single", data["B"])
}

func TestParseEnv_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, ".env"), "URL=http://example.com?a=1&b=2\n")
	data, err := parser.ParseEnv(path)
	if err != nil {
		t.Fatalf("ParseEnv: %v", err)
	}
	assertEqual(t, "http://example.com?a=1&b=2", data["URL"])
}

func TestParseJSON(t *testing.T) {
	dir := t.TempDir()
	content, _ := json.Marshal(map[string]interface{}{"port": 8080, "host": "localhost"})
	path := writeFile(t, filepath.Join(dir, "app.json"), string(content))
	data, err := parser.ParseJSON(path)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if data["host"] != "localhost" {
		t.Errorf("host = %v; want localhost", data["host"])
	}
}

func TestParseYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, "app.yaml"), "host: db.local\nport: 5432\n")
	data, err := parser.ParseYAML(path)
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	assertEqual(t, "db.local", data["host"])
}

func TestParseTOML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, "app.toml"), "[app]\nhost = \"localhost\"\nport = 9090\n")
	data, err := parser.ParseToml(path)
	if err != nil {
		t.Fatalf("ParseToml: %v", err)
	}
	app, ok := data["app"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected [app] section, got %T", data["app"])
	}
	assertEqual(t, "localhost", app["host"])
}

func TestParseIni(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, "app.ini"), "[database]\nhost = myhost\nport = 3306\n")
	data, err := parser.ParseIni(path)
	if err != nil {
		t.Fatalf("ParseIni: %v", err)
	}
	db, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected [database] section")
	}
	assertEqual(t, "myhost", db["host"])
}

func TestParseFile_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, "app.xml"), "<root/>")
	_, err := parser.ParseFile(path)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// ---------------------------------------------------------------------------
// configget package tests
// ---------------------------------------------------------------------------

func TestGetConfigFile_FindsExisting(t *testing.T) {
	dir := t.TempDir()
	envPath := writeFile(t, filepath.Join(dir, ".env"), "K=V")

	// Override home so candidates include our temp dir
	overrideHome(t, dir)

	path, err := configget.GetConfigFile(".env", "", configget.Options{Create: false})
	if err != nil {
		t.Fatalf("GetConfigFile: %v", err)
	}
	if path != envPath {
		t.Errorf("got %s; want %s", path, envPath)
	}
}

func TestGetConfigFile_ReturnsPath(t *testing.T) {
	path, err := configget.GetConfigFile(".env", "", configget.Options{Create: false})
	if err != nil {
		t.Fatalf("GetConfigFile: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestGetConfigFile_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	overrideHome(t, dir)
	path, err := configget.GetConfigFile("myapp.env", "unique_test_subdir", configget.Options{Create: true})
	if err != nil {
		t.Fatalf("GetConfigFile: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Errorf("expected directory to be created: %s", filepath.Dir(path))
	}
}

func TestLoadConfig_EnvFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "DB=sqlite\nDEBUG=true\n")
	overrideHome(t, dir)

	data, err := configget.LoadConfig(".env", "", configget.Options{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	assertEqual(t, "sqlite", data["DB"])
	assertEqual(t, "true", data["DEBUG"])
}

func TestLoadConfig_StrictError(t *testing.T) {
	dir := t.TempDir()
	overrideHome(t, dir) // empty dir → no config files

	_, err := configget.LoadConfig("nonexistent.env", "nonexistent_dir", configget.Options{
		Strict: true,
		Create: false,
	})
	if err == nil {
		t.Error("expected error in strict mode")
	}
}

func TestLoadConfig_EmptyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	overrideHome(t, dir)

	data, err := configget.LoadConfig("nonexistent.env", "nonexistent_dir", configget.Options{Create: false})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %v", data)
	}
}

func TestGetString_EnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "MY_KEY=from_file\n")
	overrideHome(t, dir)
	t.Setenv("MY_KEY", "from_env")

	v, err := configget.GetString("MY_KEY", ".env", "", configget.GetOptions{})
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "from_env" {
		t.Errorf("got %q; want %q", v, "from_env")
	}
}

func TestGetString_FromFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "FILE_KEY=hello\n")
	overrideHome(t, dir)
	os.Unsetenv("FILE_KEY")

	v, err := configget.GetString("FILE_KEY", ".env", "", configget.GetOptions{})
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "hello" {
		t.Errorf("got %q; want %q", v, "hello")
	}
}

func TestGetString_Default(t *testing.T) {
	dir := t.TempDir()
	overrideHome(t, dir)
	os.Unsetenv("ABSENT_KEY")

	v, err := configget.GetString("ABSENT_KEY", ".env", "", configget.GetOptions{Default: "fallback"})
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if v != "fallback" {
		t.Errorf("got %q; want %q", v, "fallback")
	}
}

func TestGetInt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "PORT=9000\n")
	overrideHome(t, dir)
	os.Unsetenv("PORT")

	v, err := configget.GetInt("PORT", ".env", "", configget.GetOptions{})
	if err != nil {
		t.Fatalf("GetInt: %v", err)
	}
	if v != 9000 {
		t.Errorf("got %d; want 9000", v)
	}
}

func TestGetBool_True(t *testing.T) {
	t.Setenv("DEBUG_FLAG", "yes")
	v, err := configget.GetBool("DEBUG_FLAG", ".env", "", configget.GetOptions{})
	if err != nil {
		t.Fatalf("GetBool: %v", err)
	}
	if !v {
		t.Error("expected true")
	}
}

func TestGetBool_False(t *testing.T) {
	t.Setenv("DEBUG_FLAG", "0")
	v, err := configget.GetBool("DEBUG_FLAG", ".env", "", configget.GetOptions{})
	if err != nil {
		t.Fatalf("GetBool: %v", err)
	}
	if v {
		t.Error("expected false")
	}
}

func TestGetFloat(t *testing.T) {
	t.Setenv("RATIO", "0.95")
	v, err := configget.GetFloat("RATIO", ".env", "", configget.GetOptions{})
	if err != nil {
		t.Fatalf("GetFloat: %v", err)
	}
	if fmt.Sprintf("%.2f", v) != "0.95" {
		t.Errorf("got %f; want 0.95", v)
	}
}

// ---------------------------------------------------------------------------
// ConfigGet struct tests
// ---------------------------------------------------------------------------

func TestConfigGet_String(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "KEY=value\n")
	overrideHome(t, dir)
	os.Unsetenv("KEY")

	cfg := configget.New(".env", "", configget.Options{})
	got := cfg.String("KEY", "default")
	if got != "value" {
		t.Errorf("got %q; want %q", got, "value")
	}
}

func TestConfigGet_Int(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "PORT=1234\n")
	overrideHome(t, dir)
	os.Unsetenv("PORT")

	cfg := configget.New(".env", "", configget.Options{})
	got := cfg.Int("PORT", 0)
	if got != 1234 {
		t.Errorf("got %d; want 1234", got)
	}
}

func TestConfigGet_Bool(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "ENABLED=true\n")
	overrideHome(t, dir)
	os.Unsetenv("ENABLED")

	cfg := configget.New(".env", "", configget.Options{})
	got := cfg.Bool("ENABLED", false)
	if !got {
		t.Error("expected true")
	}
}

func TestConfigGet_Has(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "PRESENT=1\n")
	overrideHome(t, dir)
	os.Unsetenv("PRESENT")
	os.Unsetenv("ABSENT")

	cfg := configget.New(".env", "", configget.Options{})
	if !cfg.Has("PRESENT") {
		t.Error("expected PRESENT to be found")
	}
	if cfg.Has("ABSENT") {
		t.Error("expected ABSENT to be absent")
	}
}

func TestConfigGet_MustString_Panics(t *testing.T) {
	dir := t.TempDir()
	overrideHome(t, dir)
	os.Unsetenv("MISSING_KEY")

	cfg := configget.New(".env", "", configget.Options{})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()
	cfg.MustString("MISSING_KEY")
}

func TestConfigGet_Data(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "A=1\nB=2\n")
	overrideHome(t, dir)

	cfg := configget.New(".env", "", configget.Options{})
	data, err := cfg.Data()
	if err != nil {
		t.Fatalf("Data: %v", err)
	}
	if len(data) < 2 {
		t.Errorf("expected >=2 keys, got %d", len(data))
	}
}

func TestConfigGet_Reload(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, filepath.Join(dir, ".env"), "VAL=first\n")
	overrideHome(t, dir)
	os.Unsetenv("VAL")

	cfg := configget.New(".env", "", configget.Options{})
	if v := cfg.String("VAL", ""); v != "first" {
		t.Errorf("initial: got %q; want %q", v, "first")
	}

	os.WriteFile(path, []byte("VAL=second\n"), 0o644)
	if err := cfg.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if v := cfg.String("VAL", ""); v != "second" {
		t.Errorf("after reload: got %q; want %q", v, "second")
	}
}

func TestConfigGet_WithSection_JSON(t *testing.T) {
	dir := t.TempDir()
	content, _ := json.Marshal(map[string]interface{}{
		"database": map[string]interface{}{"host": "db.local", "port": 5432},
	})
	writeFile(t, filepath.Join(dir, "app.json"), string(content))
	overrideHome(t, dir)
	os.Unsetenv("host")

	cfg := configget.New("app.json", "", configget.Options{}).WithSection("database")
	if v := cfg.String("host", ""); v != "db.local" {
		t.Errorf("got %q; want %q", v, "db.local")
	}
}

func TestConfigGet_EnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "OVER=from_file\n")
	overrideHome(t, dir)
	t.Setenv("OVER", "from_env")

	cfg := configget.New(".env", "", configget.Options{})
	if v := cfg.String("OVER", ""); v != "from_env" {
		t.Errorf("got %q; want from_env", v)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertEqual(t *testing.T, want, got interface{}) {
	t.Helper()
	ws := fmt.Sprintf("%v", want)
	gs := fmt.Sprintf("%v", got)
	if ws != gs {
		t.Errorf("got %q; want %q", gs, ws)
	}
}

// overrideHome temporarily sets HOME / USERPROFILE so platform.BaseDirs()
// returns our temp dir as the first candidate.
func overrideHome(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
		t.Setenv("USERPROFILE", dir)
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("HOME", dir)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	}
}
