package tg

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/gotd/td/telegram/auth"
	gotdtg "github.com/gotd/td/tg"
)

// TUIAuth implements auth.UserAuthenticator by blocking on channels
// that the TUI fills when the user provides input.
type TUIAuth struct {
	phoneCh    chan string
	codeCh     chan string
	passwordCh chan string
	program    *tea.Program
}

// NewTUIAuth creates a TUIAuth ready to use.
func NewTUIAuth() *TUIAuth {
	return &TUIAuth{
		phoneCh:    make(chan string, 1),
		codeCh:     make(chan string, 1),
		passwordCh: make(chan string, 1),
	}
}

// SendPhone delivers the phone number from TUI to the waiting auth flow.
func (a *TUIAuth) SendPhone(phone string) { a.phoneCh <- phone }

// SendCode delivers the SMS/app code from TUI to the waiting auth flow.
func (a *TUIAuth) SendCode(code string) { a.codeCh <- code }

// SendPassword delivers the 2FA password from TUI to the waiting auth flow.
func (a *TUIAuth) SendPassword(password string) { a.passwordCh <- password }

// Phone implements auth.UserAuthenticator.
func (a *TUIAuth) Phone(ctx context.Context) (string, error) {
	a.notifyPrompt(AuthPromptPhone)
	select {
	case phone := <-a.phoneCh:
		return phone, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Password implements auth.UserAuthenticator.
func (a *TUIAuth) Password(ctx context.Context) (string, error) {
	a.notifyPrompt(AuthPromptPassword)
	select {
	case pw := <-a.passwordCh:
		return pw, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Code implements auth.UserAuthenticator (via CodeAuthenticator).
func (a *TUIAuth) Code(ctx context.Context, _ *gotdtg.AuthSentCode) (string, error) {
	a.notifyPrompt(AuthPromptCode)
	select {
	case code := <-a.codeCh:
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// AcceptTermsOfService implements auth.UserAuthenticator — auto-accept.
func (a *TUIAuth) AcceptTermsOfService(_ context.Context, _ gotdtg.HelpTermsOfService) error {
	return nil
}

// SignUp implements auth.UserAuthenticator — not supported; return error.
func (a *TUIAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, &auth.SignUpRequired{}
}

// SetProgram attaches the Bubble Tea program so auth prompts can be signaled.
func (a *TUIAuth) SetProgram(p *tea.Program) { a.program = p }

func (a *TUIAuth) notifyPrompt(kind AuthPromptKind) {
	if a.program != nil {
		a.program.Send(AuthPromptMsg{Kind: kind})
	}
}
