package search

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
)

// ResultSelectedMsg is sent when the user picks a search result.
type ResultSelectedMsg struct {
	Dialog apptg.Dialog
}

// Model is the fuzzy search popup sub-model.
type Model struct {
	textInput textinput.Model
	dialogs   []apptg.Dialog
	titles    []string
	matches   fuzzy.Matches
	cursor    int
	width     int
	height    int
}

// New creates an inactive search model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Search chats/users…"
	ti.CharLimit = 100
	return Model{textInput: ti}
}

// Open prepares the search popup with the current dialog list.
func (m *Model) Open(dialogs []apptg.Dialog, width, height int) tea.Cmd {
	m.dialogs = dialogs
	m.titles = make([]string, len(dialogs))
	for i, d := range dialogs {
		m.titles[i] = d.Title
	}
	m.matches = nil
	m.cursor = 0
	m.width = width
	m.height = height
	m.textInput.Reset()
	return m.textInput.Focus()
}

// MoveDown moves the result cursor down by n.
func (m *Model) MoveDown(n int) {
	total := m.resultCount()
	if total == 0 {
		return
	}
	m.cursor += n
	if m.cursor >= total {
		m.cursor = total - 1
	}
}

// MoveUp moves the result cursor up by n.
func (m *Model) MoveUp(n int) {
	m.cursor -= n
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Selected returns the currently highlighted dialog.
func (m Model) Selected() (apptg.Dialog, bool) {
	if len(m.matches) == 0 {
		if m.cursor < len(m.dialogs) {
			return m.dialogs[m.cursor], true
		}
		return apptg.Dialog{}, false
	}
	if m.cursor < len(m.matches) {
		idx := m.matches[m.cursor].Index
		return m.dialogs[idx], true
	}
	return apptg.Dialog{}, false
}

func (m Model) resultCount() int {
	if len(m.matches) > 0 || m.textInput.Value() != "" {
		return len(m.matches)
	}
	return len(m.dialogs)
}

// Update handles messages for the search sub-model.
// Returns (Model, cmd, selectedDialog, closed).
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd, *apptg.Dialog, bool) {
	key, isKey := msg.(tea.KeyPressMsg)
	if isKey {
		switch key.String() {
		case "esc":
			return m, nil, nil, true
		case "enter":
			if d, ok := m.Selected(); ok {
				return m, nil, &d, true
			}
			return m, nil, nil, true
		case "down", "ctrl+n":
			m.MoveDown(1)
			return m, nil, nil, false
		case "up", "ctrl+p":
			m.MoveUp(1)
			return m, nil, nil, false
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Re-filter on every keystroke.
	query := m.textInput.Value()
	if query == "" {
		m.matches = nil
		m.cursor = 0
	} else {
		m.matches = fuzzy.Find(query, m.titles)
		m.cursor = 0
	}

	return m, cmd, nil, false
}

// View renders the search popup as a centered box.
func (m Model) View() string {
	popupW := m.width * 2 / 3
	if popupW < 40 {
		popupW = 40
	}
	if popupW > m.width-4 {
		popupW = m.width - 4
	}

	var sb strings.Builder
	sb.WriteString(m.textInput.View())
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", popupW-4))
	sb.WriteString("\n")

	results := m.buildResults(popupW - 4)
	sb.WriteString(results)

	inner := sb.String()
	popup := popupStyle.Width(popupW).Render(inner)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		popup,
	)
}

func (m Model) buildResults(maxW int) string {
	var sb strings.Builder
	maxRows := 10

	if len(m.matches) == 0 && m.textInput.Value() == "" {
		// Show all dialogs up to maxRows.
		for i, d := range m.dialogs {
			if i >= maxRows {
				break
			}
			line := truncate(d.Title, maxW)
			if i == m.cursor {
				sb.WriteString(selectedStyle.Width(maxW).Render("> " + line))
			} else {
				sb.WriteString(normalStyle.Width(maxW).Render("  " + line))
			}
			sb.WriteString("\n")
		}
		return sb.String()
	}

	if len(m.matches) == 0 {
		sb.WriteString(noResultStyle.Render("  No results"))
		return sb.String()
	}

	for i, match := range m.matches {
		if i >= maxRows {
			sb.WriteString(noResultStyle.Render(fmt.Sprintf("  … %d more", len(m.matches)-maxRows)))
			break
		}
		line := highlightMatch(match, maxW)
		if i == m.cursor {
			sb.WriteString(selectedStyle.Width(maxW).Render("> " + line))
		} else {
			sb.WriteString(normalStyle.Width(maxW).Render("  " + line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// highlightMatch renders a fuzzy match with matched chars bolded.
func highlightMatch(m fuzzy.Match, maxW int) string {
	runes := []rune(m.Str)
	matchSet := make(map[int]bool, len(m.MatchedIndexes))
	for _, idx := range m.MatchedIndexes {
		matchSet[idx] = true
	}

	var sb strings.Builder
	for i, r := range runes {
		if matchSet[i] {
			sb.WriteString(matchStyle.Render(string(r)))
		} else {
			sb.WriteRune(r)
		}
	}
	return truncate(sb.String(), maxW)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

var (
	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#61AFEF")).
			Background(lipgloss.Color("#21252B")).
			Padding(1, 2)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2F3642")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D7DCE5")).
			Bold(true)

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD166")).
			Bold(true)

	noResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).
			Italic(true)
)
