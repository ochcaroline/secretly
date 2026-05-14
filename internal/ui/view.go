package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const px = "   "

type helpEntry struct{ key, desc string }

var helpSections = []struct {
	title   string
	entries []helpEntry
}{
	{"Navigation", []helpEntry{
		{"j / k", "move down / up"},
		{"g / G", "jump to top / bottom"},
		{"/ <query>", "search by name, username, notes"},
	}},
	{"Actions", []helpEntry{
		{"a", "add entry"},
		{"d", "delete selected"},
		{"r", "rename selected"},
		{"e", "edit entry"},
	}},
	{"Clipboard & Preview", []helpEntry{
		{"p", "preview entry (p again to reveal password)"},
		{"y", "copy password to clipboard"},
		{"Y", "copy username to clipboard"},
	}},
	{"Misc", []helpEntry{
		{"P", "change master password"},
		{"?", "toggle this help"},
		{"q / ctrl+c", "save and quit"},
	}},
	{"Icons", []helpEntry{
		{"󰌋", "username + password"},
		{"󰀉", "username only"},
		{"󰌾", "password only"},
	}},
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.state {
	case stateUnlock:
		return m.viewUnlock()
	case stateForm:
		return m.overlayForm()
	default:
		if m.showHelp {
			return m.overlayHelp()
		}
		if m.showPreview {
			return m.overlayPreview()
		}
		return m.viewMain()
	}
}

func (m Model) viewUnlock() string {
	w := max(m.width-6, 10)
	sep := m.st.sep.Render(strings.Repeat("─", w))
	pw := strings.Repeat("●", len([]rune(m.input)))
	input := px + m.st.prompt.Render("master password: ") + pw + "█"
	hint := px + m.st.dim.Render("enter  unlock   ctrl+c  quit")

	lines := []string{"", px + m.st.header.Render("secretly"), px + sep, "", input, ""}
	if m.status != "" {
		if m.isError {
			lines = append(lines, px+m.st.errStyle.Render(m.status))
		} else {
			lines = append(lines, px+m.st.ok.Render(m.status))
		}
		lines = append(lines, "")
	}
	lines = append(lines, hint)

	content := strings.Join(lines, "\n")
	pad := max((m.height-(strings.Count(content, "\n")+1))/2, 0)
	return strings.Repeat("\n", pad) + content
}

func (m Model) viewMain() string {
	w := max(m.width-6, 10)
	sep := m.st.sep.Render(strings.Repeat("─", w))

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(px + m.st.header.Render("secretly") + "\n")
	sb.WriteString(px + sep + "\n")

	entries := m.visibleEntries()
	listH := m.listHeight()

	if len(entries) == 0 {
		if m.searchQuery != "" {
			sb.WriteString(px + m.st.dim.Render("(no results)") + "\n")
		} else {
			sb.WriteString(px + m.st.dim.Render("(empty)  ·  press a to add") + "\n")
		}
		for i := 1; i < listH; i++ {
			sb.WriteString("\n")
		}
	} else {
		hlBg := lipgloss.Color(m.cfg.HighlightColor)
		hlFg := lipgloss.Color(m.cfg.HighlightForeground)

		start := m.offset
		end := clamp(start+listH, 0, len(entries))

		for i := start; i < end; i++ {
			entry := entries[i]
			selected := i == m.cursor

			styled := func(s lipgloss.Style) lipgloss.Style {
				if selected {
					return s.Background(hlBg).Foreground(hlFg)
				}
				return s
			}

			cur := styled(lipgloss.NewStyle()).Render("  ")
			if selected {
				cur = styled(m.st.cursor).Render("▸ ")
			}

			label := entryIcon(entry, styled, m.st) + styled(m.st.bold).Render(entry.Name)
			if entry.Username != "" {
				label += styled(m.st.dim).Render("  user: ") +
					styled(lipgloss.NewStyle()).Render(entry.Username)
			}
			if entry.Notes != "" {
				label += styled(m.st.dim).Render("  notes: ") +
					styled(lipgloss.NewStyle()).Render(entry.Notes)
			}

			row := lipgloss.NewStyle().Width(m.width)
			if selected {
				row = row.Background(hlBg)
			}
			sb.WriteString(row.Render(px+cur+label) + "\n")
		}
		for i := end - start; i < listH; i++ {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(px + sep + "\n")

	if m.state == stateInput {
		sb.WriteString(m.viewInputLine() + "\n")
	} else if m.status != "" {
		if m.isError {
			sb.WriteString(px + m.st.errStyle.Render(m.status) + "\n")
		} else {
			sb.WriteString(px + m.st.ok.Render("✓ "+m.status) + "\n")
		}
	} else {
		sb.WriteString("\n")
	}

	if m.searching {
		sb.WriteString(px + m.st.prompt.Render("/") + m.searchQuery + "█  " + m.st.dim.Render("enter/esc  done"))
	} else if m.searchQuery != "" {
		sb.WriteString(px + m.st.dim.Render("/  search: ") + m.searchQuery + "  " + m.st.dim.Render("esc  clear"))
	} else {
		sb.WriteString(px + m.st.dim.Render("? help  /  search"))
	}
	return sb.String()
}

func (m Model) viewInputLine() string {
	var prompt string
	masked := false

	switch m.step {
	case stepRename:
		prompt = "rename: "
	case stepChangeMasterNew:
		prompt = "new master password: "
		masked = true
	case stepChangeMasterConfirm:
		prompt = "confirm new password: "
		masked = true
	case stepConfirmQuit:
		prompt = "quit? [y/n]: "
	case stepConfirmDelete:
		name := ""
		if m.count() > 0 {
			name = m.visibleEntries()[m.cursor].Name
		}
		prompt = "delete " + m.st.errStyle.Render(name) + "? [y/n]: "
	}

	display := m.input
	if masked {
		display = strings.Repeat("●", len([]rune(m.input)))
	}
	return "  " + m.st.prompt.Render(prompt) + display + "█"
}

func (m Model) overlayHelp() string {
	styleKey := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	styleDesc := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.cfg.AccentColor)).
		Padding(1, 3)

	var sb strings.Builder
	for i, section := range helpSections {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(m.st.header.Render(section.title) + "\n")
		for _, e := range section.entries {
			sb.WriteString("  " + styleKey.Render(fmt.Sprintf("%-14s", e.key)) +
				"  " + styleDesc.Render(e.desc) + "\n")
		}
	}
	sb.WriteString("\n" + m.st.dim.Render("? / esc  close"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		border.Render(sb.String()))
}

func (m Model) overlayPreview() string {
	if m.visibleCount() == 0 {
		return m.viewMain()
	}
	entry := m.visibleEntries()[m.cursor]

	styleLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.cfg.AccentColor)).
		Padding(1, 3)

	pw := entry.Password
	if m.previewMask {
		pw = strings.Repeat("●", len([]rune(pw)))
	}
	hint := "p  reveal"
	if !m.previewMask {
		hint = "p  hide"
	}

	row := func(label, val string) string {
		return styleLabel.Render(fmt.Sprintf("%-12s", label)) + "  " + styleValue.Render(val) + "\n"
	}

	var sb strings.Builder
	sb.WriteString(m.st.header.Render(entry.Name) + "\n\n")
	sb.WriteString(row("username", entry.Username))
	sb.WriteString(row("password", pw))
	if entry.Notes != "" {
		sb.WriteString(row("notes", entry.Notes))
	}
	sb.WriteString("\n" + m.st.dim.Render(hint+"   esc  close"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		border.Render(sb.String()))
}

func (m Model) overlayForm() string {
	styleLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleAccent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.cfg.AccentColor)).Bold(true)
	styleFocused := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Underline(true)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.cfg.AccentColor)).
		Padding(1, 3)

	var sb strings.Builder
	sb.WriteString(m.st.header.Render(m.formTitle) + "\n\n")

	for i := range m.formFields {
		field := m.formFields[i]
		val := field.value
		if field.masked {
			val = strings.Repeat("●", len([]rune(val)))
		}
		label := styleLabel.Render(fmt.Sprintf("%-12s", field.label))
		var cur, valStr string
		if i == m.formFocus {
			cur = styleAccent.Render("▸ ")
			valStr = styleFocused.Render(val) + styleAccent.Render("█")
		} else {
			cur = "  "
			valStr = styleValue.Render(val)
		}
		sb.WriteString(cur + label + "  " + valStr + "\n")
	}

	sb.WriteString("\n" + m.st.dim.Render("tab  next field   ctrl+s  save   esc  cancel"))
	if m.status != "" {
		sb.WriteString("\n")
		if m.isError {
			sb.WriteString(m.st.errStyle.Render(m.status))
		} else {
			sb.WriteString(m.st.ok.Render("✓ " + m.status))
		}
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		border.Render(sb.String()))
}
