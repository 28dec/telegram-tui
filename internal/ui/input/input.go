package input

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// InputType distinguishes how the user is composing a message.
type InputType int

const (
	Inline     InputType = iota // "i" - single line, Enter sends
	Multiline                   // "I" - textarea, Ctrl+Enter sends
	Reply                       // "r" - inline reply
	ReplyMulti                  // "R" - multiline reply
)

// Model manages the message composition area.
type Model struct {
	inputType InputType
	single    textinput.Model
	multi     textarea.Model
	replyToID *int
	active    bool
	width     int
}

// New creates an inactive input model.
func New(width int) Model {
	return Model{width: width}
}

// Activate prepares the model for the given input type and optional replyToID.
func (m *Model) Activate(t InputType, replyToID *int, width int) tea.Cmd {
	m.inputType = t
	m.replyToID = replyToID
	m.active = true
	m.width = width

	var cmd tea.Cmd
	switch t {
	case Inline, Reply:
		ti := textinput.New()
		ti.Placeholder = "Type a message…"
		ti.CharLimit = 4096
		ti.SetWidth(width - 4)
		m.single = ti
		cmd = m.single.Focus()
	case Multiline, ReplyMulti:
		ta := textarea.New()
		ta.Placeholder = "Type a message… (Ctrl+Enter to send, Esc to cancel)"
		ta.CharLimit = 4096
		ta.SetWidth(width - 4)
		ta.SetHeight(4)
		m.multi = ta
		cmd = m.multi.Focus()
	}
	return cmd
}

// Deactivate clears the input state.
func (m *Model) Deactivate() {
	m.active = false
	m.single = textinput.Model{}
	m.multi = textarea.Model{}
	m.replyToID = nil
}

// Value returns the current text.
func (m Model) Value() string {
	switch m.inputType {
	case Inline, Reply:
		return strings.TrimSpace(m.single.Value())
	default:
		return strings.TrimSpace(m.multi.Value())
	}
}

// ReplyToID returns the reply target message ID, if any.
func (m Model) ReplyToID() *int { return m.replyToID }

// Active reports whether the input area is currently shown.
func (m Model) Active() bool { return m.active }

// Update handles messages for the input sub-model.
// Returns (Model, cmd, shouldSend) where shouldSend triggers message delivery.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd, bool) {
	if !m.active {
		return m, nil, false
	}

	key, isKey := msg.(tea.KeyPressMsg)

	switch m.inputType {
	case Inline, Reply:
		if isKey {
			switch key.String() {
			case "enter":
				if m.Value() != "" {
					return m, nil, true
				}
				return m, nil, false
			case "esc":
				m.Deactivate()
				return m, nil, false
			}
		}
		var cmd tea.Cmd
		m.single, cmd = m.single.Update(msg)
		return m, cmd, false

	case Multiline, ReplyMulti:
		if isKey {
			switch key.String() {
			case "ctrl+enter":
				if m.Value() != "" {
					return m, nil, true
				}
				return m, nil, false
			case "esc":
				m.Deactivate()
				return m, nil, false
			}
		}
		var cmd tea.Cmd
		m.multi, cmd = m.multi.Update(msg)
		return m, cmd, false
	}

	return m, nil, false
}

// View renders the input area.
func (m Model) View() string {
	if !m.active {
		return ""
	}

	var sb strings.Builder

	if m.replyToID != nil {
		sb.WriteString(replyLabelStyle.Render("  ↩ Replying"))
		sb.WriteString("\n")
	}

	switch m.inputType {
	case Inline, Reply:
		sb.WriteString(inputBoxStyle.Width(m.width).Render("> " + m.single.View()))
	case Multiline, ReplyMulti:
		sb.WriteString(inputBoxStyle.Width(m.width).Render(m.multi.View()))
	}

	return sb.String()
}

var (
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Foreground(lipgloss.Color("#D7DCE5")).
			Bold(true)

	replyLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7CC7FF")).
			Bold(true)

	inputHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).
			Italic(true)
)
