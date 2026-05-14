package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all user-configurable settings.
type Config struct {
	// Path to the encrypted database file.
	DBPath string `yaml:"db_path"`
	// Seconds before clipboard is automatically cleared after a copy.
	ClipboardTimeout int `yaml:"clipboard_timeout"`

	// Background color for the selected row.
	HighlightColor string `yaml:"highlight_color"`
	// Foreground (text) color for the selected row.
	HighlightForeground string `yaml:"highlight_foreground"`
	// Color of the ▸ cursor and prompt text.
	AccentColor string `yaml:"accent_color"`
	// Color for success/confirmation messages.
	OKColor string `yaml:"ok_color"`
	// Color for error messages.
	ErrorColor string `yaml:"error_color"`
	// Color for dim/secondary text (usernames, separators, hints).
	DimColor string `yaml:"dim_color"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		DBPath:              filepath.Join(home, ".secretly.db"),
		ClipboardTimeout:    10,
		HighlightColor:      "#bd15b2",
		HighlightForeground: "#ffffff",
		AccentColor:         "#bd15b2",
		OKColor:             "#50c878",
		ErrorColor:          "#ff5555",
		DimColor:            "#606060",
	}
}

// Path returns $XDG_CONFIG_HOME/secretly/config.yaml or ~/.config/secretly/config.yaml.
func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "secretly", "config.yaml")
}

// Load reads the config file, falling back to defaults for missing keys.
// It is not an error if the file does not exist.
func Load() (Config, error) {
	cfg := Default()
	path := Path()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}

	// expand ~ in db_path
	if strings.HasPrefix(cfg.DBPath, "~/") {
		home, _ := os.UserHomeDir()
		cfg.DBPath = filepath.Join(home, cfg.DBPath[2:])
	}

	if cfg.ClipboardTimeout < 1 {
		return cfg, fmt.Errorf("config: clipboard_timeout must be a positive integer")
	}

	return cfg, nil
}

// WriteDefault writes a commented default config file if none exists.
func WriteDefault() error {
	path := Path()
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	cfg := Default()
	content := fmt.Sprintf(`# secretly configuration
# All settings are optional; commented-out values show the defaults.
# Colors accept a 256-color number ("211") or a hex string ("#c75f87").

# Path to the encrypted database file.
# db_path: %s

# Seconds before the clipboard is automatically cleared after copying a password.
# clipboard_timeout: %d

# Selected row background color.
# highlight_color: %s

# Selected row text color.
# highlight_foreground: %s

# Cursor (▸) and prompt color.
# accent_color: %s

# Success message color.
# ok_color: %s

# Error message color.
# error_color: %s

# Dim/secondary text color (usernames, hints, separators).
# dim_color: %s
`, cfg.DBPath, cfg.ClipboardTimeout,
		cfg.HighlightColor, cfg.HighlightForeground,
		cfg.AccentColor, cfg.OKColor, cfg.ErrorColor, cfg.DimColor)

	return os.WriteFile(path, []byte(content), 0600)
}
