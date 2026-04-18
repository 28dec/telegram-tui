package tg

import (
	"context"
	"math/rand"
	"time"

	tea "charm.land/bubbletea/v2"
	gotdtg "github.com/gotd/td/tg"
)

// MessageSentMsg is sent after a message is successfully sent.
type MessageSentMsg struct {
	Updates gotdtg.UpdatesClass
	Text    string
}

// MessageSendErrMsg is sent when message sending fails.
type MessageSendErrMsg struct {
	Err error
}

// SendMessage returns a tea.Cmd that sends a text message to the given peer.
// If replyToID is non-nil, the message is a reply.
func SendMessage(api *gotdtg.Client, peer gotdtg.InputPeerClass, text string, replyToID *int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &gotdtg.MessagesSendMessageRequest{
			Peer:     peer,
			Message:  text,
			RandomID: rand.Int63(),
		}
		if replyToID != nil {
			req.SetReplyTo(&gotdtg.InputReplyToMessage{
				ReplyToMsgID: *replyToID,
			})
		}

		result, err := api.MessagesSendMessage(ctx, req)
		if err != nil {
			return MessageSendErrMsg{Err: err}
		}
		return MessageSentMsg{Updates: result, Text: text}
	}
}
