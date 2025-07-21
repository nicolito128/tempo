package panel

import tea "github.com/charmbracelet/bubbletea"

// Panel : Music list selection panel
type Panel struct{}

var _ tea.Model = (*Panel)(nil)

func New() *Panel {
	p := new(Panel)
	return p
}

func (p *Panel) Init() tea.Cmd {
	return nil
}

func (p *Panel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

func (p *Panel) View() string {
	return ""
}
