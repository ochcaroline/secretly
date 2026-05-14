package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ochcaroline/secretly/internal/store"
)

type clipboardClearedMsg struct{}

func clearClipboardAfter(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		_ = clipboard.WriteAll("")
		return clipboardClearedMsg{}
	}
}

func (m Model) updateUnlock(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEnter:
		if m.input == "" {
			m.setStatus("password required", true)
			return m, nil
		}
		db, err := store.Load(m.dbPath, m.input)
		if err != nil {
			m.input = ""
			m.setStatus(err.Error(), true)
			return m, nil
		}
		if db == nil {
			db = &store.DB{}
		}
		m.db = db
		m.master = m.input
		m.input = ""
		m.sortEntries()
		m.state = stateBrowse

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}

	default:
		if key.Type == tea.KeyRunes {
			m.input += string(key.Runes)
		}
	}
	return m, nil
}

func (m Model) updateBrowse(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While actively typing a search query, intercept all keys.
	if m.searching {
		switch key.Type {
		case tea.KeyCtrlC:
			m.saveDB()
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyEnter:
			m.searching = false
		case tea.KeyBackspace:
			if len(m.searchQuery) > 0 {
				r := []rune(m.searchQuery)
				m.searchQuery = string(r[:len(r)-1])
				m.updateFilter()
				m.cursor = 0
				m.offset = 0
			}
		default:
			if key.Type == tea.KeyRunes {
				m.searchQuery += string(key.Runes)
				m.updateFilter()
				m.cursor = 0
				m.offset = 0
			}
		}
		return m, nil
	}

	n := m.visibleCount()
	m.setStatus("", false)

	switch key.String() {
	case "ctrl+c":
		m.saveDB()
		return m, tea.Quit

	case "q":
		m.state = stateInput
		m.step = stepConfirmQuit
		m.input = ""

	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.filteredIndices = nil
			m.cursor = 0
			m.offset = 0
		} else {
			m.showHelp = false
			m.showPreview = false
			m.previewMask = false
		}

	case "?":
		m.showHelp = !m.showHelp

	case "/":
		m.searching = true

	case "j", "down":
		if n > 0 && m.cursor < n-1 {
			m.cursor++
			m.scrollToCursor()
			m.showPreview = false
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.scrollToCursor()
			m.showPreview = false
		}

	case "g":
		m.cursor = 0
		m.offset = 0

	case "G":
		if n > 0 {
			m.cursor = n - 1
			m.scrollToCursor()
		}

	case "a":
		m.openForm("Add Entry", store.Entry{}, false, 0)

	case "d":
		if n > 0 {
			m.state = stateInput
			m.step = stepConfirmDelete
			m.input = ""
		}

	case "r":
		if n > 0 {
			realIdx := m.realCursorIndex()
			m.input = m.db.Entries[realIdx].Name
			m.state = stateInput
			m.step = stepRename
		}

	case "e":
		if n > 0 {
			realIdx := m.realCursorIndex()
			m.openForm("Edit Entry", m.db.Entries[realIdx], true, realIdx)
		}

	case "y":
		if n > 0 {
			realIdx := m.realCursorIndex()
			return m.copyToClipboard(m.db.Entries[realIdx].Password, "password")
		}

	case "Y":
		if n > 0 {
			realIdx := m.realCursorIndex()
			return m.copyToClipboard(m.db.Entries[realIdx].Username, "username")
		}

	case "p":
		if n > 0 {
			if m.showPreview {
				m.previewMask = !m.previewMask
			} else {
				m.showPreview = true
				m.previewMask = true
			}
		}

	case "P":
		m.state = stateInput
		m.step = stepChangeMasterNew
		m.input = ""
	}

	return m, nil
}

func (m Model) copyToClipboard(text, label string) (tea.Model, tea.Cmd) {
	if err := clipboard.WriteAll(text); err != nil {
		m.setStatus("clipboard unavailable", true)
		return m, nil
	}
	m.setStatus(fmt.Sprintf("%s copied (clears in %ds)", label, m.cfg.ClipboardTimeout), false)
	return m, clearClipboardAfter(m.clipboardTTL())
}

func (m *Model) openForm(title string, entry store.Entry, isEdit bool, editIdx int) {
	m.formTitle = title
	m.formIsEdit = isEdit
	m.formEditIdx = editIdx
	m.formFocus = 0
	m.formPassWarn = false
	m.formFields = [formFieldCount]formField{
		{label: "name", value: entry.Name},
		{label: "username", value: entry.Username},
		{label: "password", value: entry.Password, masked: true},
		{label: "notes", value: entry.Notes},
	}
	m.state = stateForm
}

func (m Model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyCtrlC:
		m.saveDB()
		return m, tea.Quit

	case tea.KeyEsc:
		m.state = stateBrowse
		m.setStatus("cancelled", false)
		return m, nil

	case tea.KeyTab, tea.KeyDown:
		m.formFocus = (m.formFocus + 1) % formFieldCount
		return m, nil

	case tea.KeyShiftTab, tea.KeyUp:
		m.formFocus = (m.formFocus - 1 + formFieldCount) % formFieldCount
		return m, nil

	case tea.KeyBackspace:
		f := &m.formFields[m.formFocus]
		if len(f.value) > 0 {
			r := []rune(f.value)
			f.value = string(r[:len(r)-1])
		}
		return m, nil

	case tea.KeySpace:
		m.formFields[m.formFocus].value += " "
		return m, nil

	case tea.KeyEnter:
		if m.formFocus < formFieldCount-1 {
			m.formFocus++
			return m, nil
		}
		return m.submitForm()

	case tea.KeyCtrlS:
		return m.submitForm()

	default:
		if key.Type == tea.KeyRunes {
			m.formFields[m.formFocus].value += string(key.Runes)
		}
	}
	return m, nil
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.formFields[formFieldName].value)
	if name == "" {
		m.setStatus("name is required", true)
		m.formFocus = formFieldName
		return m, nil
	}
	pass := m.formFields[formFieldPass].value
	if pass == "" && !m.formPassWarn {
		m.formPassWarn = true
		m.setStatus("password is empty — ctrl+s again to confirm", true)
		return m, nil
	}
	m.formPassWarn = false

	entry := store.Entry{
		Name:     name,
		Username: m.formFields[formFieldUser].value,
		Password: pass,
		Notes:    strings.TrimSpace(m.formFields[formFieldNotes].value),
	}

	if m.formIsEdit {
		m.db.Entries[m.formEditIdx] = entry
		m.sortEntries()
		m.updateFilter()
		m.setCursorToName(entry.Name)
		m.setStatus("entry updated", false)
	} else {
		m.db.Entries = append(m.db.Entries, entry)
		m.sortEntries()
		m.updateFilter()
		m.setCursorToName(entry.Name)
		m.setStatus("entry added", false)
	}
	m.saveDB()
	m.state = stateBrowse
	return m, nil
}

func (m Model) updateInput(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyCtrlC:
		m.saveDB()
		return m, tea.Quit

	case tea.KeyEsc:
		m.state = stateBrowse
		m.step = stepNone
		m.input = ""
		m.setStatus("cancelled", false)
		return m, nil

	case tea.KeyBackspace:
		if len(m.input) > 0 {
			r := []rune(m.input)
			m.input = string(r[:len(r)-1])
		}
		return m, nil

	case tea.KeyEnter:
		return m.confirmInput()

	case tea.KeySpace:
		if m.step != stepConfirmDelete && m.step != stepConfirmQuit {
			m.input += " "
		}

	default:
		if key.Type == tea.KeyRunes {
			if m.step == stepConfirmDelete || m.step == stepConfirmQuit {
				m.input = string(key.Runes)
				return m.confirmInput()
			}
			m.input += string(key.Runes)
		}
	}
	return m, nil
}

func (m Model) confirmInput() (tea.Model, tea.Cmd) {
	switch m.step {

	case stepRename:
		name := strings.TrimSpace(m.input)
		if name == "" {
			m.setStatus("name required", true)
			return m, nil
		}
		realIdx := m.realCursorIndex()
		m.db.Entries[realIdx].Name = name
		m.sortEntries()
		m.saveDB()
		m.updateFilter()
		m.setCursorToName(name)
		m.setStatus("renamed", false)
		m.state = stateBrowse
		m.step = stepNone
		m.input = ""

	case stepChangeMasterNew:
		if m.input == "" {
			m.setStatus("password cannot be empty", true)
			return m, nil
		}
		m.pendingMaster = m.input
		m.step = stepChangeMasterConfirm
		m.input = ""

	case stepChangeMasterConfirm:
		if !store.ConstantTimeEq(m.input, m.pendingMaster) {
			m.pendingMaster = ""
			m.step = stepChangeMasterNew
			m.input = ""
			m.setStatus("passwords do not match, try again", true)
			return m, nil
		}
		oldMaster := m.master
		m.master = m.pendingMaster
		m.pendingMaster = ""
		if err := store.Save(m.db, m.dbPath, m.master); err != nil {
			m.master = oldMaster
			m.setStatus("save error: "+err.Error(), true)
			m.state = stateBrowse
			m.step = stepNone
			m.input = ""
			return m, nil
		}
		m.state = stateBrowse
		m.step = stepNone
		m.input = ""
		m.setStatus("master password changed", false)

	case stepConfirmQuit:
		if m.input == "y" {
			m.saveDB()
			return m, tea.Quit
		}
		m.state = stateBrowse
		m.step = stepNone
		m.input = ""
		m.setStatus("cancelled", false)

	case stepConfirmDelete:
		if m.input == "y" && m.count() > 0 {
			realIdx := m.realCursorIndex()
			m.db.Entries = append(m.db.Entries[:realIdx], m.db.Entries[realIdx+1:]...)
			m.saveDB()
			m.updateFilter()
			if m.cursor >= m.visibleCount() && m.cursor > 0 {
				m.cursor--
			}
			m.setStatus("deleted", false)
		} else {
			m.setStatus("cancelled", false)
		}
		m.state = stateBrowse
		m.step = stepNone
		m.input = ""
	}

	return m, nil
}
