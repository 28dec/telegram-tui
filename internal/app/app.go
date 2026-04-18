package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
	"github.com/dxlongnh/telegram-tui/internal/ui/channellist"
	"github.com/dxlongnh/telegram-tui/internal/ui/chatview"
	"github.com/dxlongnh/telegram-tui/internal/ui/input"
	"github.com/dxlongnh/telegram-tui/internal/ui/media"
	"github.com/dxlongnh/telegram-tui/internal/ui/search"
	"github.com/gotd/td/telegram"
	gotdtg "github.com/gotd/td/tg"
)

// Model is the root bubbletea model.
type Model struct {
	mode       Mode
	activeView ActiveView
	authState  AuthState
	width      int
	height     int

	// Sub-models
	authInput   textinput.Model
	channelList channellist.Model
	chatView    chatview.Model
	msgInput    input.Model
	searchPopup search.Model
	mediaViewer media.Model

	// Active peer for chat view.
	activePeer           *gotdtg.InputPeerClass
	activePeerTitle      string
	activeReadInboxMaxID int

	// Startup/auth splash state.
	startupStatus   string
	showAuthPrompt  bool
	readyReceivedAt *time.Time
	startupSpinner  int

	// Telegram state
	program  *tea.Program
	tuiAuth  *apptg.TUIAuth
	api      *gotdtg.Client
	tgClient *telegram.Client
	selfID   int64
	selfName string
	err      error
}

// NewModel creates the initial app model.
func NewModel(tuiAuth *apptg.TUIAuth) *Model {
	ti := textinput.New()
	ti.Placeholder = "Phone number (e.g. +14155550100)"
	ti.CharLimit = 50

	return &Model{
		mode:          ModeAuth,
		authState:     AuthPhone,
		authInput:     ti,
		startupStatus: "Starting Telegram in background…",
		tuiAuth:       tuiAuth,
		channelList:   channellist.New(0, 0),
		msgInput:      input.New(0),
		searchPopup:   search.New(),
		mediaViewer:   media.New(0, 0),
	}
}

// SetProgram stores the program reference so sub-systems can call Send.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
	m.tuiAuth.SetProgram(p)
}

// Init starts Telegram in the background.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.authInput.Focus(),
		apptg.StartTelegram(m.program, m.tuiAuth),
		startupSpinnerTickCmd(),
		startupTickCmd(),
	)
}

// Update handles all incoming messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Always handle background Telegram updates regardless of current mode.
	// These must never be swallowed by mode-specific routing below.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		bodyHeight := m.contentHeight()
		m.channelList.SetSize(m.width, bodyHeight)
		m.chatView.SetSize(m.width, bodyHeight)
		m.mediaViewer.SetSize(m.width, m.height)
		return m, nil

	case apptg.StartupStatusMsg:
		m.startupStatus = msg.Text
		return m, nil

	case apptg.AuthPromptMsg:
		m.showAuthPrompt = true
		m.readyReceivedAt = nil
		switch msg.Kind {
		case apptg.AuthPromptPhone:
			m.authState = AuthPhone
			m.authInput.Placeholder = "Phone number (e.g. +14155550100)"
			m.startupStatus = "Login required: enter phone number"
		case apptg.AuthPromptCode:
			m.authState = AuthCode
			m.authInput.Placeholder = "Verification code"
			m.startupStatus = "Enter verification code"
		case apptg.AuthPromptPassword:
			m.authState = AuthPassword
			m.authInput.Placeholder = "2FA password (Enter to skip if none)"
			m.startupStatus = "Enter 2FA password"
		}
		m.authInput.Reset()
		return m, m.authInput.Focus()

	case apptg.TelegramReadyMsg:
		m.showAuthPrompt = false
		now := time.Now()
		m.readyReceivedAt = &now
		m.startupStatus = "Connected. Preparing workspace…"
		m.selfID = msg.Self.GetID()
		username, _ := msg.Self.GetUsername()
		if username == "" {
			username = fmt.Sprintf("%d", m.selfID)
		}
		m.selfName = username
		m.api = msg.API
		m.tgClient = msg.Client
		return m, nil

	case StartupTickMsg:
		if m.mode == ModeAuth && !m.showAuthPrompt && m.readyReceivedAt != nil {
			if time.Since(*m.readyReceivedAt) >= 3*time.Second {
				m.authState = AuthDone
				m.mode = ModeNormal
				m.activeView = ViewChannelList
				bodyHeight := m.contentHeight()
				m.chatView = chatview.New(m.width, bodyHeight, m.selfID)
				m.msgInput = input.New(m.width)
				m.mediaViewer = media.New(m.width, m.height)
				m.readyReceivedAt = nil
				return m, apptg.FetchDialogs(m.api)
			}
		}
		return m, startupTickCmd()

	case StartupSpinnerMsg:
		if m.mode == ModeAuth && !m.showAuthPrompt {
			m.startupSpinner = (m.startupSpinner + 1) % len(startupSpinnerFrames)
			return m, startupSpinnerTickCmd()
		}
		return m, nil

	case apptg.TelegramErrorMsg:
		m.err = msg.Err
		return m, nil

	case apptg.TelegramUpdateMsg:
		// Always process — must not be blocked by mode checks.
		return m.handleTelegramUpdate(msg)

	case apptg.FetchDialogsMsg:
		m.channelList.SetDialogs(msg.Dialogs)
		return m, nil

	case apptg.FetchDialogsErrMsg:
		m.err = msg.Err
		return m, nil

	case apptg.FetchHistoryMsg:
		if msg.Prepend {
			m.chatView.PrependMessages(msg.Messages)
		} else {
			m.chatView.SetMessages(msg.Messages)
		}
		return m, m.markCursorRead()

	case apptg.FetchHistoryErrMsg:
		m.err = msg.Err
		return m, nil

	case apptg.MediaReadyMsg:
		m.mediaViewer.SetReady(msg.Path)
		return m, nil

	case apptg.MediaErrMsg:
		m.mediaViewer.SetError(msg.Err)
		return m, nil

	case apptg.MessageSentMsg:
		id, date := extractSentMessage(msg.Updates)
		cm := apptg.ChatMessage{
			ID:       id,
			FromID:   m.selfID,
			FromName: m.selfName,
			Date:     date,
			Text:     msg.Text,
			Out:      true,
		}
		m.chatView.AppendMessageAndFollow(cm)
		return m, nil

	case apptg.MessageSendErrMsg:
		m.err = msg.Err
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	// Forward non-key messages to sub-models that need them (textinput, textarea ticking, etc.).
	if m.mode == ModeAuth {
		var cmd tea.Cmd
		m.authInput, cmd = m.authInput.Update(msg)
		return m, cmd
	}
	if m.mode == ModeSearch {
		updated, cmd, _, _ := m.searchPopup.Update(msg)
		m.searchPopup = updated
		return m, cmd
	}

	return m, nil
}

// handleTelegramUpdate routes real-time Telegram updates.
func (m *Model) handleTelegramUpdate(msg apptg.TelegramUpdateMsg) (tea.Model, tea.Cmd) {
	var updateList []gotdtg.UpdateClass
	var userList []gotdtg.UserClass

	switch u := msg.Raw.(type) {
	case *gotdtg.Updates:
		updateList = u.Updates
		userList = u.Users
	case *gotdtg.UpdatesCombined:
		updateList = u.Updates
		userList = u.Users
	case *gotdtg.UpdateShort:
		updateList = []gotdtg.UpdateClass{u.Update}
	default:
		return m, nil
	}

	// Build name map from the users bundled with this update batch.
	userNames := buildUserNames(userList)

	for _, u := range updateList {
		switch upd := u.(type) {
		case *gotdtg.UpdateNewMessage:
			if chatMsg, ok := upd.Message.(*gotdtg.Message); ok {
				m.applyNewMessage(chatMsg, userNames)
			}
		case *gotdtg.UpdateNewChannelMessage:
			if chatMsg, ok := upd.Message.(*gotdtg.Message); ok {
				m.applyNewMessage(chatMsg, userNames)
			}
		case *gotdtg.UpdateReadHistoryInbox:
			peerID := peerIDFromPeer(upd.Peer)
			m.channelList.ApplyReadUpdate(peerID, upd.GetMaxID(), upd.GetStillUnreadCount())
			if m.activeView == ViewChat && m.activePeer != nil && peerID == activePeerID(m.activePeer) {
				if upd.GetMaxID() > m.activeReadInboxMaxID {
					m.activeReadInboxMaxID = upd.GetMaxID()
				}
			}
		case *gotdtg.UpdateReadChannelInbox:
			m.channelList.ApplyReadUpdate(upd.ChannelID, upd.GetMaxID(), upd.GetStillUnreadCount())
			if m.activeView == ViewChat && m.activePeer != nil && upd.ChannelID == activePeerID(m.activePeer) {
				if upd.GetMaxID() > m.activeReadInboxMaxID {
					m.activeReadInboxMaxID = upd.GetMaxID()
				}
			}
		}
	}
	return m, nil
}

// buildUserNames builds an ID→name map from a slice of UserClass.
func buildUserNames(users []gotdtg.UserClass) map[int64]string {
	names := make(map[int64]string, len(users))
	for _, u := range users {
		user, ok := u.(*gotdtg.User)
		if !ok {
			continue
		}
		fn, _ := user.GetFirstName()
		ln, _ := user.GetLastName()
		name := fn
		if ln != "" {
			name += " " + ln
		}
		if name == "" {
			name = fmt.Sprintf("User%d", user.GetID())
		}
		names[user.GetID()] = name
	}
	return names
}

// applyNewMessage handles a newly arrived message:
// - appends it to the chat view if it belongs to the active chat
// - updates the dialog list entry (last message, date, unread count)
func (m *Model) applyNewMessage(chatMsg *gotdtg.Message, userNames map[int64]string) {
	peerID := peerIDFromMessage(chatMsg)
	date := apptg.TimeFromUnix(int64(chatMsg.GetDate()))
	text := chatMsg.GetMessage()

	// Resolve sender name.
	var fromID int64
	var fromName string
	if fromPeer, ok := chatMsg.GetFromID(); ok {
		switch f := fromPeer.(type) {
		case *gotdtg.PeerUser:
			fromID = f.GetUserID()
			if name, ok := userNames[fromID]; ok {
				fromName = name
			} else {
				fromName = fmt.Sprintf("User%d", fromID)
			}
		case *gotdtg.PeerChannel:
			fromID = f.GetChannelID()
			fromName = "Channel"
		}
	}
	if fromName == "" {
		fromName = "Unknown"
	}

	// Update dialog list for this peer (always, regardless of active view).
	// Don't increment unread for our own outgoing messages or the currently viewed chat.
	deltaUnread := 0
	if !chatMsg.GetOut() && !(m.activeView == ViewChat && peerID == activePeerID(m.activePeer)) {
		deltaUnread = 1
	}
	m.channelList.UpdateDialog(peerID, text, date, deltaUnread)

	// Append to chat view only if this is the currently open chat.
	if m.activeView == ViewChat && peerID == activePeerID(m.activePeer) {
		cm := apptg.ChatMessage{
			ID:       chatMsg.GetID(),
			FromID:   fromID,
			FromName: fromName,
			Date:     date,
			Text:     text,
			Out:      chatMsg.GetOut(),
		}
		m.chatView.AppendMessage(cm)
	}
}

// isActiveChat checks if a message belongs to the currently open chat.
func (m *Model) isActiveChat(msg *gotdtg.Message) bool {
	if m.activePeer == nil {
		return false
	}
	return peerIDFromMessage(msg) == activePeerID(m.activePeer)
}

// peerIDFromPeer extracts the numeric ID from a PeerClass.
func peerIDFromPeer(peer gotdtg.PeerClass) int64 {
	switch p := peer.(type) {
	case *gotdtg.PeerUser:
		return p.UserID
	case *gotdtg.PeerChat:
		return p.ChatID
	case *gotdtg.PeerChannel:
		return p.ChannelID
	}
	return 0
}

// peerIDFromMessage extracts the numeric peer ID from a message's PeerID field.
func peerIDFromMessage(msg *gotdtg.Message) int64 {
	switch p := msg.PeerID.(type) {
	case *gotdtg.PeerUser:
		return p.UserID
	case *gotdtg.PeerChat:
		return p.ChatID
	case *gotdtg.PeerChannel:
		return p.ChannelID
	}
	return 0
}

// activePeerID extracts the numeric ID from an InputPeerClass.
func activePeerID(peer *gotdtg.InputPeerClass) int64 {
	switch p := (*peer).(type) {
	case *gotdtg.InputPeerUser:
		return p.UserID
	case *gotdtg.InputPeerChat:
		return p.ChatID
	case *gotdtg.InputPeerChannel:
		return p.ChannelID
	}
	return 0
}

// handleKey dispatches key events based on current mode.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.mode {
	case ModeAuth:
		return m.handleAuthKey(msg)
	case ModeNormal:
		return m.handleNormalKey(msg)
	case ModeInput, ModeInputMultiline, ModeReply, ModeReplyMultiline:
		return m.handleInputKey(msg)
	case ModeSearch:
		return m.handleSearchKey(msg)
	case ModeMedia:
		return m.handleMediaKey(msg)
	}
	return m, nil
}

// handleAuthKey handles key events during authentication.
func (m *Model) handleAuthKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.showAuthPrompt {
		return m, nil
	}
	if msg.String() == "enter" {
		value := strings.TrimSpace(m.authInput.Value())
		switch m.authState {
		case AuthPhone:
			if value == "" {
				return m, nil
			}
			m.tuiAuth.SendPhone(value)
			m.startupStatus = "Requesting verification code…"
		case AuthCode:
			if value == "" {
				return m, nil
			}
			m.tuiAuth.SendCode(value)
			m.startupStatus = "Verifying code…"
		case AuthPassword:
			if value == "" {
				value = " " // gotd uses empty/space to skip when no 2FA.
			}
			m.tuiAuth.SendPassword(value)
			m.startupStatus = "Signing in…"
		}
		m.authInput.Reset()
		return m, nil
	}

	var cmd tea.Cmd
	m.authInput, cmd = m.authInput.Update(msg)
	return m, cmd
}

// handleNormalKey handles navigation and action keys in normal mode.
func (m *Model) handleNormalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.activeView {
	case ViewChannelList:
		switch msg.String() {
		case "j":
			m.channelList.MoveDown(1)
		case "k":
			m.channelList.MoveUp(1)
		case "J":
			m.channelList.MoveDown(10)
		case "K":
			m.channelList.MoveUp(10)
		case "a":
			m.channelList.ToggleArchived()
		case "enter":
			if dlg, ok := m.channelList.Selected(); ok {
				m.activeView = ViewChat
				peer := dlg.InputPeer
				m.activePeer = &peer
				m.activePeerTitle = dlg.Title
				m.activeReadInboxMaxID = dlg.ReadInboxMaxID
				m.chatView = chatview.New(m.width, m.contentHeight(), m.selfID)
				m.chatView.SetPeer(dlg.Title)
				return m, apptg.FetchHistory(m.api, dlg.InputPeer, m.selfID, 0, dlg.ReadInboxMaxID)
			}
		case "/":
			cmd := m.searchPopup.Open(m.channelList.AllDialogs(), m.width, m.height)
			m.mode = ModeSearch
			return m, cmd
		}

	case ViewChat:
		switch msg.String() {
		case "j":
			m.chatView.MoveCursor(1)
			return m, m.markCursorRead()
		case "k":
			_, cmd := m.moveCursorUp(1)
			return m, tea.Batch(cmd, m.markCursorRead())
		case "J":
			m.chatView.MoveCursor(10)
			return m, m.markCursorRead()
		case "K":
			_, cmd := m.moveCursorUp(10)
			return m, tea.Batch(cmd, m.markCursorRead())
		case "esc":
			if m.activePeer != nil && m.activeReadInboxMaxID > 0 {
				m.channelList.SetReadInboxMax(activePeerID(m.activePeer), m.activeReadInboxMaxID)
			}
			m.activeView = ViewChannelList
		case "i":
			cmd := m.msgInput.Activate(input.Inline, nil, m.width)
			m.mode = ModeInput
			return m, cmd
		case "I":
			cmd := m.msgInput.Activate(input.Multiline, nil, m.width)
			m.mode = ModeInputMultiline
			return m, cmd
		case "r":
			cmd := m.msgInput.Activate(input.Reply, m.chatView.CursorMessageID(), m.width)
			m.mode = ModeReply
			return m, cmd
		case "R":
			cmd := m.msgInput.Activate(input.ReplyMulti, m.chatView.CursorMessageID(), m.width)
			m.mode = ModeReplyMultiline
			return m, cmd
		case "/":
			cmd := m.searchPopup.Open(m.channelList.AllDialogs(), m.width, m.height)
			m.mode = ModeSearch
			return m, cmd
		}
	}
	return m, nil
}

// handleInputKey delegates to the input sub-model and fires send if needed.
func (m *Model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	updated, cmd, shouldSend := m.msgInput.Update(msg)
	m.msgInput = updated

	if !m.msgInput.Active() {
		m.mode = ModeNormal
		return m, cmd
	}

	if shouldSend && m.activePeer != nil {
		text := m.msgInput.Value()
		replyToID := m.msgInput.ReplyToID() // read before Deactivate clears it
		m.msgInput.Deactivate()
		m.mode = ModeNormal
		return m, apptg.SendMessage(m.api, *m.activePeer, text, replyToID)
	}

	return m, cmd
}

// handleMediaKey handles keys while the media viewer is shown.
func (m *Model) handleMediaKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
	case "enter":
		if path := m.mediaViewer.Path(); path != "" {
			// Open in system viewer (macOS: open, Linux: xdg-open).
			return m, openExternalCmd(path)
		}
	}
	return m, nil
}

// handleSearchKey delegates to the search popup sub-model.
func (m *Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	updated, cmd, selected, closed := m.searchPopup.Update(msg)
	m.searchPopup = updated

	if closed {
		m.mode = ModeNormal
		if selected != nil {
			// Open the selected dialog.
			peer := selected.InputPeer
			m.activePeer = &peer
			m.activePeerTitle = selected.Title
			m.activeReadInboxMaxID = selected.ReadInboxMaxID
			m.activeView = ViewChat
			m.chatView = chatview.New(m.width, m.contentHeight(), m.selfID)
			m.chatView.SetPeer(selected.Title)
			return m, apptg.FetchHistory(m.api, selected.InputPeer, m.selfID, 0, selected.ReadInboxMaxID)
		}
	}
	return m, cmd
}

// View renders the current screen.
func (m *Model) View() tea.View {
	var content string

	if m.err != nil {
		content = errorStyle.Render("Error: " + m.err.Error())
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	switch m.mode {
	case ModeAuth:
		content = m.authView()
	default:
		content = m.mainView()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *Model) authView() string {
	center := lipgloss.NewStyle().Width(maxInt(1, m.width)).Align(lipgloss.Center)
	artLines := strings.Split(strings.TrimSpace(telegramSplashASCII), "\n")
	status := m.startupStatus
	if status == "" {
		status = "Starting…"
	}

	var lines []string
	for _, line := range artLines {
		lines = append(lines, center.Render(splashTitleStyle.Render(line)))
	}
	lines = append(lines, "")

	if !m.showAuthPrompt {
		spinner := startupSpinnerFrames[m.startupSpinner%len(startupSpinnerFrames)]
		lines = append(lines, center.Render(statusStyle.Render(spinner+"  "+status)))
		lines = append(lines, center.Render(hintStyle.Render("Background checks are running…")))
		body := strings.Join(lines, "\n")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
	}

	var prompt string
	switch m.authState {
	case AuthPhone:
		prompt = "Enter your phone number"
	case AuthCode:
		prompt = "Enter verification code"
	case AuthPassword:
		prompt = "Enter 2FA password (or press Enter to skip)"
	default:
		prompt = "Authentication"
	}

	lines = append(lines, center.Render(statusStyle.Render(status)))
	lines = append(lines, "")
	lines = append(lines, center.Render(inputPromptStyle.Render(prompt)))
	lines = append(lines, center.Render(m.authInput.View()))
	lines = append(lines, "")
	lines = append(lines, center.Render(hintStyle.Render("Press Enter to confirm • Ctrl+C to quit")))

	body := strings.Join(lines, "\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

func (m *Model) renderHeader() string {
	var line1, line2 string

	switch m.activeView {
	case ViewChannelList:
		user := m.selfName
		if user == "" {
			user = "guest"
		}
		line1 = "User: " + user
		line2 = fmt.Sprintf("User ID: %d", m.selfID)

	case ViewChat:
		title := oneLine(m.activePeerTitle)
		if title == "" {
			title = "(unknown thread)"
		}
		peerID := activePeerID(m.activePeer)
		line1 = title
		line2 = fmt.Sprintf("%s • ID: %d", activePeerKind(m.activePeer), peerID)

	default:
		line1 = "Telegram TUI"
		line2 = ""
	}

	content := truncateForHeader(line1, m.width-2) + "\n" + truncateForHeader(line2, m.width-2)
	return headerBarStyle.Width(m.width).Align(lipgloss.Center).Render(content)
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

func truncateForHeader(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	r := []rune(s)
	if len(r) >= max {
		return string(r[:max-1]) + "…"
	}
	return s
}

func (m *Model) findDialogByPeerID(peerID int64) (apptg.Dialog, bool) {
	for _, d := range m.channelList.Dialogs() {
		if d.PeerID == peerID {
			return d, true
		}
	}
	return apptg.Dialog{}, false
}

func (m *Model) renderChatUnreadBar() string {
	if m.activeView != ViewChat || m.activePeer == nil {
		return ""
	}
	peerID := activePeerID(m.activePeer)
	unread := 0
	if d, ok := m.findDialogByPeerID(peerID); ok {
		unread = d.UnreadCount
	}
	if unread <= 0 {
		return lipgloss.NewStyle().Width(m.width).Render("")
	}
	label := fmt.Sprintf("Unread: %d", unread)
	return unreadBarStyle.Width(m.width).Align(lipgloss.Center).Render(label)
}

func (m *Model) renderFooter() string {
	var guide string
	switch m.activeView {
	case ViewChannelList:
		guide = "j/k: move • J/K: jump • a: archived/main • Enter: open • /: search • Ctrl+C: quit"
	case ViewChat:
		guide = "j/k: scroll • i/I: message • r/R: reply • /: search • Esc: back • Ctrl+C: quit"
	default:
		guide = "Ctrl+C: quit"
	}
	return footerBarStyle.Width(m.width).Align(lipgloss.Center).Render(guide)
}

func (m *Model) contentHeight() int {
	// Base global chrome: 2 header lines + header border + footer line + footer border.
	h := m.height - 5
	if m.activeView == ViewChat {
		// Reserve static typing row + unread row above footer in chat view.
		h -= 2
	}
	if h < 1 {
		return 1
	}
	return h
}

func activePeerKind(peer *gotdtg.InputPeerClass) string {
	if peer == nil {
		return "Chat"
	}
	switch (*peer).(type) {
	case *gotdtg.InputPeerChannel:
		return "Group"
	case *gotdtg.InputPeerChat:
		return "Group"
	case *gotdtg.InputPeerUser:
		return "User"
	default:
		return "Chat"
	}
}

func (m *Model) mainView() string {
	if m.mode == ModeMedia {
		return m.mediaViewer.View()
	}
	if m.mode == ModeSearch {
		return m.searchPopup.View()
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	switch m.activeView {
	case ViewChannelList:
		return header + "\n" + m.channelList.View() + "\n" + footer
	case ViewChat:
		inputArea := m.msgInput.View()
		if !m.msgInput.Active() {
			inputArea = lipgloss.NewStyle().Width(m.width).Render("")
		}
		unreadBar := m.renderChatUnreadBar()
		return header + "\n" + m.chatView.View() + "\n" + inputArea + "\n" + unreadBar + "\n" + footer
	}
	return header + "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render("Connecting…") + "\n" + footer
}

const telegramSplashASCII = `
████████╗███████╗██╗     ███████╗ ██████╗ ██████╗  █████╗ ███╗   ███╗
╚══██╔══╝██╔════╝██║     ██╔════╝██╔════╝ ██╔══██╗██╔══██╗████╗ ████║
   ██║   █████╗  ██║     █████╗  ██║  ███╗██████╔╝███████║██╔████╔██║
   ██║   ██╔══╝  ██║     ██╔══╝  ██║   ██║██╔══██╗██╔══██║██║╚██╔╝██║
   ██║   ███████╗███████╗███████╗╚██████╔╝██║  ██║██║  ██║██║ ╚═╝ ██║
   ╚═╝   ╚══════╝╚══════╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝

T E R M I N A L   U I
`

var (
	splashTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#61AFEF"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ABB2BF"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370")).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D7DCE5")).
			Bold(true)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ABB2BF")).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75")).
			Bold(true)

	headerBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#21252B")).
			Foreground(lipgloss.Color("#7CC7FF")).
			Bold(true).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3E4451"))

	footerBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#21252B")).
			Foreground(lipgloss.Color("#D7DCE5")).
			Bold(true).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#3E4451"))

	unreadBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD166")).
			Bold(true).
			PaddingRight(1)
)

func (m *Model) markCursorRead() tea.Cmd {
	if m.activePeer == nil {
		return nil
	}
	cursorMsg, ok := m.chatView.CursorMessage()
	if !ok || !cursorMsg.Unread || cursorMsg.Out {
		return nil
	}

	changed := m.chatView.MarkReadThrough(cursorMsg.ID)
	if changed <= 0 {
		return nil
	}

	peerID := activePeerID(m.activePeer)
	m.channelList.ConsumeUnread(peerID, changed)
	if cursorMsg.ID > m.activeReadInboxMaxID {
		m.activeReadInboxMaxID = cursorMsg.ID
	}
	m.channelList.SetReadInboxMax(peerID, m.activeReadInboxMaxID)
	return apptg.ReadHistory(m.api, *m.activePeer, cursorMsg.ID)
}

// moveCursorUp moves the chat cursor up and loads older history when it reaches the top.
func (m *Model) moveCursorUp(n int) (tea.Model, tea.Cmd) {
	atTop := m.chatView.MoveCursor(-n)
	if atTop && m.activePeer != nil {
		oldestID := m.chatView.OldestMessageID()
		return m, apptg.FetchHistory(m.api, *m.activePeer, m.selfID, oldestID, m.activeReadInboxMaxID)
	}
	return m, nil
}

// extractSentMessage pulls the message ID and timestamp from a send response.
// Falls back to ID=0 and now if the response type is unexpected.
func extractSentMessage(u gotdtg.UpdatesClass) (id int, date time.Time) {
	date = time.Now()
	if u == nil {
		return
	}
	var updates []gotdtg.UpdateClass
	switch v := u.(type) {
	case *gotdtg.Updates:
		updates = v.Updates
	case *gotdtg.UpdatesCombined:
		updates = v.Updates
	case *gotdtg.UpdateShort:
		updates = []gotdtg.UpdateClass{v.Update}
	}
	for _, upd := range updates {
		switch v := upd.(type) {
		case *gotdtg.UpdateMessageID:
			id = v.ID
		case *gotdtg.UpdateNewMessage:
			if m, ok := v.Message.(*gotdtg.Message); ok {
				id = m.GetID()
				date = time.Unix(int64(m.GetDate()), 0)
			}
		case *gotdtg.UpdateNewChannelMessage:
			if m, ok := v.Message.(*gotdtg.Message); ok {
				id = m.GetID()
				date = time.Unix(int64(m.GetDate()), 0)
			}
		}
	}
	return
}

var startupSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func startupTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return StartupTickMsg{} })
}

func startupSpinnerTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return StartupSpinnerMsg{} })
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// openExternalCmd returns a tea.Cmd that opens a file in the system viewer.
func openExternalCmd(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", path)
		default:
			cmd = exec.Command("xdg-open", path)
		}
		_ = cmd.Start()
		return nil
	}
}
