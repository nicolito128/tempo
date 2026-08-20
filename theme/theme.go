package theme

import (
	"charm.land/lipgloss/v2"
)

var (
	Primary   = lipgloss.Color("#6b84ff")
	Secundary = lipgloss.Color("#6bddff")
	Contrast  = lipgloss.Color("#ff6b6b")
	Problem   = lipgloss.Color("#df4e45")
	Grey      = lipgloss.Color("#777b7d")
)

var (
	BaseContainerStyle = lipgloss.NewStyle().
				Padding(1, 3).
				Align(lipgloss.Center, lipgloss.Center).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Primary)

	PrimaryHighlightStyle = lipgloss.NewStyle().
				Background(Primary).
				Foreground(lipgloss.Color("white"))

	ContrastHighlightStyle = lipgloss.NewStyle().
				Background(Contrast).
				Foreground(lipgloss.Color("white"))

	HelpStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(Grey)
)

func BaseContainer(width, height int, xs ...string) string {
	return BaseContainerStyle.Width(width).Height(height).Render(xs...)
}

func PrimaryHighlight(xs ...string) string {
	return PrimaryHighlightStyle.Render(xs...)
}

func ContrastHighlight(xs ...string) string {
	return ContrastHighlightStyle.Render(xs...)
}

func Help(xs ...string) string {
	return HelpStyle.Render(xs...)
}
