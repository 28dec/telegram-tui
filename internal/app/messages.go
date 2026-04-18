package app

import (
	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
)

// Mode represents the current input/interaction mode.
type Mode int

const (
	ModeAuth           Mode = iota // waiting for phone/code/password
	ModeNormal                     // navigating list or chat
	ModeInput                      // inline message input
	ModeInputMultiline             // multiline message input
	ModeReply                      // inline reply input
	ModeReplyMultiline             // multiline reply input
	ModeSearch                     // fuzzy search popup
	ModeMedia                      // media viewer overlay
)

// ActiveView tracks which top-level view is shown.
type ActiveView int

const (
	ViewChannelList ActiveView = iota
	ViewChat
)

// AuthState tracks which auth prompt is active.
type AuthState int

const (
	AuthPhone    AuthState = iota // waiting for user to enter phone
	AuthCode                      // waiting for SMS/app code
	AuthPassword                  // waiting for 2FA password
	AuthDone                      // authenticated
)

// ---- Custom tea.Msg types ----

// MessageSentMsg confirms a message was sent.
type MessageSentMsg struct{}

// MessageSendErrorMsg carries a send failure.
type MessageSendErrorMsg struct {
	Err error
}

// SelectDialogMsg is sent when the user selects a dialog.
type SelectDialogMsg struct {
	Dialog apptg.Dialog
}

// MediaDownloadedMsg carries the local path of a downloaded media file.
type MediaDownloadedMsg struct {
	Path string
	Info apptg.MediaInfo
}

// StartupTickMsg advances splash spinner/progress during startup.
type StartupTickMsg struct{}

// StartupSpinnerMsg advances spinner frame while startup splash is shown.
type StartupSpinnerMsg struct{}
