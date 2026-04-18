package tg

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/gotd/td/telegram"
	gotdtg "github.com/gotd/td/tg"
)

// TelegramReadyMsg is sent when Telegram connects and authenticates.
type TelegramReadyMsg struct {
	API    *gotdtg.Client
	Self   *gotdtg.User
	Client *telegram.Client // raw client for downloads
}

// TelegramErrorMsg is sent when the Telegram client encounters a fatal error.
type TelegramErrorMsg struct {
	Err error
}

// TelegramUpdateMsg carries raw Telegram updates from the Bridge.
type TelegramUpdateMsg struct {
	Raw gotdtg.UpdatesClass
}

// Bridge implements telegram.UpdateHandler by forwarding all updates
// into the bubbletea program as TelegramUpdateMsg.
type Bridge struct {
	program *tea.Program
}

// NewBridge creates a Bridge bound to the given program.
func NewBridge(p *tea.Program) *Bridge {
	return &Bridge{program: p}
}

// Handle satisfies telegram.UpdateHandler. Called from gotd's goroutine.
func (b *Bridge) Handle(_ context.Context, u gotdtg.UpdatesClass) error {
	b.program.Send(TelegramUpdateMsg{Raw: u})
	return nil
}

// StartupStatusMsg communicates background startup progress to the UI splash.
type StartupStatusMsg struct {
	Text string
}

// AuthPromptKind identifies which credential is currently required.
type AuthPromptKind int

const (
	AuthPromptPhone AuthPromptKind = iota
	AuthPromptCode
	AuthPromptPassword
)

// AuthPromptMsg tells the UI which auth prompt to display.
type AuthPromptMsg struct {
	Kind AuthPromptKind
}
