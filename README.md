# secretly

A terminal-based password manager with vim-style keybindings encryption. Built in Go using [bubbletea](https://github.com/charmbracelet/bubbletea).

Requires a [Nerd Font](https://www.nerdfonts.com/) in your terminal for icons.

---

## Features

- Encrypted database (XChaCha20-Poly1305 + scrypt)
- Vim-style navigation (`j`/`k`, `g`/`G`, etc.)
- Clipboard copy with auto-clear
- Entry preview overlay with masked password
- Form-based entry editing (name, username, password, notes)
- Fully configurable colors via YAML config
- Atomic writes — a crash never corrupts your database
- File locking — safe to keep the DB on a synced folder

---

## Installation

```sh
brew tap ochcaroline/tap
```

```sh
brew install ochcaroline/tap/secretly
```

```sh
git clone https://github.com/ochcaroline/secretly
cd secretly
go build -o secretly ./cmd/secretly
```

Move the binary somewhere on your `$PATH`:

```sh
mv secretly /usr/local/bin/
```

---

## Usage

```sh
secretly
```

On first run, a default config file is written and you are prompted to set a master password. The database is created on first save.

---

## Keybindings

### Navigation

| Key       | Action               |
| --------- | -------------------- |
| `j` / `k` | Move down / up       |
| `g` / `G` | Jump to top / bottom |

### Actions

| Key | Action                  |
| --- | ----------------------- |
| `a` | Add entry               |
| `d` | Delete selected item    |
| `r` | Rename selected item    |
| `e` | Edit entry (opens form) |
| `P` | Change master password  |

### Entry form (add / edit)

| Key               | Action                    |
| ----------------- | ------------------------- |
| `tab` / `↓`       | Next field                |
| `shift+tab` / `↑` | Previous field            |
| `enter`           | Next field (save on last) |
| `ctrl+s`          | Save from any field       |
| `esc`             | Cancel                    |

### Clipboard & Preview

| Key         | Action                     |
| ----------- | -------------------------- |
| `y`         | Copy password to clipboard |
| `Y`         | Copy username to clipboard |
| `p`         | Preview entry (masked)     |
| `p` (again) | Reveal password in preview |

### Misc

| Key      | Action                         |
| -------- | ------------------------------ |
| `?`      | Toggle help overlay            |
| `q`      | Quit (asks for confirmation)   |
| `ctrl+c` | Quit immediately (saves first) |

### Icons

| Icon | Meaning                        |
| ---- | ------------------------------ |
| `󰌋`  | Entry with username + password |
| `󰀉`  | Entry with username only       |
| `󰌾`  | Entry with password only       |

---

## Configuration

Config is written on first run to:

| OS            | Path                                                                                |
| ------------- | ----------------------------------------------------------------------------------- |
| macOS / Linux | `$XDG_CONFIG_HOME/secretly/config.yaml` (default: `~/.config/secretly/config.yaml`) |

```yaml
# Path to the encrypted database file.
# Move this to a synced folder (OneDrive, Dropbox, etc.) to share across machines.
db_path: ~/.secretly.db

# Seconds before clipboard is automatically cleared after copying a password.
clipboard_timeout: 10

# ── Colors ─────────────────────────────────────────────────────────────────────
# Accepts hex (#rrggbb) or 256-color terminal codes ("205", "196", etc.)

# Background color of the selected row.
highlight_color: "#bd15b2"

# Text color of the selected row.
highlight_foreground: "#ffffff"

# Color of the ▸ cursor, prompt text, and form borders.
accent_color: "#bd15b2"

# Color for success messages.
ok_color: "#50c878"

# Color for error messages.
error_color: "#ff5555"

# Color for secondary text (usernames, hints, separators).
dim_color: "#606060"
```

---

## Database location & syncing

The default database path is `~/.secretly.db`. You can move it to a cloud-synced folder by changing `db_path` in the config:

```yaml
db_path: /Users/you/OneDrive/secretly.db
```

**Important:** only open secretly on one machine at a time when the DB is synced. The app uses a file lock to prevent two instances on the _same_ machine from conflicting, but cloud sync has no cross-machine locking. Opening on two machines simultaneously can result in a sync conflict and lost changes.

The lock file is always stored locally in the OS temp directory (e.g. `/tmp/secretly-<hash>.lock`) and is never synced.

---

## Security

### Encryption

Every save encrypts the entire database as a single authenticated blob:

```
[version: 1 byte][salt: 32 bytes][nonce: 24 bytes][ciphertext + auth tag: N bytes]
```

| Primitive  | Detail                                                                         |
| ---------- | ------------------------------------------------------------------------------ |
| **Cipher** | XChaCha20-Poly1305 (256-bit key, 192-bit nonce)                                |
| **Salt**   | 32 bytes from `crypto/rand`, unique per save                                   |
| **AAD**    | Salt is authenticated — any tampering with it is detected and decryption fails |

### Key derivation cost

scrypt at N=131072 requires ~128 MB of RAM and ~0.5 s per attempt on modern hardware. This makes GPU/ASIC brute-force attacks extremely expensive:

| Password type              | Estimated GPU attempts/sec | Time to exhaust space |
| -------------------------- | -------------------------- | --------------------- |
| 6-char lowercase           | ~2 000                     | hours–days            |
| 8-char mixed case + digits | ~2 000                     | decades               |
| 4 random diceware words    | ~2 000                     | millions of years     |
| 12+ random characters      | ~2 000                     | effectively infinite  |

The memory-hard property of scrypt means a $5 000 GPU rig is limited to ~2 000 guesses/second, compared to billions/second against a plain SHA-256 hash.

**The encryption is not the weak link — your master password is.**  
Use at least 4 random words (diceware) or 12+ random characters.

### What is protected

- The database file at rest: fully encrypted, indistinguishable from random bytes without the master password
- Writes: atomic (temp file → fsync → rename) — a crash or power loss never leaves a partial or corrupt file
- Clipboard: auto-cleared after a configurable timeout (default 10 s)
- Key material: zeroed in memory immediately after use with `defer`

### Known limitations / trade-offs

| Limitation                        | Notes                                                                                                                                                                                                                                     |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Secrets live in Go strings**    | Go strings are immutable and GC'd at unknown times. Passwords may persist in RAM briefly after use and could in theory be swapped to disk. Mitigating this fully requires OS-level memory locking (`mlock`), which is not implemented.    |
| **Clipboard window**              | Any application can read the clipboard during the 10-second auto-clear window. This is inherent to how OS clipboards work.                                                                                                                |
| **No master password recovery**   | There is no recovery mechanism. If you forget your master password, the database is unrecoverable.                                                                                                                                        |
| **No cross-machine file locking** | Cloud sync providers do not propagate advisory locks. Only open the database on one machine at a time.                                                                                                                                    |
| **OneDrive version history**      | Every save creates a new version in OneDrive's history. The versions are encrypted blobs (safe), but they live in Microsoft's cloud indefinitely ("harvest now, decrypt later" risk if XChaCha20 is ever broken — considered negligible). |

---

## Dependencies

| Package                                                               | Purpose                     |
| --------------------------------------------------------------------- | --------------------------- |
| [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | TUI framework               |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss)   | Terminal styling            |
| [atotto/clipboard](https://github.com/atotto/clipboard)               | Clipboard access            |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)         | scrypt + XChaCha20-Poly1305 |
| [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)               | Config file parsing         |
