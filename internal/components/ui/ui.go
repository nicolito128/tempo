package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nicolito128/tempo/internal/components/player"
)

// UI : Tempo user interface model
//
// It holds the player, queue and current state of the UI.
type UI struct {
	width  int
	height int

	player *player.Player
}

var _ tea.Model = (*UI)(nil)

func New() *UI {
	ui := new(UI)
	// TODO: read init volume value from user configuration
	ui.player = player.New(50)
	return ui
}

func (ui *UI) Init() tea.Cmd {
	ui.player.Init()
	return nil
}

func (ui *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ui.Error() != nil {
		return ui, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ui.width = msg.Width
		ui.height = msg.Height
		return ui, tea.ClearScreen
	}

	_, cmd := ui.player.Update(msg)
	if cmd != nil {
		return ui, cmd
	}

	return ui, nil
}

func (ui *UI) View() string {
	if ui.Error() != nil {
		return fmt.Sprintf("Error: %s", ui.Error())
	}

	var xs string
	xs += ui.player.View()

	return xs
}

func (ui *UI) Error() error {
	if ui.player.Error() != nil {
		return fmt.Errorf("audio player fail: %w", ui.player.Error())
	}
	return nil
}

func (ui *UI) Player() *player.Player {
	return ui.player
}
