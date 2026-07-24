# Telegram TUI

Go-based Telegram terminal client using bubbletea v2 + gotd.

## Architecture
- `main.go` — entrypoint

- `internal/app/` — root bubbletea model, key handling, view rendering
- `internal/tg/` — Telegram client, auth, dialogs, history, read markers, updates
- `internal/ui/` — sub-models: channellist, chatview, input, media, search

## Key decisions
- Session stored at `~/.telegram-tui-session.json` (FileSessionStorage)
- Auth input only focused when server requests it (not on startup)
- Password field uses EchoPassword masking
- Unread count: local +1 on new incoming message, decremented immediately on cursor scroll via ConsumeUnread
- Mark-as-read: only on cursor movement (j/k), NOT on chat enter; cursor lands on first unread

## Build
```
go build .
```
