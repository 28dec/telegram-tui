package tg

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	gotdtg "github.com/gotd/td/tg"
)

// FetchDialogsMsg is returned by FetchDialogs on success.
type FetchDialogsMsg struct {
	Dialogs []Dialog
}

// FetchDialogsErrMsg is returned by FetchDialogs on failure.
type FetchDialogsErrMsg struct {
	Err error
}

// FetchDialogs returns a tea.Cmd that fetches the dialog list.
func FetchDialogs(api *gotdtg.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := api.MessagesGetDialogs(ctx, &gotdtg.MessagesGetDialogsRequest{
			OffsetPeer: &gotdtg.InputPeerEmpty{},
			Limit:      100,
		})
		if err != nil {
			return FetchDialogsErrMsg{Err: err}
		}

		dialogs, err := parseDialogs(result)
		if err != nil {
			return FetchDialogsErrMsg{Err: err}
		}
		return FetchDialogsMsg{Dialogs: dialogs}
	}
}

// parseDialogs converts a MessagesDialogsClass into our Dialog slice.
func parseDialogs(raw gotdtg.MessagesDialogsClass) ([]Dialog, error) {
	var (
		dlgList  []gotdtg.DialogClass
		msgList  []gotdtg.MessageClass
		chatList []gotdtg.ChatClass
		userList []gotdtg.UserClass
	)

	switch d := raw.(type) {
	case *gotdtg.MessagesDialogs:
		dlgList = d.Dialogs
		msgList = d.Messages
		chatList = d.Chats
		userList = d.Users
	case *gotdtg.MessagesDialogsSlice:
		dlgList = d.Dialogs
		msgList = d.Messages
		chatList = d.Chats
		userList = d.Users
	case *gotdtg.MessagesDialogsNotModified:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown dialogs type: %T", raw)
	}

	// Build lookup maps by ID.
	chatMap := make(map[int64]gotdtg.ChatClass, len(chatList))
	for _, c := range chatList {
		chatMap[c.GetID()] = c
	}
	userMap := make(map[int64]gotdtg.UserClass, len(userList))
	for _, u := range userList {
		userMap[u.GetID()] = u
	}
	// Map message by ID for last-message preview.
	msgByID := make(map[int]gotdtg.MessageClass, len(msgList))
	for _, msg := range msgList {
		switch m := msg.(type) {
		case *gotdtg.Message:
			msgByID[m.GetID()] = m
		}
	}

	out := make([]Dialog, 0, len(dlgList))
	for _, d := range dlgList {
		dlg, ok := d.(*gotdtg.Dialog)
		if !ok {
			continue
		}

		dialog := Dialog{
			UnreadCount: dlg.GetUnreadCount(),
		}
		dialog.ReadInboxMaxID = dlg.GetReadInboxMaxID()
		if folderID, ok := dlg.GetFolderID(); ok && folderID == 1 {
			dialog.Archived = true
		}

		switch peer := dlg.GetPeer().(type) {
		case *gotdtg.PeerUser:
			dialog.PeerID = peer.GetUserID()
			dialog.PeerType = PeerUser
			dialog.InputPeer = &gotdtg.InputPeerUser{
				UserID: peer.GetUserID(),
			}
			if u, ok := userMap[peer.GetUserID()]; ok {
				if user, ok := u.(*gotdtg.User); ok {
					fn, _ := user.GetFirstName()
					ln, _ := user.GetLastName()
					dialog.Title = fn + " " + ln
					if hash, ok := user.GetAccessHash(); ok {
						dialog.InputPeer = &gotdtg.InputPeerUser{
							UserID:     peer.GetUserID(),
							AccessHash: hash,
						}
					}
				}
			}

		case *gotdtg.PeerChat:
			dialog.PeerID = peer.GetChatID()
			dialog.PeerType = PeerChat
			dialog.InputPeer = &gotdtg.InputPeerChat{ChatID: peer.GetChatID()}
			if c, ok := chatMap[peer.GetChatID()]; ok {
				if chat, ok := c.(*gotdtg.Chat); ok {
					dialog.Title = chat.GetTitle()
				}
			}

		case *gotdtg.PeerChannel:
			dialog.PeerID = peer.GetChannelID()
			dialog.PeerType = PeerChannel
			dialog.InputPeer = &gotdtg.InputPeerChannel{ChannelID: peer.GetChannelID()}
			if c, ok := chatMap[peer.GetChannelID()]; ok {
				if ch, ok := c.(*gotdtg.Channel); ok {
					dialog.Title = ch.GetTitle()
					if hash, ok := ch.GetAccessHash(); ok {
						dialog.InputPeer = &gotdtg.InputPeerChannel{
							ChannelID:  peer.GetChannelID(),
							AccessHash: hash,
						}
					}
				}
			}
		}

		if dialog.Title == "" {
			dialog.Title = fmt.Sprintf("ID:%d", dialog.PeerID)
		}

		// Last message preview.
		if msg, ok := msgByID[dlg.GetTopMessage()]; ok {
			if m, ok := msg.(*gotdtg.Message); ok {
				dialog.LastMessage = truncate(m.GetMessage(), 40)
				dialog.LastDate = time.Unix(int64(m.GetDate()), 0)
			}
		}

		out = append(out, dialog)
	}
	return out, nil
}

// truncate shortens s to maxLen runes, appending "…" if cut.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
