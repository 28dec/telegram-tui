package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/dxlongnh/telegram-tui/internal/app"
	apptg "github.com/dxlongnh/telegram-tui/internal/tg"
)

func main() {
	tuiAuth := apptg.NewTUIAuth()
	model := app.NewModel(tuiAuth)

	p := tea.NewProgram(model)
	model.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
