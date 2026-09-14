package tui

import "github.com/charmbracelet/lipgloss"

var (
	AccentColor    = lipgloss.AdaptiveColor{Light: "11", Dark: "11"}
	DimColor       = lipgloss.AdaptiveColor{Light: "245", Dark: "240"}
	ErrorColor     = lipgloss.AdaptiveColor{Light: "11", Dark: "11"}
	HighlightColor = lipgloss.AdaptiveColor{Light: "11", Dark: "11"}
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor)

	DimStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(DimColor)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(HighlightColor).
			Bold(true)

	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	SelectedListItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Background(lipgloss.AdaptiveColor{Light: "7", Dark: "236"}).
				Bold(true).
				Foreground(HighlightColor)

	SearchInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(AccentColor).
				Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(DimColor).
			PaddingTop(1)

	LoadingStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(DimColor)

	ErrorBoxStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true).
			Padding(1)

	EpisodeStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	ControlStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor)
)
