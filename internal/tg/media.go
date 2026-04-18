package tg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	gotdtg "github.com/gotd/td/tg"
)

// MediaReadyMsg is sent when a media download completes.
type MediaReadyMsg struct {
	Path string
	Info *MediaInfo
}

// MediaErrMsg is sent when a media download fails.
type MediaErrMsg struct {
	Err error
}

// DownloadMedia returns a tea.Cmd that downloads the media attached to the
// given message ID and saves it to a temp file.
func DownloadMedia(client *telegram.Client, info *MediaInfo, peer gotdtg.InputPeerClass) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		api := gotdtg.NewClient(client)
		dl := downloader.NewDownloader()

		// Fetch the message to get the media location.
		result, err := api.MessagesGetMessages(ctx, []gotdtg.InputMessageClass{
			&gotdtg.InputMessageID{ID: info.MessageID},
		})
		if err != nil {
			return MediaErrMsg{Err: fmt.Errorf("fetch message: %w", err)}
		}

		var location gotdtg.InputFileLocationClass
		var fileName string

		switch r := result.(type) {
		case *gotdtg.MessagesMessages:
			location, fileName = extractLocation(r.Messages, info)
		case *gotdtg.MessagesMessagesSlice:
			location, fileName = extractLocation(r.Messages, info)
		case *gotdtg.MessagesChannelMessages:
			location, fileName = extractLocation(r.Messages, info)
		}

		if location == nil {
			return MediaErrMsg{Err: fmt.Errorf("no downloadable media found")}
		}

		tmpDir := os.TempDir()
		dst := filepath.Join(tmpDir, "tgtui_"+fileName)

		f, err := os.Create(dst)
		if err != nil {
			return MediaErrMsg{Err: fmt.Errorf("create temp file: %w", err)}
		}
		defer f.Close()

		if _, err := dl.Download(client.API(), location).Stream(ctx, f); err != nil {
			return MediaErrMsg{Err: fmt.Errorf("download: %w", err)}
		}

		return MediaReadyMsg{Path: dst, Info: info}
	}
}

func extractLocation(msgs []gotdtg.MessageClass, info *MediaInfo) (gotdtg.InputFileLocationClass, string) {
	for _, raw := range msgs {
		m, ok := raw.(*gotdtg.Message)
		if !ok || m.GetID() != info.MessageID {
			continue
		}
		media, ok := m.GetMedia()
		if !ok {
			continue
		}
		switch md := media.(type) {
		case *gotdtg.MessageMediaPhoto:
			photo, ok := md.GetPhoto()
			if !ok {
				continue
			}
			p, ok := photo.(*gotdtg.Photo)
			if !ok {
				continue
			}
			// Use the largest size.
			var best *gotdtg.PhotoSize
			for _, sz := range p.GetSizes() {
				if s, ok := sz.(*gotdtg.PhotoSize); ok {
					if best == nil || s.GetW() > best.GetW() {
						best = s
					}
				}
			}
			if best != nil {
				return &gotdtg.InputPhotoFileLocation{
					ID:            p.GetID(),
					AccessHash:    p.GetAccessHash(),
					FileReference: p.GetFileReference(),
					ThumbSize:     best.GetType(),
				}, "photo.jpg"
			}
		case *gotdtg.MessageMediaDocument:
			doc, ok := md.GetDocument()
			if !ok {
				continue
			}
			d, ok := doc.(*gotdtg.Document)
			if !ok {
				continue
			}
			name := info.FileName
			if name == "" {
				name = fmt.Sprintf("document_%d", info.MessageID)
			}
			return &gotdtg.InputDocumentFileLocation{
				ID:            d.GetID(),
				AccessHash:    d.GetAccessHash(),
				FileReference: d.GetFileReference(),
			}, name
		}
	}
	return nil, ""
}
