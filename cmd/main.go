package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ochcaroline/secretly/internal/config"
	"github.com/ochcaroline/secretly/internal/store"
	"github.com/ochcaroline/secretly/internal/ui"
)

func main() {
	if err := config.WriteDefault(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not write default config:", err)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	lock, err := store.AcquireLock(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "secretly:", err)
		os.Exit(1)
	}
	defer lock.Close()

	p := tea.NewProgram(ui.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
