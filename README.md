# telegram-tui

A fast, keyboard-first Telegram client for the terminal, written in Go.

`telegram-tui` is designed around a Vim-like workflow and a stable asynchronous UI: browse chats, jump into conversations, search with `/`, read media, and send messages without leaving your terminal.

---

## Features

- **Single-screen Telegram workflow**
  - Open app → chat list
  - `Enter` to open a chat
  - `Esc` to go back
- **Vim-like navigation**
  - `j`/`k` move
  - `J`/`K` jump 10 lines
- **Fast global search**
  - `/` opens fuzzy search popup
  - Search across **all chats** (main + archived, including users)
- **Messaging modes**
  - `i` inline message
  - `I` multiline message
  - `r` quick reply
  - `R` multiline reply
- **Unread-aware behavior**
  - Opening a chat jumps to the **oldest unread message**
  - Moving cursor over messages marks them read and updates UI live
- **Readable message layout**
  - Local datetime timestamps (`yyyy-MM-dd hh:mm`)
  - Sender coloring by hashed user ID
  - Incoming left-aligned / outgoing right-aligned
  - Multiline-safe rendering
- **Media support**
  - `Space` on media message to open media viewer/downloader
- **Terminal-native UX**
  - Static centered header/footer
  - Non-blocking background updates
  - `Ctrl+C` to quit

---

## Tech stack

- [Go](https://go.dev/)
- [gotd/td](https://github.com/gotd/td) for Telegram MTProto client
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) + [Bubbles](https://github.com/charmbracelet/bubbles) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) for TUI
- [sahilm/fuzzy](https://github.com/sahilm/fuzzy) for chat search

---

## Prerequisites

- Go **1.25+**
- Telegram API credentials (`APP_ID`, `APP_HASH`)

Get credentials from: https://my.telegram.org → API development tools.

---

## Quick start

### 1) Clone

```bash
git clone <your-repo-url>
cd telegram-tui
```

### 2) Configure environment

```bash
cp .env.example .env
```

Edit `.env`:

```env
APP_ID=your_app_id_here
APP_HASH=your_app_hash_here
# SESSION_FILE=/path/to/custom/session.json
```

### 3) Run

```bash
go run ./cmd/telegram-tui
```

Or build binary:

```bash
go build -o telegram-tui ./cmd/telegram-tui
./telegram-tui
```

On first run, log in with phone/code (and 2FA password if enabled).

---

## Keybindings

### Global

- `Ctrl+C` — quit

### Chat list

- `j` / `k` — move cursor
- `J` / `K` — jump 10 items
- `a` — toggle main/archived view
- `Enter` — open selected chat
- `/` — fuzzy search popup

### Chat view

- `j` / `k` — move through messages
- `J` / `K` — jump through messages
- `i` — inline message
- `I` — multiline message
- `r` — reply
- `R` — multiline reply
- `Esc` — back to chat list
- `/` — fuzzy search popup

---

## Project structure

```text
cmd/telegram-tui/main.go   # app entrypoint
internal/app/              # root Bubble Tea app model + routing
internal/tg/               # Telegram API/auth/history/dialogs/media
internal/ui/               # chat list, chat view, input, search, media components
```

---

## Notes

- Keep your real `.env` private. Commit only `.env.example`.
- Do not commit built binaries (`telegram-tui`) to Git.
- Session state is managed by gotd (default in user home unless overridden).

---

## Current status

Active development. Interface and keymaps may evolve.

If you want, open an issue with your preferred terminal workflow and we can tune it.
