package tg

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	gotdtg "github.com/gotd/td/tg"
)

// ReadHistory marks messages up to maxID as read for a peer.
func ReadHistory(api *gotdtg.Client, peer gotdtg.InputPeerClass, maxID int) tea.Cmd {
	return func() tea.Msg {
		if api == nil || peer == nil || maxID <= 0 {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = api.MessagesReadHistory(ctx, &gotdtg.MessagesReadHistoryRequest{
			Peer:  peer,
			MaxID: maxID,
		})
		return nil
	}
}
