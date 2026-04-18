package tg

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	gotdtg "github.com/gotd/td/tg"
)

// StartTelegram returns a tea.Cmd that connects to Telegram in a goroutine.
// On success it sends TelegramReadyMsg; on failure, TelegramErrorMsg.
func StartTelegram(program *tea.Program, tuiAuth *TUIAuth) tea.Cmd {
	return func() tea.Msg {
		bridge := NewBridge(program)
		if program != nil {
			program.Send(StartupStatusMsg{Text: "Checking internet connection…"})
		}

		client, err := telegram.ClientFromEnvironment(telegram.Options{
			UpdateHandler: bridge,
		})
		if err != nil {
			return TelegramErrorMsg{Err: err}
		}

		ctx := context.Background()
		err = client.Run(ctx, func(ctx context.Context) error {
			if program != nil {
				program.Send(StartupStatusMsg{Text: "Checking session…"})
			}
			flow := auth.NewFlow(tuiAuth, auth.SendCodeOptions{})
			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				return err
			}

			self, err := client.Self(ctx)
			if err != nil {
				return err
			}

			api := gotdtg.NewClient(client)
			if program != nil {
				program.Send(StartupStatusMsg{Text: "Connected. Loading chats…"})
				program.Send(TelegramReadyMsg{API: api, Self: self, Client: client})
			}

			// Keep the connection alive until context is cancelled.
			<-ctx.Done()
			return ctx.Err()
		})

		if err != nil && err != context.Canceled {
			return TelegramErrorMsg{Err: err}
		}
		return nil
	}
}
