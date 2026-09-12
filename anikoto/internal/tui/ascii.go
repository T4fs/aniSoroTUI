package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type logoLetter struct {
	art   []string
	color lipgloss.Color
}

var logoLetters = []logoLetter{
	{
		art: []string{
			" █████╗",
			"██╔══██╗",
			"███████║",
			"██╔══██║",
			"██║  ██║",
			"╚═╝  ╚═╝",
		},
		color: lipgloss.Color("15"),
	},
	{
		art: []string{
			"███╗   ██╗",
			"████╗  ██║",
			"██╔██╗ ██║",
			"██║╚██╗██║",
			"██║ ╚████║",
			"╚═╝  ╚═══╝",
		},
		color: lipgloss.Color("15"),
	},
	{
		art: []string{
			"██╗",
			"██║",
			"██║",
			"██║",
			"██║",
			"╚═╝",
		},
		color: lipgloss.Color("15"),
	},
	{
		art: []string{
			"███████╗",
			"██╔════╝",
			"███████╗",
			"╚════██║",
			"███████║",
			"╚══════╝",
		},
		color: lipgloss.Color("11"),
	},
	{
		art: []string{
			" ██████╗",
			"██╔═══██╗",
			"██║   ██║",
			"██║   ██║",
			"╚██████╔╝",
			" ╚═════╝",
		},
		color: lipgloss.Color("11"),
	},
	{
		art: []string{
			"██████╗",
			"██╔══██╗",
			"██████╔╝",
			"██╔══██╗",
			"██║  ██║",
			"╚═╝  ╚═╝",
		},
		color: lipgloss.Color("11"),
	},
	{
		art: []string{
			" ██████╗",
			"██╔═══██╗",
			"██║   ██║",
			"██║   ██║",
			"╚██████╔╝",
			" ╚═════╝",
		},
		color: lipgloss.Color("11"),
	},
}

const tagline = "Watch anime from your terminal"

func RenderLogo(termWidth int) string {
	if termWidth <= 0 {
		termWidth = 80
	}

	rows := make([]string, 0, len(logoLetters[0].art))
	var maxWidth int
	for i := range logoLetters[0].art {
		var b strings.Builder
		for j, letter := range logoLetters {
			if j > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(letter.color).
				Render(letter.art[i]))
		}
		row := b.String()
		rows = append(rows, row)
		if w := lipgloss.Width(row); w > maxWidth {
			maxWidth = w
		}
	}

	lines := []string{
		"╔" + strings.Repeat("═", maxWidth) + "╗",
		"║" + strings.Repeat(" ", maxWidth) + "║",
	}
	for _, row := range rows {
		pad := maxWidth - lipgloss.Width(row)
		left := pad / 2
		right := pad - left
		lines = append(lines, "║"+strings.Repeat(" ", left)+row+strings.Repeat(" ", right)+"║")
	}
	lines = append(lines,
		"║"+strings.Repeat(" ", maxWidth)+"║",
		"║"+centerIn(tagline, maxWidth)+"║",
		"╚"+strings.Repeat("═", maxWidth)+"╝",
	)

	return lipgloss.PlaceHorizontal(termWidth, lipgloss.Center, strings.Join(lines, "\n"))
}

func centerIn(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func BrandName() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render("Ani"))
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("soro"))
	return b.String()
}
