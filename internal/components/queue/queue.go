package queue

import tea "github.com/charmbracelet/bubbletea"

type Queue struct{}

var _ tea.Model = (*Queue)(nil)

func New() *Queue {
	q := new(Queue)
	return q
}

func (q *Queue) Init() tea.Cmd {
	return nil
}

func (q *Queue) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return q, nil
}

func (q *Queue) View() string {
	return ""
}
