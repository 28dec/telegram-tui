package chatview

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"

	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
	"github.com/dxlongnh/telegram-tui/internal/ui/style"
)

// Model is the chat message view sub-model.
type Model struct {
	viewport  viewport.Model
	messages  []apptg.ChatMessage
	cursor    int // index into messages; -1 = none
	peerTitle string
	selfID    int64
	width     int
	height    int
	loading   bool
}

// New creates a new chat view.
func New(width, height int, selfID int64) Model {
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(maxInt(1, height-1)), // reserve input row; header is global
	)
	return Model{
		viewport: vp,
		cursor:   -1,
		width:    width,
		height:   height,
		selfID:   selfID,
		loading:  true,
	}
}

// SetSize updates terminal dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.SetWidth(w)
	m.viewport.SetHeight(maxInt(1, h-1))
	m.renderContent()
}

// SetMessages replaces the message list and re-renders.
func (m *Model) SetMessages(msgs []apptg.ChatMessage) {
	m.messages = msgs
	m.loading = false
	if len(msgs) > 0 {
		m.cursor = len(msgs) - 1 // default: newest
		for i, msg := range msgs {
			if msg.Unread {
				m.cursor = i // jump to oldest unread in loaded range
				break
			}
		}
	} else {
		m.cursor = -1
	}
	m.renderContent()
	if m.cursor >= 0 && m.cursor < len(msgs) && msgs[m.cursor].Unread {
		m.viewport.GotoTop()
		m.scrollToCursor()
	} else {
		m.viewport.GotoBottom()
	}
}

// AppendMessage adds a new message. Keeps cursor at the same message unless
// the cursor was already at the last message (auto-follow).
func (m *Model) AppendMessage(msg apptg.ChatMessage) {
	m.appendMessage(msg, false)
}

// AppendMessageAndFollow adds a new message and always jumps to it.
// Used for local-send acknowledgements so the just-sent message is visible.
func (m *Model) AppendMessageAndFollow(msg apptg.ChatMessage) {
	m.appendMessage(msg, true)
}

func (m *Model) appendMessage(msg apptg.ChatMessage, forceFollow bool) {
	wasAtLast := m.cursor == len(m.messages)-1
	m.messages = append(m.messages, msg)
	if forceFollow || wasAtLast {
		m.cursor = len(m.messages) - 1
	}
	m.renderContent()
	if forceFollow || wasAtLast {
		m.viewport.GotoBottom()
	}
}

// PrependMessages adds older messages at the top (for pagination).
func (m *Model) PrependMessages(msgs []apptg.ChatMessage) {
	// Shift cursor by the number of prepended messages.
	m.cursor += len(msgs)
	m.messages = append(msgs, m.messages...)
	m.renderContent()
}

// SetPeer sets the title shown in the header.
func (m *Model) SetPeer(title string) {
	m.peerTitle = title
	m.loading = true
}

// AtTop returns true if the viewport is scrolled to the top (triggers pagination).
func (m Model) AtTop() bool {
	return m.viewport.AtTop()
}

// MoveCursor moves the cursor by delta messages, clamped to valid range.
// The viewport follows the cursor. Returns true if the cursor hit the top
// (caller should load more history).
func (m *Model) MoveCursor(delta int) bool {
	if len(m.messages) == 0 {
		return false
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.messages) {
		m.cursor = len(m.messages) - 1
	}
	m.renderContent()
	m.scrollToCursor()
	return m.cursor == 0
}

// scrollToCursor scrolls the viewport so the cursor message is fully visible.
func (m *Model) scrollToCursor() {
	if m.cursor < 0 || len(m.messages) == 0 {
		return
	}
	vpHeight := m.viewport.Height()
	total := m.viewport.TotalLineCount()
	if total == 0 {
		return
	}

	// Calculate the first line of the cursor message.
	cursorStart := m.linesBeforeIndex(m.cursor)
	cursorEnd := cursorStart + m.messageHeight(m.messages[m.cursor])

	// Current visible window.
	visibleTop := int(m.viewport.ScrollPercent() * float64(total-vpHeight+1))
	visibleBottom := visibleTop + vpHeight

	if cursorStart < visibleTop {
		// Cursor is above: scroll up so cursor is at the top.
		m.viewport.ScrollUp(visibleTop - cursorStart)
	} else if cursorEnd > visibleBottom {
		// Cursor bottom is below: scroll down so cursor bottom aligns with viewport bottom.
		m.viewport.ScrollDown(cursorEnd - visibleBottom)
	}
}

// linesBeforeIndex counts rendered lines before message at index i,
// including date separators.
func (m *Model) linesBeforeIndex(idx int) int {
	lines := 0
	var prevDate time.Time
	for i, msg := range m.messages {
		if i >= idx {
			break
		}
		if !sameDay(prevDate, msg.Date) {
			prevDate = msg.Date
			lines++
		}
		lines += m.messageHeight(msg)
		lines += messageGapLines
	}
	return lines
}

// LoadedCount returns number of currently loaded messages.
func (m Model) LoadedCount() int { return len(m.messages) }

// OldestMessageID returns the ID of the oldest loaded message (for pagination).
// Returns 0 if no messages are loaded.
func (m Model) OldestMessageID() int {
	if len(m.messages) == 0 {
		return 0
	}
	return m.messages[0].ID
}

// messageHeight returns how many terminal lines a message occupies, excluding
// inter-message gap lines.
func (m *Model) messageHeight(msg apptg.ChatMessage) int {
	lines := 1 // header
	if msg.ReplyToID != nil {
		lines++ // reply indicator line
	}
	body := msg.Text
	if body == "" {
		lines++ // placeholder or blank
	} else {
		lines += len(strings.Split(body, "\n"))
	}
	return lines
}

// CursorMedia returns the media of the cursor message, or nil.
func (m Model) CursorMedia() *apptg.MediaInfo {
	if m.cursor < 0 || m.cursor >= len(m.messages) {
		return nil
	}
	return m.messages[m.cursor].Media
}

// CursorMessageID returns the ID of the cursor message, or nil if none.
func (m Model) CursorMessageID() *int {
	if m.cursor < 0 || m.cursor >= len(m.messages) {
		return nil
	}
	id := m.messages[m.cursor].ID
	return &id
}

// CursorMessage returns a copy of the cursor message, or false if none.
func (m Model) CursorMessage() (apptg.ChatMessage, bool) {
	if m.cursor < 0 || m.cursor >= len(m.messages) {
		return apptg.ChatMessage{}, false
	}
	return m.messages[m.cursor], true
}

// MarkReadThrough marks all incoming unread messages with ID <= maxID as read.
// Returns the number of messages that changed.
func (m *Model) MarkReadThrough(maxID int) int {
	if maxID <= 0 || len(m.messages) == 0 {
		return 0
	}
	changed := 0
	for i := range m.messages {
		if m.messages[i].Unread && !m.messages[i].Out && m.messages[i].ID <= maxID {
			m.messages[i].Unread = false
			changed++
		}
	}
	if changed > 0 {
		m.renderContent()
	}
	return changed
}

// renderContent re-renders all messages into the viewport.
func (m *Model) renderContent() {
	m.viewport.SetContent(m.buildContent())
}

func (m *Model) buildContent() string {
	if len(m.messages) == 0 {
		return loadingStyle.Render("  No messages yet.")
	}

	var sb strings.Builder
	var prevDate time.Time

	for i, msg := range m.messages {
		// Date separator when the day changes.
		if !sameDay(prevDate, msg.Date) {
			prevDate = msg.Date
			sep := m.renderDateSeparator(msg.Date)
			sb.WriteString(sep)
			sb.WriteString("\n")
		}

		sb.WriteString(m.renderMessage(msg, i == m.cursor))
		sb.WriteString("\n")
		if i < len(m.messages)-1 {
			sb.WriteString(strings.Repeat("\n", messageGapLines))
		}
	}

	return sb.String()
}

func (m *Model) renderMessage(msg apptg.ChatMessage, isCursor bool) string {
	tsLabel := "[" + msg.Date.Local().Format("2006-01-02 15:04") + "]"

	// When highlighted, skip per-element ANSI styles so the cursor background
	// fills the entire row uniformly (timestamp, username, body all same bg).
	var cursorMark, tsStr, nameStr, unread, mediaHint, edited string
	if isCursor {
		cursorMark = "▶ "
		tsStr = tsLabel
		nameStr = msg.FromName
		if msg.Unread {
			unread = " ●"
		}
		if msg.Media != nil {
			mediaHint = " [Space]"
		}
		if msg.EditDate != nil {
			edited = " (edited)"
		}
	} else {
		cursorMark = "  "
		tsStr = timeStyle.Render(tsLabel)
		nameStr = style.ColorForUser(msg.FromID).Render(msg.FromName)
		if msg.Unread {
			unread = unreadStyle.Render(" ●")
		}
		if msg.EditDate != nil {
			edited = editedStyle.Render(" (edited)")
		}
	}

	line1 := fmt.Sprintf("%s%s %s%s%s", cursorMark, tsStr, nameStr, unread, mediaHint)
	bodyIndent := strings.Repeat(" ", runeLen(cursorMark)+runeLen(tsLabel)+1)

	// Reply indicator: ↩ <sender>: <preview>
	replyLine := ""
	if msg.ReplyToID != nil {
		replyLine = m.formatReplyLine(*msg.ReplyToID, bodyIndent, isCursor)
	}

	// Line 2+: message body (always at least one line)
	body := msg.Text
	if body == "" {
		if msg.Media != nil {
			body = formatMediaPlaceholder(msg.Media)
		} else {
			body = "\u200b" // zero-width space keeps the line present
		}
	}

	bodyLines := strings.Split(body, "\n")
	var bodyBuilder strings.Builder
	for i, line := range bodyLines {
		if i > 0 {
			bodyBuilder.WriteString("\n")
		}
		bodyBuilder.WriteString(bodyIndent)
		bodyBuilder.WriteString(line)
	}
	bodyBuilder.WriteString(edited)

	full := line1 + "\n" + bodyBuilder.String()
	if replyLine != "" {
		full = line1 + "\n" + replyLine + "\n" + bodyBuilder.String()
	}

	contentWidth := widestLine(full)
	innerWidth, sidePad := m.dynamicContainerWidth(contentWidth)

	container := lipgloss.NewStyle().
		PaddingLeft(sidePad).
		PaddingRight(sidePad).
		Width(m.width)

	msgStyle := lipgloss.NewStyle().Width(innerWidth)
	if msg.Out {
		msgStyle = msgStyle.Align(lipgloss.Right)
		if !isCursor {
			msgStyle = msgStyle.Foreground(lipgloss.Color("#ABB2BF"))
		}
	} else {
		msgStyle = msgStyle.Align(lipgloss.Left)
	}

	return container.Render(msgStyle.Render(full))
}

func widestLine(s string) int {
	lines := strings.Split(s, "\n")
	maxW := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}
	if maxW == 0 {
		return 1
	}
	return maxW
}

func (m Model) dynamicContainerWidth(contentWidth int) (innerWidth int, sidePad int) {
	if m.width <= 2 {
		return m.width, 0
	}

	minSidePad := 2
	maxSidePad := m.width / 4
	if maxSidePad < minSidePad {
		maxSidePad = minSidePad
	}

	// Short messages get more side padding; long messages naturally reduce it.
	targetContent := contentWidth + 4
	sidePad = (m.width - targetContent) / 2
	if sidePad < minSidePad {
		sidePad = minSidePad
	}
	if sidePad > maxSidePad {
		sidePad = maxSidePad
	}

	innerWidth = m.width - (sidePad * 2)
	if innerWidth < 8 {
		innerWidth = 8
		sidePad = (m.width - innerWidth) / 2
		if sidePad < 0 {
			sidePad = 0
		}
	}

	return innerWidth, sidePad
}

func (m Model) renderDateSeparator(t time.Time) string {
	label := t.Local().Format("2006-01-02")
	if m.width <= lipgloss.Width(label)+2 {
		return dateSepStyle.Width(m.width).Align(lipgloss.Center).Render(label)
	}

	fill := m.width - lipgloss.Width(label) - 2 // spaces around label
	left := fill / 2
	right := fill - left
	line := strings.Repeat("─", left) + " " + label + " " + strings.Repeat("─", right)
	return dateSepStyle.Width(m.width).Render(line)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatReplyLine returns the indented reply quote line, or empty string if the
// replied-to message is not found in the loaded history.
func (m *Model) formatReplyLine(replyToID int, indent string, isCursor bool) string {
	for _, msg := range m.messages {
		if msg.ID != replyToID {
			continue
		}
		preview := msg.Text
		if preview == "" && msg.Media != nil {
			preview = formatMediaPlaceholder(msg.Media)
		}
		if len([]rune(preview)) > 60 {
			preview = string([]rune(preview)[:59]) + "…"
		}
		line := indent + "↩ " + msg.FromName + ": " + preview
		if isCursor {
			return line // plain text, cursor bg covers it
		}
		return replyStyle.Render(line)
	}
	// Replied-to message not loaded — show a generic indicator.
	line := indent + "↩ (original message)"
	if isCursor {
		return line
	}
	return replyStyle.Render(line)
}

func formatMediaPlaceholder(media *apptg.MediaInfo) string {
	switch media.Type {
	case apptg.MediaPhoto:
		return "📷 Photo"
	case apptg.MediaVideo:
		return "🎥 Video: " + media.FileName
	case apptg.MediaAudio:
		return "🎵 Audio: " + media.FileName
	case apptg.MediaDocument:
		return "📎 Document: " + media.FileName
	default:
		return "📎 Media"
	}
}

func sameDay(a, b time.Time) bool {
	if a.IsZero() {
		return false
	}
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}

func runeLen(s string) int {
	return len([]rune(s))
}

const messageGapLines = 1

// View renders the chat view.
func (m Model) View() string {
	if m.loading {
		return loadingStyle.Width(m.width).Align(lipgloss.Center).Render("Loading messages…")
	}
	return m.viewport.View()
}

var (
	timeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B8C0CE")).
			Bold(true)

	unreadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD166")).
			Bold(true)

	editedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAB2C0")).
			Bold(true)

	dateSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BA4C0")).
			Bold(true)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAB2C0")).
			Bold(true)

	mediaHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379")).
			Italic(true)

	replyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAB2C0")).
			Bold(true)
)
