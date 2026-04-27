# config-get (Go)

> Cross-platform configuration file discovery, loading, and value retrieval for Go.

**Author:** Hadi Cahyadi \<cumulus13@gmail.com\>  
**Homepage:** [https://github.com/cumulus13/go-config-get](https://github.com/cumulus13/go-config-get)  
**Go module:** `github.com/cumulus13/go-config-get`

---

## Features

- 🔍 **Auto-discovery** — searches standard platform locations automatically
- 📄 **Multi-format** — `.env`, `.ini`, `.toml`, `.json`, `.yml` / `.yaml`
- 🌍 **Env-var precedence** — `os.Environ` always wins over file values
- 🖥 **Cross-platform** — Windows (`%APPDATA%`), macOS / Linux (XDG-aware)
- 🪶 **Near-zero dependencies** — only `github.com/BurntSushi/toml`; all other formats use stdlib
- 🔒 **Concurrency-safe** — `ConfigGet` struct uses `sync.RWMutex`
- 🔧 **CLI tool** — `config-get` binary included
- 🐹 **Go 1.22+**

---

## Installation

```bash
go get github.com/cumulus13/go-config-get/configget
```

Install the CLI:

```bash
go install github.com/cumulus13/go-config-get/cmd/config-get@latest
```

---

## Quick Start

### Functional API

```go
import "github.com/cumulus13/go-config-get/configget"

// Discover the config file path
path, err := configget.GetConfigFile("myapp.toml", "myapp", configget.Options{Create: true})

// Load the entire config
data, err := configget.LoadConfig("myapp.toml", "myapp", configget.Options{})

// Typed getters (env vars take precedence)
host, err := configget.GetString("DB_HOST", "myapp.toml", "myapp", configget.GetOptions{Default: "localhost"})
port, err := configget.GetInt("DB_PORT", "myapp.toml", "myapp", configget.GetOptions{Default: 5432})
debug, err := configget.GetBool("DEBUG",  "myapp.toml", "myapp", configget.GetOptions{Default: false})
ratio, err := configget.GetFloat("RATIO", ".env", "",       configget.GetOptions{Default: 1.0})
```

### Object-Oriented API

```go
// Create once; config is discovered and parsed lazily on first access.
cfg := configget.New("myapp.toml", "myapp", configget.Options{Create: true})

// Typed getters with inline defaults — never return errors
host  := cfg.String("DB_HOST", "localhost")
port  := cfg.Int("DB_PORT", 5432)
debug := cfg.Bool("DEBUG", false)
ratio := cfg.Float("RATIO", 1.0)

// Panic on missing required keys
secret := cfg.MustString("API_SECRET")

// Navigate into a section (for INI / TOML / JSON / YAML)
db := cfg.WithSection("database")
dbHost := db.String("host", "localhost")

// Check presence
if cfg.Has("FEATURE_FLAG") { ... }

// Full config as map
data, err := cfg.Data()

// Reload from disk (e.g. after SIGHUP)
err = cfg.Reload()

// Auto-reload on every access
cfg = cfg.WithAutoReload()

// Resolved path
path, err := cfg.Path()
```

---

## Search Order

### Linux / macOS (XDG-aware)

| Priority | Path template |
|---|---|
| 1 | `$HOME/<configDir>/<file>` |
| 2 | `$XDG_CONFIG_HOME/<configDir>/<file>` (or `$HOME/.config/…`) |
| 3 | `/etc/<configDir>/<file>` |
| 4 | `$HOME/<file>` |
| 5 | `$XDG_CONFIG_HOME/<file>` |
| 6 | `/etc/<file>` |

### Windows

| Priority | Path template |
|---|---|
| 1 | `%APPDATA%\<configDir>\<file>` |
| 2 | `%USERPROFILE%\<configDir>\<file>` |
| 3 | `%LOCALAPPDATA%\<configDir>\<file>` |
| 4–6 | Same bases without `<configDir>` |

Within each location, extensions are tried in order:  
`.env` → `.ini` → `.toml` → `.json` → `.yml` → `.yaml`

---

## Supported Formats

### `.env`
```dotenv
DB_HOST=localhost
DB_PORT=5432
DEBUG=true
API_KEY="my secret key"
```

### `.ini`
```ini
[database]
host = localhost
port = 5432

[app]
debug = true
```
```go
host := cfg.WithSection("database").String("host", "localhost")
```

### `.toml`
```toml
[database]
host = "localhost"
port = 5432
```

### `.json`
```json
{
  "database": { "host": "localhost", "port": 5432 },
  "debug": true
}
```

### `.yml` / `.yaml`
```yaml
database:
  host: localhost
  port: 5432
debug: true
```

---

## Options Reference

### `Options`

| Field | Type | Default | Description |
|---|---|---|---|
| `Create` | `bool` | `false` | Create parent dir if it doesn't exist |
| `Extensions` | `[]string` | `nil` | Restrict search to these extensions |
| `Strict` | `bool` | `false` | Error if no config file found |

### `GetOptions`

| Field | Type | Default | Description |
|---|---|---|---|
| `Default` | `interface{}` | `nil` | Fallback value |
| `Section` | `string` | `""` | Navigate into this section |
| `Extensions` | `[]string` | `nil` | Restrict file search |

---

## CLI Reference

```bash
# Find and print the config file path
config-get --locate --config myapp.toml --dir myapp

# Retrieve a string value
config-get DB_HOST --config myapp.toml --dir myapp --default localhost

# Retrieve with type casting
config-get PORT --cast int --default 8080
config-get RATIO --cast float
config-get DEBUG --cast bool

# Navigate into a section
config-get host --section database --config myapp.ini

# Dump the full config as JSON
config-get --dump --config myapp.toml

# Restrict search to specific extensions
config-get DB_HOST --ext .toml,.ini

# Enable debug logging
config-get DB_HOST -v

# Version
config-get --version
```

---

## Package Structure

```
github.com/cumulus13/go-config-get/
├── configget/           # Public API (ConfigGet struct + functional helpers)
│   ├── configget.go
│   └── configget_test.go
├── cmd/config-get/      # CLI binary
│   └── main.go
├── internal/
│   ├── cast/            # Type coercion (string→int/float/bool)
│   ├── parser/          # Format parsers (.env .ini .toml .json .yaml)
│   └── platform/        # Cross-platform base directory resolution
├── go.mod
├── go.sum
└── README.md
```

---

## Development

```bash
git clone https://github.com/cumulus13/go-config-get
cd config-get
go test ./...
go build ./cmd/config-get
```

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/BurntSushi/toml` | TOML parsing |
| stdlib only | `.env`, `.ini`, `.json`, `.yml` parsing |

---

## License

MIT © 2026 Hadi Cahyadi


## 👤 Author
        
[Hadi Cahyadi](mailto:cumulus13@gmail.com)
    

[![Buy Me a Coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/cumulus13)

[![Donate via Ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/cumulus13)
 
[Support me on Patreon](https://www.patreon.com/cumulus13)