# devclean 🧹

> **An intelligent, zero-footgun disk cleanup CLI for macOS developers.**

[![Go Report Card](https://goreportcard.com/badge/github.com/sebavidal10/devclean)](https://goreportcard.com/report/github.com/sebavidal10/devclean)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![macOS Compatible](https://img.shields.io/badge/platform-macOS%20(Apple%20Silicon%20%7C%20Intel)-black.svg)](https://apple.com/macos)
[![GitHub Sponsor](https://img.shields.io/badge/Sponsor-GitHub-ea4aaa?logo=github)](https://github.com/sponsors/sebavidal10)

Commercial disk cleaners don't understand developer workflows. They either miss gigabytes of build artifacts or blindly delete active database containers and persistent volumes. 

**devclean** runs entirely in user space, queries native macOS APFS geometry, scans development environments concurrently, and guarantees zero risk to code, configuration, or databases.

---

```text
  ██████╗ ███████╗██╗   ██╗ ██████╗██╗     ███████╗ █████╗ ███╗   ██╗
  ██╔══██╗██╔════╝██║   ██║██╔════╝██║     ██╔════╝██╔══██╗████╗  ██║
  ██║  ██║█████╗  ██║   ██║██║     ██║     █████╗  ███████║██╔██╗ ██║
  ██║  ██║██╔══╝  ╚██╗ ██╔╝██║     ██║     ██╔══╝  ██╔══██║██║╚██╗██║
  ██████╔╝███████╗ ╚████╔╝ ╚██████╗███████╗███████╗██║  ██║██║ ╚████║
  ╚═════╝ ╚══════╝  ╚═══╝   ╚═════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝
v0.1.0 · Intelligent Cleaner for macOS Developers by @sebavidal10

╭──────────────────────────────────────────────────────────────────────╮
│ Punto de montaje: /   Almacenamiento APFS: 460.43 GiB Total          │
│ Espacio Usado:    286.12 GiB [██████████████████░░░░░░░░░░░░]  62.1% │
│ Espacio Libre:    174.31 GiB                                         │
╰──────────────────────────────────────────────────────────────────────╯
```

---

## ✨ Features

- 🛡️ **Zero-Footgun Philosophy:** Operates strictly with user privileges (no `sudo` required). Never touches persistent Docker volumes, `.git/` trees, `.env*` secrets, or SQLite databases.
- ⚡ **Concurrent Engine:** Fast parallel scanning powered by Go goroutines and native macOS APFS syscalls (`unix.Statfs`).
- 🖥️ **Interactive Terminal UI:** Built with [Charm](https://charm.sh)'s **Bubble Tea**, **Lip Gloss**, and **Bubbles**. Inspect granular details before deleting a single byte.
- 🔍 **Granular Drill-down:** Expand any category to see individual paths, ages, and sizes, allowing selective cleanup.
- 📊 **Before & After Disk Telemetry:** Live feedback showing exact gigabytes recovered and updated disk stats.
- 🤖 **CI & Scripting Friendly:** Automatic terminal detection with `--scan` and `--no-tui` flags for automated reporting.

---

## 🛡️ Zero-Footgun Guarantee

| Cleaned Safely 🧹 | Protected at All Costs 🔒 |
|---|---|
| Xcode `DerivedData` & Simulator caches | Source code & `.git/` repositories |
| Inactive `node_modules` (>30 days untouched) | Active project dependencies |
| Global npm cache (`~/.npm`) | Environment files (`.env*`, `.env.local`) |
| Docker dangling layers & build cache | Persistent Docker volumes & running containers |
| macOS user application logs (`~/Library/Logs`) | SQLite & local databases (`*.db`, `*.sqlite*`) |

---

## 🚀 Installation

### Homebrew (Recommended)

```bash
brew tap sebavidal10/tap
brew install devclean
```

### Via `go install`

```bash
go install github.com/sebavidal10/devclean/cmd/devclean@latest
```

### From Source

```bash
# Clone the repository
git clone https://github.com/sebavidal10/devclean.git
cd devclean

# Build the binary
go build -o devclean ./cmd/devclean

# (Optional) Move to your PATH
mv devclean /usr/local/bin/
```

---

## 🎮 Usage

### 1. Interactive TUI Mode (Default)

Simply run `devclean` in any terminal:

```bash
devclean
```

#### Keyboard Controls

| Key | Main Screen (Selection) | Drill-Down View (Details) |
|---|---|---|
| `↑ / k` or `↓ / j` | Navigate categories | Navigate individual items |
| `space` | Toggle category on/off | Toggle single item on/off |
| `a` | Select all / Deselect all | Select all / Deselect all in category |
| `d` or `enter` | Enter Drill-down view | Return to main categories |
| `esc` or `b` | - | Return to main categories |
| `c` | Confirm & execute cleanup | - |
| `q` or `ctrl+c` | Quit immediately | Quit immediately |

---

### 2. Non-Interactive / Scripting Mode

Need a quick report or running in a CI pipeline? Use `--scan` or `--no-tui`:

```bash
devclean --scan
```

Output:

```text
CATEGORÍA / ENTORNO              ARTEFACTO / RUTA                              EDAD       TAMAÑO      
──────────────────────────────────────────────────────────────────────────────────────────────────────
JavaScript & Node.js  • Node.js & NPM
  ✔ Seguro: Removes inactive node_modules (>30 days untouched) and global npm cache.
  ├─ npm-cache                   Global NPM Cache (~/.npm)                     -          1.51 GiB
  ├─ node-modules:.../project-a/node_modules                                   34d ago    170.69 MiB
  ├─ node-modules:.../project-b/node_modules                                   131d ago   55.20 MiB
──────────────────────────────────────────────────────────────────────────────────────────────────────
╭──────────────────────────────────────────────────────────────────────────────╮
│ Total Recuperable: 1.74 GiB a través de 3 objetivos detectados               │
│ Garantía Zero-Footgun activa: .git, .env* y bases de datos están protegidos. │
╰──────────────────────────────────────────────────────────────────────────────╯
```

---

## 🧩 Built-in Plugins

- **Apple & Mobile (`xcode.go`):**
  - Cleans `~/Library/Developer/Xcode/DerivedData` and `~/Library/Developer/CoreSimulator/Caches`.
  - Xcode rebuilds these automatically on your next compile.

- **Containers & Virtualization (`docker.go`):**
  - Runs safe `docker image prune -f` and `docker builder prune -f`.
  - **Strict rule:** Never passes `-a` and **never** calls `docker volume prune`.

- **Web & JavaScript (`node.go`):**
  - Scans global `~/.npm` cache.
  - Recursively finds `node_modules` inside `~/Workspace` with zero modifications in the last 30 days.
  - Automatically skips `.git` directories and nested package directories to prevent infinite recursion or system slowdown.

- **System Diagnostics (`system.go`):**
  - Scans `~/Library/Logs` for orphaned application logs and crash reports larger than 1 MiB.

---

## 🏗️ Architecture

```text
devclean/
├── cmd/
│   └── devclean/
│       └── main.go           # CLI runner & TTY auto-detection
├── internal/
│   ├── disk/
│   │   ├── stat.go           # Native macOS APFS unix.Statfs syscalls
│   │   └── stat_test.go      # Storage geometry tests
│   ├── plugins/
│   │   ├── plugin.go         # CleanerPlugin interface & concurrent Registry
│   │   ├── safety.go         # Zero-Footgun path validator & SafeRemoveAll
│   │   ├── safety_test.go    # Whitelist & forbidden path tests
│   │   ├── xcode.go          # Xcode & Simulator plugin
│   │   ├── docker.go         # Docker dangling layer & builder cache plugin
│   │   ├── node.go           # NPM & inactive node_modules plugin
│   │   └── system.go         # macOS system logs plugin
│   └── tui/
│       ├── model.go          # Bubble Tea state machine (Scan/Select/Drill/Clean/Summary)
│       ├── views.go          # Lip Gloss renderers for all views
│       └── styles.go         # Theme, colors, borders & badges
└── go.mod
```

---

## 🧪 Testing

Run the full test suite across all packages:

```bash
go test -v ./...
```

---

## 🤝 Contributing

Contributions are welcome! If you want to add a plugin (e.g. Gradle, Rust `target/`, Homebrew cache, PyPI):

1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/rust-cargo-plugin`).
3. Implement the `CleanerPlugin` interface in `internal/plugins/`.
4. Ensure all unit tests and safety validations pass (`go test -v ./...`).
5. Open a Pull Request.

---

## 📄 License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.

---

## 👤 Author

Developed with ❤️ by **Sebastián Vidal** ([@sebavidal10](https://github.com/sebavidal10)).
