package channellist

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
)

// Model is the channel/group list sub-model.
type Model struct {
	allDialogs   []apptg.Dialog
	dialogs      []apptg.Dialog
	cursor       int
	offset       int // first visible row index
	width        int
	height       int
	loading      bool
	showArchived bool
}

// New creates an empty channel list.
func New(width, height int) Model {
	return Model{
		width:   width,
		height:  height,
		loading: true,
	}
}

// SetDialogs replaces the dialog list and resets navigation.
func (m *Model) SetDialogs(dialogs []apptg.Dialog) {
	m.allDialogs = append([]apptg.Dialog(nil), dialogs...)
	m.cursor = 0
	m.offset = 0
	m.loading = false
	m.applySort(0, false)
}

// SetSize updates terminal dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.clampOffset()
}

// Dialogs returns the currently visible dialog list.
func (m Model) Dialogs() []apptg.Dialog { return m.dialogs }

// AllDialogs returns all chats (main + archived) used for global search.
func (m Model) AllDialogs() []apptg.Dialog { return m.allDialogs }

// TotalCount returns total chats in all folders.
func (m Model) TotalCount() int { return len(m.allDialogs) }

// ArchivedCount returns count of archived chats.
func (m Model) ArchivedCount() int {
	c := 0
	for _, d := range m.allDialogs {
		if d.Archived {
			c++
		}
	}
	return c
}

// MainCount returns count of non-archived chats.
func (m Model) MainCount() int {
	c := 0
	for _, d := range m.allDialogs {
		if !d.Archived {
			c++
		}
	}
	return c
}

// ArchivedModeLabel returns whether main or archived groups are shown.
func (m Model) ArchivedModeLabel() string {
	if m.showArchived {
		return "Archived"
	}
	return "Main"
}

// ToggleArchived switches between main and archived group lists.
func (m *Model) ToggleArchived() {
	m.showArchived = !m.showArchived
	m.rebuildVisibleDialogs(0, false)
}

// UpdateDialog updates the last message, date, and unread count for the given peer ID.
// If the peer is not in the list, does nothing.
func (m *Model) UpdateDialog(peerID int64, lastMsg string, lastDate time.Time, deltaUnread int) {
	selectedPeerID, selectedOK := m.selectedPeerID()
	for i := range m.allDialogs {
		if m.allDialogs[i].PeerID == peerID {
			m.allDialogs[i].LastMessage = lastMsg
			m.allDialogs[i].LastDate = lastDate
			if deltaUnread > 0 {
				m.allDialogs[i].UnreadCount += deltaUnread
			}
			m.applySort(selectedPeerID, selectedOK)
			return
		}
	}
}

// ConsumeUnread decreases unread count for the given peer ID by n (clamped at 0).
func (m *Model) ConsumeUnread(peerID int64, n int) {
	if n <= 0 {
		return
	}
	selectedPeerID, selectedOK := m.selectedPeerID()
	for i := range m.allDialogs {
		if m.allDialogs[i].PeerID == peerID {
			m.allDialogs[i].UnreadCount -= n
			if m.allDialogs[i].UnreadCount < 0 {
				m.allDialogs[i].UnreadCount = 0
			}
			m.applySort(selectedPeerID, selectedOK)
			return
		}
	}
}

// SetReadInboxMax updates the read-inbox max message ID for a peer.
func (m *Model) SetReadInboxMax(peerID int64, maxID int) {
	if maxID <= 0 {
		return
	}
	selectedPeerID, selectedOK := m.selectedPeerID()
	for i := range m.allDialogs {
		if m.allDialogs[i].PeerID == peerID {
			if maxID > m.allDialogs[i].ReadInboxMaxID {
				m.allDialogs[i].ReadInboxMaxID = maxID
				m.applySort(selectedPeerID, selectedOK)
			}
			return
		}
	}
}

// ApplyReadUpdate applies a Telegram read update for a peer.
func (m *Model) ApplyReadUpdate(peerID int64, maxID int, stillUnreadCount int) {
	selectedPeerID, selectedOK := m.selectedPeerID()
	for i := range m.allDialogs {
		if m.allDialogs[i].PeerID == peerID {
			if maxID > m.allDialogs[i].ReadInboxMaxID {
				m.allDialogs[i].ReadInboxMaxID = maxID
			}
			if stillUnreadCount >= 0 {
				m.allDialogs[i].UnreadCount = stillUnreadCount
			} else {
				m.allDialogs[i].UnreadCount = 0
			}
			m.applySort(selectedPeerID, selectedOK)
			return
		}
	}
}

// ClearUnread zeroes the unread count for the given peer ID.
func (m *Model) ClearUnread(peerID int64) {
	selectedPeerID, selectedOK := m.selectedPeerID()
	for i := range m.allDialogs {
		if m.allDialogs[i].PeerID == peerID {
			m.allDialogs[i].UnreadCount = 0
			m.applySort(selectedPeerID, selectedOK)
			return
		}
	}
}

// Selected returns the currently highlighted dialog, if any.
func (m Model) Selected() (apptg.Dialog, bool) {
	if len(m.dialogs) == 0 || m.cursor < 0 || m.cursor >= len(m.dialogs) {
		return apptg.Dialog{}, false
	}
	return m.dialogs[m.cursor], true
}

// MoveDown moves the cursor down by n items.
func (m *Model) MoveDown(n int) {
	if len(m.dialogs) == 0 {
		return
	}
	m.cursor += n
	if m.cursor >= len(m.dialogs) {
		m.cursor = len(m.dialogs) - 1
	}
	m.clampOffset()
}

// MoveUp moves the cursor up by n items.
func (m *Model) MoveUp(n int) {
	m.cursor -= n
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.clampOffset()
}

// clampOffset adjusts scroll so the cursor stays visible.
func (m *Model) clampOffset() {
	if len(m.dialogs) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.dialogs) {
		m.cursor = len(m.dialogs) - 1
	}

	visible := m.visibleRows()
	selectedTop := m.dialogStartLine(m.cursor)

	if selectedTop < m.offset {
		m.offset = selectedTop
	}
	if selectedTop >= m.offset+visible {
		m.offset = selectedTop - visible + 1
	}

	maxOffset := m.totalContentLines() - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// visibleRows is the number of content rows that fit.
func (m Model) visibleRows() int {
	if m.height < 1 {
		return 1
	}
	return m.height
}

// View renders the dialog list as a string.
func (m Model) View() string {
	if m.loading {
		return loadingStyle.Width(m.width).Align(lipgloss.Center).Render("Loading groups…")
	}
	if len(m.dialogs) == 0 {
		msg := "No chats found."
		if m.showArchived {
			msg = "No archived chats."
		}
		return loadingStyle.Width(m.width).Align(lipgloss.Center).Render(msg)
	}

	lines := make([]string, 0, m.visibleRows())
	visible := m.visibleRows()
	windowStart := m.offset
	windowEnd := windowStart + visible

	viewRows := make([]string, 0, visible)
	for i := windowStart; i < windowEnd && i < len(m.dialogs); i++ {
		viewRows = append(viewRows, m.renderRow(m.dialogs[i], i == m.cursor))
	}

	topPadding := 0
	if len(m.dialogs) <= visible {
		topPadding = (visible - len(viewRows)) / 2
	}
	for i := 0; i < topPadding; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, viewRows...)
	for len(lines) < visible {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderRow(d apptg.Dialog, selected bool) string {
	dateStr := formatDate(d.LastDate)
	badgeRaw := unreadBadge(d.UnreadCount)

	cursor := " "
	if selected {
		cursor = ">"
	}

	prefixPlain := cursor + " "
	titleSuffix := ""
	if badgeRaw != "" {
		titleSuffix = " " + badgeRaw
	}

	rowWidth := m.rowWidth()
	titleWidth := rowWidth - displayWidth(prefixPlain) - displayWidth(dateStr) - 1
	if titleWidth < 4 {
		titleWidth = 4
	}

	title := truncateDisplay(oneLineText(d.Title)+titleSuffix, titleWidth)
	title = padRightDisplay(title, titleWidth)
	line := fitDisplayWidth(prefixPlain+title+" "+dateStr, rowWidth)
	rowStyle := normalStyle
	if selected {
		rowStyle = selectedStyle
	}
	inner := rowStyle.Render(line)
	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(inner)
}

func (m Model) rowWidth() int {
	w := m.width - 4
	if w < 1 {
		return 1
	}
	if w > 88 {
		w = 88
	}
	if w < 24 && m.width >= 24 {
		w = 24
	}
	return w
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "                "
	}
	return t.Local().Format("2006-01-02 15:04")
}

func peerMarker(t apptg.PeerType) (label string, s lipgloss.Style) {
	switch t {
	case apptg.PeerUser:
		return "[USER]", userMarkerStyle
	case apptg.PeerChat:
		return "[GROUP]", groupMarkerStyle
	case apptg.PeerChannel:
		return "[GROUP]", groupMarkerStyle
	default:
		return "[UNKNOWN]", unknownMarkerStyle
	}
}

func unreadBadge(unread int) string {
	if unread <= 0 {
		return ""
	}
	count := fmt.Sprintf("%d", unread)
	if unread > 99 {
		count = "99+"
	}
	return "(" + count + ")"
}

func displayWidth(s string) int {
	return lipgloss.Width(s)
}

func padRightDisplay(s string, width int) string {
	diff := width - displayWidth(s)
	if diff <= 0 {
		return s
	}
	return s + strings.Repeat(" ", diff)
}

func oneLineText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

func isGroupPeer(t apptg.PeerType) bool {
	return t == apptg.PeerChat || t == apptg.PeerChannel
}

func filterGroupDialogs(dialogs []apptg.Dialog) []apptg.Dialog {
	out := make([]apptg.Dialog, 0, len(dialogs))
	for _, d := range dialogs {
		if isGroupPeer(d.PeerType) {
			out = append(out, d)
		}
	}
	return out
}

func (m Model) selectedPeerID() (int64, bool) {
	if len(m.dialogs) == 0 || m.cursor < 0 || m.cursor >= len(m.dialogs) {
		return 0, false
	}
	return m.dialogs[m.cursor].PeerID, true
}

func (m *Model) applySort(selectedPeerID int64, preserveSelection bool) {
	sort.SliceStable(m.allDialogs, func(i, j int) bool {
		return lessDialogLatest(m.allDialogs[i], m.allDialogs[j])
	})
	m.rebuildVisibleDialogs(selectedPeerID, preserveSelection)
}

func (m *Model) rebuildVisibleDialogs(selectedPeerID int64, preserveSelection bool) {
	m.dialogs = m.dialogs[:0]
	for _, d := range m.allDialogs {
		if d.Archived == m.showArchived {
			m.dialogs = append(m.dialogs, d)
		}
	}

	if preserveSelection {
		for i := range m.dialogs {
			if m.dialogs[i].PeerID == selectedPeerID {
				m.cursor = i
				m.clampOffset()
				return
			}
		}
	}

	m.cursor = 0
	m.offset = 0
	m.clampOffset()
}

func lessDialogLatest(a, b apptg.Dialog) bool {
	if a.Archived != b.Archived {
		return !a.Archived
	}

	if !a.LastDate.Equal(b.LastDate) {
		return a.LastDate.After(b.LastDate)
	}
	if a.UnreadCount != b.UnreadCount {
		return a.UnreadCount > b.UnreadCount
	}

	at := strings.ToLower(a.Title)
	bt := strings.ToLower(b.Title)
	if at != bt {
		return at < bt
	}
	return a.PeerID < b.PeerID
}

// dialogStartLine returns the rendered top line index for dialog at idx.
func (m Model) dialogStartLine(idx int) int {
	if idx <= 0 {
		return 0
	}
	if idx >= len(m.dialogs) {
		return len(m.dialogs) - 1
	}
	return idx
}

func (m Model) totalContentLines() int {
	return len(m.dialogs)
}

func truncateDisplay(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if displayWidth(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	var b strings.Builder
	limit := max - 1
	current := 0
	for _, r := range s {
		rw := displayWidth(string(r))
		if current+rw > limit {
			break
		}
		b.WriteRune(r)
		current += rw
	}
	return b.String() + "…"
}

func fitDisplayWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) > width {
		s = truncateDisplay(s, width)
	}
	return padRightDisplay(s, width)
}

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#21252B")).
			Foreground(lipgloss.Color("#61AFEF"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2F3642")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D7DCE5")).
			Bold(true)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAB2C0")).
			Bold(true)

	userMarkerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Bold(true)

	groupMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#98C379")).
				Bold(true)

	channelMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5C07B")).
				Bold(true)

	unknownMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#5C6370")).
				Bold(true)

	unreadBadgeLowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#56B6C2")).
				Bold(true)

	unreadBadgeHighStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E06C75")).
				Bold(true)
)
