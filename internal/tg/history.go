package tg

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	gotdtg "github.com/gotd/td/tg"
)

// FetchHistoryMsg is returned by FetchHistory on success.
type FetchHistoryMsg struct {
	Messages []ChatMessage
	HasMore  bool
	Prepend  bool // true when fetching older messages (pagination), false for initial load
}

// FetchHistoryErrMsg is returned by FetchHistory on failure.
type FetchHistoryErrMsg struct {
	Err error
}

const historyBatchSize = 50

// FetchHistory returns a tea.Cmd that fetches message history for a peer.
// offsetID = 0 performs initial load. If readInboxMaxID > 0, initial load
// scans older pages so the oldest unread message can be included.
// For pagination, pass offsetID as the oldest loaded message ID.
func FetchHistory(api *gotdtg.Client, peer gotdtg.InputPeerClass, selfID int64, offsetID int, readInboxMaxID int) tea.Cmd {
	return func() tea.Msg {
		if offsetID != 0 {
			msgs, hasMore, err := fetchHistoryBatch(api, peer, selfID, offsetID, readInboxMaxID)
			if err != nil {
				return FetchHistoryErrMsg{Err: err}
			}
			return FetchHistoryMsg{Messages: msgs, HasMore: hasMore, Prepend: true}
		}

		msgs, hasMore, err := fetchHistoryBatch(api, peer, selfID, 0, readInboxMaxID)
		if err != nil {
			return FetchHistoryErrMsg{Err: err}
		}

		// Initial open: if chat has unread, load older pages until oldest unread
		// is in range (or we hit limits).
		if readInboxMaxID > 0 {
			const maxBatches = 20
			batches := 1
			for hasMore && len(msgs) > 0 && msgs[0].ID > readInboxMaxID+1 && batches < maxBatches {
				older, olderHasMore, err := fetchHistoryBatch(api, peer, selfID, msgs[0].ID, readInboxMaxID)
				if err != nil {
					return FetchHistoryErrMsg{Err: err}
				}
				if len(older) == 0 {
					break
				}
				msgs = append(older, msgs...)
				hasMore = olderHasMore
				batches++
			}
		}

		return FetchHistoryMsg{Messages: msgs, HasMore: hasMore, Prepend: false}
	}
}

func fetchHistoryBatch(api *gotdtg.Client, peer gotdtg.InputPeerClass, selfID int64, offsetID int, readInboxMaxID int) ([]ChatMessage, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := api.MessagesGetHistory(ctx, &gotdtg.MessagesGetHistoryRequest{
		Peer:     peer,
		OffsetID: offsetID,
		Limit:    historyBatchSize,
	})
	if err != nil {
		return nil, false, err
	}

	msgs, _, hasMore := parseHistory(result, selfID, readInboxMaxID)
	return msgs, hasMore, nil
}

// parseHistory extracts ChatMessage slice from a MessagesMessagesClass result.
func parseHistory(raw gotdtg.MessagesMessagesClass, selfID int64, readInboxMaxID int) ([]ChatMessage, map[int64]string, bool) {
	var (
		msgList  []gotdtg.MessageClass
		userList []gotdtg.UserClass
		count    int
	)

	switch r := raw.(type) {
	case *gotdtg.MessagesMessages:
		msgList = r.Messages
		userList = r.Users
		count = len(r.Messages)
	case *gotdtg.MessagesMessagesSlice:
		msgList = r.Messages
		userList = r.Users
		count = r.Count
	case *gotdtg.MessagesChannelMessages:
		msgList = r.Messages
		userList = r.Users
		count = r.Count
	case *gotdtg.MessagesMessagesNotModified:
		return nil, nil, false
	}

	userNames := make(map[int64]string, len(userList))
	for _, u := range userList {
		if user, ok := u.(*gotdtg.User); ok {
			fn, _ := user.GetFirstName()
			ln, _ := user.GetLastName()
			name := fn
			if ln != "" {
				name += " " + ln
			}
			if name == "" {
				name = fmt.Sprintf("User%d", user.GetID())
			}
			userNames[user.GetID()] = name
		}
	}

	out := make([]ChatMessage, 0, len(msgList))
	for _, raw := range msgList {
		m, ok := raw.(*gotdtg.Message)
		if !ok {
			continue
		}

		cm := ChatMessage{
			ID:   m.GetID(),
			Date: time.Unix(int64(m.GetDate()), 0),
			Text: m.GetMessage(),
			Out:  m.GetOut(),
		}

		// Sender ID + name.
		if fromPeer, ok := m.GetFromID(); ok {
			switch f := fromPeer.(type) {
			case *gotdtg.PeerUser:
				cm.FromID = f.GetUserID()
				cm.Out = cm.Out || (cm.FromID == selfID)
				if name, ok := userNames[cm.FromID]; ok {
					cm.FromName = name
				} else {
					cm.FromName = fmt.Sprintf("User%d", cm.FromID)
				}
			case *gotdtg.PeerChannel:
				cm.FromID = f.GetChannelID()
				cm.FromName = "Channel"
			}
		}
		if cm.FromName == "" {
			cm.FromName = "Unknown"
		}

		if readInboxMaxID > 0 && !cm.Out && cm.ID > readInboxMaxID {
			cm.Unread = true
		}

		// Edit date.
		if ed, ok := m.GetEditDate(); ok {
			t := time.Unix(int64(ed), 0)
			cm.EditDate = &t
		}

		// Reply-to.
		if replyTo, ok := m.GetReplyTo(); ok {
			if rh, ok := replyTo.(*gotdtg.MessageReplyHeader); ok {
				if id, ok := rh.GetReplyToMsgID(); ok {
					cm.ReplyToID = &id
				}
			}
		}

		// Media.
		if media, ok := m.GetMedia(); ok {
			cm.Media = extractMedia(media, m.GetID())
		}

		out = append(out, cm)
	}

	// Messages come newest-first from the API; reverse to chronological order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	hasMore := count > len(msgList)
	return out, userNames, hasMore
}

func extractMedia(media gotdtg.MessageMediaClass, msgID int) *MediaInfo {
	info := &MediaInfo{MessageID: msgID}
	switch m := media.(type) {
	case *gotdtg.MessageMediaPhoto:
		info.Type = MediaPhoto
		info.FileName = "photo"
	case *gotdtg.MessageMediaDocument:
		info.Type = MediaDocument
		if doc, ok := m.GetDocument(); ok {
			if d, ok := doc.(*gotdtg.Document); ok {
				info.Size = d.GetSize()
				info.MimeType = d.GetMimeType()
				for _, attr := range d.GetAttributes() {
					if fn, ok := attr.(*gotdtg.DocumentAttributeFilename); ok {
						info.FileName = fn.GetFileName()
					}
					if _, ok := attr.(*gotdtg.DocumentAttributeVideo); ok {
						info.Type = MediaVideo
					}
					if _, ok := attr.(*gotdtg.DocumentAttributeAudio); ok {
						info.Type = MediaAudio
					}
				}
				if info.FileName == "" {
					info.FileName = "document"
				}
			}
		}
	default:
		info.Type = MediaOther
		info.FileName = "media"
	}
	return info
}
