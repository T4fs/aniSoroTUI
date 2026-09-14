package tui

import (
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const creditURL = "https://github.com/T4fs/aniSoroTUI"

var creditText = "made by t4fs"

var CreditButtonStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder(), true).
	BorderForeground(lipgloss.Color("11")).
	Foreground(lipgloss.Color("11")).
	Padding(0, 1)

var CreditButtonHoverStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder(), true).
	BorderForeground(lipgloss.Color("11")).
	Background(lipgloss.Color("11")).
	Foreground(lipgloss.Color("0")).
	Padding(0, 1)

func renderCreditButton(hover bool) []string {
	style := CreditButtonStyle
	if hover {
		style = CreditButtonHoverStyle
	}
	return strings.Split(style.Render(creditText), "\n")
}

var openURL = openURLImpl

func openURLImpl(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := openURL(url); err != nil {
			debugLog.Printf("[TUI] open url %s: %v", url, err)
		}
		return nil
	}
}

type creditBounds struct {
	x, y, w, h int
}

// creditContentRow is the 0-based row of the credit button's text line
// within the home content block; the button spans creditContentRow-1..+1.
const creditContentRow = 18

const creditButtonPadding = 2
const creditButtonBorders = 2

func (m Model) creditPosition() creditBounds {
	layoutW, topPad := homeLayout(m.width, m.height)
	w := lipgloss.Width(creditText) + creditButtonPadding + creditButtonBorders
	x := (m.width-layoutW)/2 + (layoutW-w)/2
	y := topPad + creditContentRow - 1
	return creditBounds{x: x, y: y, w: w, h: 3}
}