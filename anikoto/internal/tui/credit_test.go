package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}

func TestCreditRendersAsButton(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := newTestModel()
	m.width = 120
	m.height = 30
	out := m.View()

	lines := strings.Split(out, "\n")
	var creditLines []string
	for _, line := range lines {
		if strings.Contains(stripANSI(line), "made by t4fs") {
			creditLines = append(creditLines, line)
		}
	}
	if len(creditLines) != 1 {
		t.Fatalf("expected exactly one credit line, got %d\n%q", len(creditLines), out)
	}
	creditLine := creditLines[0]
	if !strings.Contains(creditLine, "93") {
		t.Errorf("credit not rendered in yellow: %q", creditLine)
	}
	hasBorder := strings.Contains(out, "╭") || strings.Contains(out, "╰")
	if !hasBorder {
		t.Errorf("credit button border not rendered\n%q", out)
	}

	m.creditHover = true
	hoverOut := m.View()
	if !strings.Contains(hoverOut, "103") {
		t.Errorf("hovered credit button not filled with yellow background\n%q", hoverOut)
	}
}

func TestCreditHoverToggles(t *testing.T) {
	m := newTestModel()
	got, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = got.(Model)

	if m.credit.h != 3 {
		t.Fatalf("credit button height: expected 3, got %d", m.credit.h)
	}

	in := m.credit.x + m.credit.w/2
	got, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonNone, X: in, Y: m.credit.y})
	m = got.(Model)
	if !m.creditHover {
		t.Fatal("expected creditHover=true when mouse is over the button")
	}

	got, _ = m.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonNone, X: m.credit.x + m.credit.w + 20, Y: m.credit.y})
	m = got.(Model)
	if m.creditHover {
		t.Fatal("expected creditHover=false when mouse leaves the button")
	}
}

func TestCreditClickOpensGitHub(t *testing.T) {
	var opened string
	orig := openURL
	openURL = func(url string) error {
		opened = url
		return nil
	}
	defer func() { openURL = orig }()

	m := newTestModel()
	got, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = got.(Model)

	if m.credit.w != lipglossWidth(creditText)+creditButtonPadding+creditButtonBorders {
		t.Fatalf("credit width mismatch: %d", m.credit.w)
	}

	in := m.credit.x + m.credit.w/2
	got, cmds := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: in, Y: m.credit.y})
	_ = got
	if cmds == nil {
		t.Fatal("expected a command for a click on the credit")
	}
	cmds()
	if opened != creditURL {
		t.Fatalf("expected to open %s, opened %q", creditURL, opened)
	}
}

func TestCreditClickOutsideBoundsIgnored(t *testing.T) {
	called := false
	orig := openURL
	openURL = func(url string) error {
		called = true
		return nil
	}
	defer func() { openURL = orig }()

	m := newTestModel()
	got, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = got.(Model)

	_, cmds := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: m.credit.x + m.credit.w + 20, Y: m.credit.y})
	if cmds != nil { cmds() }
	if called {
		t.Fatal("click outside credit bounds must not open anything")
	}

	_, cmds = m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: m.credit.x, Y: m.credit.y + 5})
	if cmds != nil { cmds() }
	if called {
		t.Fatal("click outside credit row must not open anything")
	}
}

func stripANSI(s string) string {
	re := regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	return re.ReplaceAllString(s, "")
}
