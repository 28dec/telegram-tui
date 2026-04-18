package tg

import (
	"time"

	gotdtg "github.com/gotd/td/tg"
)

// TimeFromUnix converts a Unix timestamp to time.Time.
func TimeFromUnix(unix int64) time.Time {
	return time.Unix(unix, 0)
}

// PeerType distinguishes dialog types.
type PeerType int

const (
	PeerUser PeerType = iota
	PeerChat
	PeerChannel
)

// Dialog represents a chat entry in the dialog list.
type Dialog struct {
	PeerID         int64
	PeerType       PeerType
	Title          string
	UnreadCount    int
	ReadInboxMaxID int
	LastMessage    string
	LastDate       time.Time
	Archived       bool                  // true when FolderID == 1
	InputPeer      gotdtg.InputPeerClass // needed for API calls; stores access hash
}

// MediaType categorizes media attachments.
type MediaType int

const (
	MediaPhoto MediaType = iota
	MediaVideo
	MediaDocument
	MediaAudio
	MediaOther
)

// MediaInfo describes an attached media item.
type MediaInfo struct {
	Type      MediaType
	FileName  string
	Size      int64
	MimeType  string
	MessageID int
}

// ChatMessage is a single message in a chat.
type ChatMessage struct {
	ID        int
	FromID    int64
	FromName  string
	Date      time.Time
	Text      string
	EditDate  *time.Time
	Out       bool // true if sent by self
	ReplyToID *int
	Media     *MediaInfo
	Unread    bool
}
