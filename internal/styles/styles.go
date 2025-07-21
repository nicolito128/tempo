package styles

import "github.com/charmbracelet/lipgloss"

const (
	PrimaryColor   lipgloss.Color = "#6b84ff"
	SecundaryColor lipgloss.Color = "#6bddff"
	ContrastColor  lipgloss.Color = "#ff6b6b"
	ProblemColor   lipgloss.Color = "#df4e45"
	GreyColor      lipgloss.Color = "#777b7d"
)

var (
	BaseContainerStyle = lipgloss.NewStyle().
				Padding(1, 3).
				Align(lipgloss.Center, lipgloss.Center).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor)

	PrimaryHighlightStyle = lipgloss.NewStyle().
				Background(PrimaryColor).
				Foreground(lipgloss.Color("white"))

	ContrastHighlightStyle = lipgloss.NewStyle().
				Background(ContrastColor).
				Foreground(lipgloss.Color("white"))

	HelpStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(GreyColor)
)

func BaseContainer(xs ...string) string {
	return BaseContainerStyle.Render(xs...)
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
