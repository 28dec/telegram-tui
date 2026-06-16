package tg

import (
	"context"
	"os"
	"path/filepath"

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

		// Credentials from https://github.com/paul-nameless/tg
		sessionDir, _ := os.UserHomeDir()
		sessionPath := filepath.Join(sessionDir, ".telegram-tui-session.json")

		client := telegram.NewClient(559815, "fd121358f59d764c57c55871aa0807ca", telegram.Options{
			UpdateHandler:  bridge,
			SessionStorage: &telegram.FileSessionStorage{Path: sessionPath},
		})

		ctx := context.Background()
		err := client.Run(ctx, func(ctx context.Context) error {
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
