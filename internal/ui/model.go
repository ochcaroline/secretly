package ui

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ochcaroline/secretly/internal/config"
	"github.com/ochcaroline/secretly/internal/store"
)

type styles struct {
	header   lipgloss.Style
	dim      lipgloss.Style
	sep      lipgloss.Style
	ok       lipgloss.Style
	errStyle lipgloss.Style
	prompt   lipgloss.Style
	cursor   lipgloss.Style
	bold     lipgloss.Style
}

func buildStyles(cfg config.Config) styles {
	dim := lipgloss.Color(cfg.DimColor)
	accent := lipgloss.Color(cfg.AccentColor)
	return styles{
		header:   lipgloss.NewStyle().Bold(true),
		dim:      lipgloss.NewStyle().Foreground(dim),
		sep:      lipgloss.NewStyle().Foreground(dim),
		ok:       lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.OKColor)),
		errStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.ErrorColor)),
		prompt:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		cursor:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		bold:     lipgloss.NewStyle().Bold(true),
	}
}

// entryIcon returns a Nerd Font icon reflecting what fields the entry has.
//
// 󰌋  user+pass   󰀉  user only   󰌾  pass only
func entryIcon(e store.Entry, styled func(lipgloss.Style) lipgloss.Style, st styles) string {
	var icon string
	switch {
	case e.Username != "" && e.Password != "":
		icon = "󰌋 "
	case e.Username != "":
		icon = "󰀉 "
	default:
		icon = "󰌾 "
	}
	return styled(st.dim).Render(icon)
}

type appState int

const (
	stateUnlock appState = iota
	stateBrowse
	stateInput
	stateForm
)

type inputStep int

const (
	stepNone inputStep = iota
	stepRename
	stepConfirmDelete
	stepConfirmQuit
	stepChangeMasterNew
	stepChangeMasterConfirm
)

type formField struct {
	label  string
	value  string
	masked bool
}

const (
	formFieldName = iota
	formFieldUser
	formFieldPass
	formFieldNotes
	formFieldCount
)

type Model struct {
	cfg    config.Config
	st     styles
	dbPath string
	db     *store.DB
	master string

	cursor int
	offset int

	state appState
	step  inputStep

	// single-line input (rename, master password, confirmations)
	input         string
	pendingMaster string

	// form overlay (entry add / edit)
	formTitle    string
	formFields   [formFieldCount]formField
	formFocus    int
	formIsEdit   bool
	formEditIdx  int
	formPassWarn bool

	// search
	searchQuery     string
	searching       bool
	filteredIndices []int

	width       int
	height      int
	showHelp    bool
	showPreview bool
	previewMask bool

	status  string
	isError bool
}

func NewModel(cfg config.Config) Model {
	return Model{
		cfg:    cfg,
		st:     buildStyles(cfg),
		dbPath: cfg.DBPath,
		state:  stateUnlock,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) clipboardTTL() time.Duration {
	return time.Duration(m.cfg.ClipboardTimeout) * time.Second
}

func (m *Model) entries() []store.Entry { return m.db.Entries }
func (m *Model) count() int             { return len(m.db.Entries) }

func (m *Model) setStatus(msg string, isErr bool) {
	m.status = msg
	m.isError = isErr
}

func (m *Model) saveDB() {
	if err := store.Save(m.db, m.dbPath, m.master); err != nil {
		m.setStatus("save error: "+err.Error(), true)
	}
}

// listHeight returns how many rows fit on screen.
// chrome = top-pad(1) + header(1) + sep(1) + sep(1) + status(1) + hint(1) = 6
func (m *Model) listHeight() int {
	if h := m.height - 6; h >= 1 {
		return h
	}
	return 1
}

func (m *Model) scrollToCursor() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
}

// updateFilter recomputes filteredIndices from searchQuery.
func (m *Model) updateFilter() {
	if m.searchQuery == "" {
		m.filteredIndices = nil
		return
	}
	q := strings.ToLower(m.searchQuery)
	m.filteredIndices = make([]int, 0)
	for i, e := range m.db.Entries {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Username), q) ||
			strings.Contains(strings.ToLower(e.Notes), q) {
			m.filteredIndices = append(m.filteredIndices, i)
		}
	}
}

func (m *Model) visibleEntries() []store.Entry {
	if m.searchQuery == "" {
		return m.db.Entries
	}
	out := make([]store.Entry, len(m.filteredIndices))
	for i, idx := range m.filteredIndices {
		out[i] = m.db.Entries[idx]
	}
	return out
}

func (m *Model) visibleCount() int {
	if m.searchQuery == "" {
		return m.count()
	}
	return len(m.filteredIndices)
}

// realCursorIndex maps the cursor position in the visible list to the real
// index in db.Entries. Returns -1 if the cursor is out of range.
func (m *Model) realCursorIndex() int {
	if m.searchQuery == "" {
		return m.cursor
	}
	if m.cursor < len(m.filteredIndices) {
		return m.filteredIndices[m.cursor]
	}
	return -1
}

// sortEntries sorts db.Entries alphabetically by name (case-insensitive).
func (m *Model) sortEntries() {
	sort.Slice(m.db.Entries, func(i, j int) bool {
		return strings.ToLower(m.db.Entries[i].Name) < strings.ToLower(m.db.Entries[j].Name)
	})
}

// setCursorToName moves the cursor to the first visible entry whose name
// matches exactly, then scrolls it into view.
func (m *Model) setCursorToName(name string) {
	for i, e := range m.visibleEntries() {
		if e.Name == name {
			m.cursor = i
			m.scrollToCursor()
			return
		}
	}
	if m.cursor >= m.visibleCount() {
		m.cursor = 0
		m.offset = 0
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case clipboardClearedMsg:
		if strings.Contains(m.status, "copied") {
			m.setStatus("", false)
		}
		return m, nil
	case tea.KeyMsg:
		switch m.state {
		case stateUnlock:
			return m.updateUnlock(msg)
		case stateBrowse:
			return m.updateBrowse(msg)
		case stateInput:
			return m.updateInput(msg)
		case stateForm:
			return m.updateForm(msg)
		}
	}
	return m, nil
}
