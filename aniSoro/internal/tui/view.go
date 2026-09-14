package tui

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func modKey() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "ctrl"
}

func (m Model) View() string {
	var content string

	switch m.screen {
	case ScreenHome:
		content = m.viewHome()
	case ScreenSearching:
		content = m.viewLoading(m.loadingText)
	case ScreenResults:
		content = m.viewResults()
	case ScreenEpisodes:
		content = m.viewEpisodes()
	case ScreenLoadingEpisode:
		content = m.viewLoading(m.loadingText)
	case ScreenWatching:
		content = m.viewWatching()
	case ScreenSelectDubSub:
		content = m.viewSelectDubSub()
	default:
		content = m.viewHome()
	}

	if m.showHelp {
		popup := m.renderHelpPopup()
		content = overlayHelp(content, popup, m.width)
	}

	return content
}

const (
	homeContentLines = 22
	homeMaxPad       = 6
)

func homeLayout(width, height int) (layoutW, topPad int) {
	layoutW = min(width, 100)
	if layoutW < 10 {
		layoutW = 10
	}
	if height == 0 {
		return layoutW, 0
	}
	available := height - 3
	if available < homeContentLines {
		available = homeContentLines
	}
	topPad = (available - homeContentLines) / 2
	if topPad > homeMaxPad {
		topPad = homeMaxPad
	}
	return layoutW, topPad
}

func (m Model) viewHome() string {
	layoutW, topPad := homeLayout(m.width, m.height)

	var sb strings.Builder

	logo := RenderLogo(layoutW)
	sb.WriteString(logo)
	sb.WriteString("\n\n")

	searchBox := SearchInputStyle.Width(min(60, layoutW-10)).Render(m.input.View())
	searchBox = lipgloss.PlaceHorizontal(layoutW, lipgloss.Center, searchBox)
	sb.WriteString(searchBox)
	sb.WriteString("\n\n")

	creditButton := renderCreditButton(m.creditHover)
	for _, line := range creditButton {
		sb.WriteString(lipgloss.PlaceHorizontal(layoutW, lipgloss.Center, line))
		sb.WriteString("\n")
	}

	contentStr := sb.String()

	versionStr := "v" + Version
	helpText := "[?] for help"
	helpWidth := len([]rune(helpText))
	versionWidth := len([]rune(versionStr))
	gaps := layoutW - helpWidth - versionWidth - 1
	if gaps < 1 {
		gaps = 1
	}
	helpLine := HelpStyle.Render(helpText + strings.Repeat(" ", gaps) + versionStr)

	separatorLine := DimStyle.Render(strings.Repeat("─", layoutW))

	if m.height == 0 {
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, contentStr+"\n\n"+separatorLine+"\n"+helpLine)
	}

	// Footer is 3 lines: separator + blank(PaddingTop) + help text
	footerHeight := 3
	contentHeight := strings.Count(contentStr, "\n") + 1
	available := m.height - footerHeight
	if available < contentHeight {
		available = contentHeight
	}
	bottomPad := available - contentHeight - topPad
	if bottomPad > homeMaxPad {
		bottomPad = homeMaxPad
	}

	var result strings.Builder
	result.WriteString(strings.Repeat("\n", topPad))
	result.WriteString(contentStr)
	result.WriteString(strings.Repeat("\n", bottomPad))
	result.WriteString(separatorLine)
	result.WriteString("\n")
	result.WriteString(helpLine)
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, result.String())
}

func (m Model) viewLoading(text string) string {
	var sb strings.Builder

	header := lipgloss.NewStyle().
		Bold(true).
		Render(BrandName())
	sb.WriteString(header)
	sb.WriteString("\n\n")

	spinnerChar := spinnerChars[m.spinIndex%len(spinnerChars)]
	loading := fmt.Sprintf("%s %s", spinnerChar, text)
	sb.WriteString(LoadingStyle.Render(loading))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

func (m Model) viewResults() string {
	if m.errorMsg != "" {
		return m.viewError(m.errorMsg)
	}

	var sb strings.Builder

	header := TitleStyle.
		PaddingBottom(1).
		Render(fmt.Sprintf("Results for '%s' (%d found)", m.query, len(m.results)))
	sb.WriteString(header)
	sb.WriteString("\n\n")

	// Empty results — show inline message instead of redirecting to error view
	if len(m.results) == 0 {
		noResultsStyle := lipgloss.NewStyle().
			Foreground(DimColor).
			Faint(true)
		noResultsText := fmt.Sprintf("No results for '%s' — try a different search.", m.query)
		sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, noResultsStyle.Render(noResultsText)))

		contentStr := sb.String()
		versionStr := "v" + Version
		help := "[?] for help"
		helpWidth := len([]rune(help))
		versionWidth := len([]rune(versionStr))
		gaps := m.width - helpWidth - versionWidth - 1
		if gaps < 1 {
			gaps = 1
		}
		helpLine := HelpStyle.Render(help + strings.Repeat(" ", gaps) + versionStr)
		separatorLine := DimStyle.Render(strings.Repeat("─", m.width))

		if m.height == 0 {
			return contentStr + separatorLine + "\n" + helpLine
		}

		footerHeight := 3
		contentHeight := strings.Count(contentStr, "\n") + 1

		var result strings.Builder
		result.WriteString(contentStr)
		if contentHeight+footerHeight < m.height {
			result.WriteString(strings.Repeat("\n", m.height-contentHeight-footerHeight))
		}
		result.WriteString(separatorLine)
		result.WriteString("\n")
		result.WriteString(helpLine)
		return result.String()
	}

	maxLines := max(1, m.height-8)

	// Scroll backward from cursor to find the starting index.
	// Unselected items take 2 lines (title + metadata); the selected item
	// takes up to 3 lines (title + metadata + synopsis).
	startIdx := m.cursor
	linesBefore := 0
	for startIdx > 0 {
		if linesBefore+2 >= maxLines {
			break
		}
		linesBefore += 2
		startIdx--
	}

	metaStyle := lipgloss.NewStyle().
		Foreground(DimColor).
		PaddingLeft(2)
	synopsisStyle := lipgloss.NewStyle().
		Foreground(DimColor).
		Faint(true).
		PaddingLeft(2)
	selBgColor := lipgloss.AdaptiveColor{Light: "7", Dark: "236"}

	linesUsed := 0
	for i := startIdx; i < len(m.results); i++ {
		anime := m.results[i]

		// ---- Title line ----
		if i == m.cursor {
			sb.WriteString(SelectedListItemStyle.Width(m.width - 2).Render("▸ " + anime.Title))
			sb.WriteString("\n")
		} else {
			sb.WriteString(ListItemStyle.Render(TitleStyle.Render(anime.Title)))
			sb.WriteString("\n")
		}
		linesUsed++

		// ---- Metadata line (wraps if genres/type/year overflow) ----
		var metaParts []string
		animeType := anime.Type
		if animeType == "" {
			animeType = "?"
		}
		metaParts = append(metaParts, fmt.Sprintf("[%s]", animeType))
		if anime.Year > 0 {
			metaParts = append(metaParts, fmt.Sprintf("%d", anime.Year))
		}
		if anime.EpisodeCount > 0 {
			metaParts = append(metaParts, fmt.Sprintf("%d eps", anime.EpisodeCount))
		}
		if len(metaParts) > 0 {
			metaLine := strings.Join(metaParts, "  |  ")
			var rendered string
			if i == m.cursor {
				rendered = metaStyle.Background(selBgColor).Width(m.width - 2).Render(metaLine)
			} else {
				rendered = metaStyle.Width(m.width - 2).Render(metaLine)
			}
			sb.WriteString(rendered)
			sb.WriteString("\n")
			linesUsed += strings.Count(rendered, "\n") + 1
		} else {
			sb.WriteString("\n")
			linesUsed++
		}

		// ---- Synopsis line (selected item only, wraps to fit) ----
		if i == m.cursor {
			synopsis := anime.Synopsis
			if synopsis == "" {
				synopsis = anime.Description
			}
			if synopsis != "" {
				rendered := synopsisStyle.Background(selBgColor).Width(m.width - 2).Render(truncate(synopsis, m.width-6))
				sb.WriteString(rendered)
				sb.WriteString("\n")
				linesUsed += strings.Count(rendered, "\n") + 1
			}
		}

		if linesUsed >= maxLines {
			break
		}
	}

	contentStr := sb.String()

	versionStr := "v" + Version
	help := "[?] for help"
	helpWidth := len([]rune(help))
	versionWidth := len([]rune(versionStr))
	gaps := m.width - helpWidth - versionWidth - 1
	if gaps < 1 {
		gaps = 1
	}
	helpLine := HelpStyle.Render(help + strings.Repeat(" ", gaps) + versionStr)

	separatorLine := DimStyle.Render(strings.Repeat("─", m.width))

	if m.height == 0 {
		return contentStr + separatorLine + "\n" + helpLine
	}

	footerHeight := 3 // separator + blank(PaddingTop) + help
	contentHeight := strings.Count(contentStr, "\n") + 1

	var result strings.Builder
	result.WriteString(contentStr)
	if contentHeight+footerHeight < m.height {
		result.WriteString(strings.Repeat("\n", m.height-contentHeight-footerHeight))
	}
	result.WriteString(separatorLine)
	result.WriteString("\n")
	result.WriteString(helpLine)
	return result.String()
}

func (m Model) viewEpisodes() string {
	if m.errorMsg != "" {
		return m.viewError(m.errorMsg)
	}

	var sb strings.Builder

	title := ""
	anime := m.selectedAnime
	if anime != nil {
		title = anime.Title
	}

	// ---- Title line ----
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor).
		Render(fmt.Sprintf("%s — Episodes (%d)", title, len(m.episodes)))
	sb.WriteString(header)

	// ---- Metadata line ----
	if anime != nil {
		var metaParts []string

		// [Type]
		if anime.Type != "" {
			metaParts = append(metaParts, fmt.Sprintf("[%s]", anime.Type))
		}

		// Year
		if anime.Year > 0 {
			metaParts = append(metaParts, fmt.Sprintf("%d", anime.Year))
		}

		// ✪ Score/10
		if anime.Score > 0 {
			metaParts = append(metaParts, fmt.Sprintf("✪ %.1f/10", anime.Score))
		}

		// Studio
		if anime.Studio != "" {
			metaParts = append(metaParts, anime.Studio)
		}

		// Genres
		if len(anime.Genres) > 0 {
			metaParts = append(metaParts, strings.Join(anime.Genres, ", "))
		}

		// Status (formatted nicely)
		status := formatStatus(anime.Status)
		if status != "" {
			metaParts = append(metaParts, status)
		}

		if len(metaParts) > 0 {
			metaStyle := lipgloss.NewStyle().
				Foreground(DimColor).
				PaddingLeft(2).
				Width(m.width - 4)
			metaLine := metaStyle.Render(strings.Join(metaParts, "  |  "))
			sb.WriteString("\n")
			sb.WriteString(metaLine)
		}
	}

	// ---- Synopsis section (collapsible with space) ----
	if anime != nil && anime.Synopsis != "" {
		sb.WriteString("\n\n")

		synopsis := anime.Synopsis
		synopsisStyle := lipgloss.NewStyle().
			Foreground(DimColor).
			Faint(true).
			PaddingLeft(2)

		wrapped := lipgloss.NewStyle().Width(m.width - 4).Render(synopsis)
		lines := strings.Split(wrapped, "\n")
		synopsisLines := len(lines)

		if m.showFullSynopsis {
			sb.WriteString(synopsisStyle.Render(wrapped))
			sb.WriteString("\n")
			sb.WriteString(DimStyle.Render("[space to collapse]"))
		} else {
			showLines := min(4, synopsisLines)
			sb.WriteString(synopsisStyle.Render(strings.Join(lines[:showLines], "\n")))
			if synopsisLines > 4 {
				sb.WriteString("\n")
				sb.WriteString(DimStyle.Render("[space to expand]"))
			}
		}
	}

	sb.WriteString("\n\n")

	// ---- Episode list ----
	preContent := sb.String()
	preLines := strings.Count(preContent, "\n") + 1
	preliminaryHeight := 3 // separator + blank(PaddingTop) + help
	availableHeight := max(1, m.height-preLines-preliminaryHeight)

	startIdx := 0
	if m.episodeCursor >= availableHeight {
		startIdx = max(0, m.episodeCursor-availableHeight+1)
	}
	endIdx := min(len(m.episodes), startIdx+availableHeight)

	for i := startIdx; i < endIdx; i++ {
		ep := m.episodes[i]
		prefix := "  "
		if i == m.episodeCursor {
			prefix = "▸ "
		}

		epNum := strings.TrimPrefix(ep.Number, "EP ")
		epNum = strings.TrimPrefix(epNum, "Episode ")
		line := fmt.Sprintf("%sEP %s - %s", prefix, epNum, ep.Title)
		if i == m.episodeCursor {
			line = SelectedStyle.Render(line)
		} else {
			line = EpisodeStyle.Render(line)
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	contentStr := sb.String()

	// ---- Help bar ----
	versionStr := "v" + Version
	help := "[?] for help"
	helpWidth := len([]rune(help))
	versionWidth := len([]rune(versionStr))
	gaps := m.width - helpWidth - versionWidth - 1
	if gaps < 1 {
		gaps = 1
	}
	helpLine := HelpStyle.Render(help + strings.Repeat(" ", gaps) + versionStr)

	separatorLine := DimStyle.Render(strings.Repeat("─", m.width))

	if m.height == 0 {
		return contentStr + separatorLine + "\n" + helpLine
	}

	footerHeight := 3
	contentHeight := strings.Count(contentStr, "\n") + 1

	var result strings.Builder
	result.WriteString(contentStr)
	if contentHeight+footerHeight < m.height {
		result.WriteString(strings.Repeat("\n", m.height-contentHeight-footerHeight))
	}
	result.WriteString(separatorLine)
	result.WriteString("\n")
	result.WriteString(helpLine)
	return result.String()
}

func (m Model) viewWatching() string {
	if m.watching == nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, LoadingStyle.Render("Loading..."))
	}

	var sb strings.Builder

	// Anime title
	title := m.watching.animeTitle
	if title == "" && m.selectedAnime != nil {
		title = m.selectedAnime.Title
	}
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, TitleStyle.Render(title)))
	sb.WriteString("\n")

	// Episode line
	var epNumber, epTitle string
	if m.episodeCursor >= 0 && m.episodeCursor < len(m.episodes) {
		epNumber = m.episodes[m.episodeCursor].Number
		epTitle = m.episodes[m.episodeCursor].Title
	}
	sb.WriteString("\n")
	epNum := strings.TrimPrefix(epNumber, "EP ")
	epNum = strings.TrimPrefix(epNum, "Episode ")
	epLine := fmt.Sprintf("EP %s - %s", epNum, epTitle)
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, epLine))
	sb.WriteString("\n")

	// Status
	sb.WriteString("\n")
	statusLine := LoadingStyle.Render("Now playing in external player...")
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, statusLine))
	sb.WriteString("\n")

	// Error message (if any) — shown above controls
	if m.errorMsg != "" {
		sb.WriteString("\n")
		errBox := ErrorBoxStyle.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ErrorColor).
			Padding(1, 2).
			Width(min(80, m.width-10)).
			Render(m.errorMsg)
		sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, errBox))
		sb.WriteString("\n")
	}

	// Controls
	sb.WriteString("\n")
	var btnControls []string
	if m.watching.episodeIndex > 0 {
		btnControls = append(btnControls, ControlStyle.Render("← prev"))
	}
	btnControls = append(btnControls, ControlStyle.Render("space [replay]"))
	if m.watching.episodeIndex < m.watching.episodesLen-1 {
		btnControls = append(btnControls, ControlStyle.Render("next →"))
	}
	controlsStr := strings.Join(btnControls, "  |  ")
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, controlsStr))
	sb.WriteString("\n")

	// Source line
	quality := ""
	sourceCount := 0
	if m.watching != nil {
		sourceCount = len(m.watching.sources)
		if m.watching.sourceIndex >= 0 && m.watching.sourceIndex < len(m.watching.sources) {
			quality = m.watching.sources[m.watching.sourceIndex].Quality
		}
	}
	mode := "SUB"
	if m.watching.dub {
		mode = "DUB"
	}
	sourcePart := ""
	if sourceCount > 1 {
		sourcePart = fmt.Sprintf("Source %d/%d", m.watching.sourceIndex+1, sourceCount)
		if quality != "" && quality != "default" {
			sourcePart += fmt.Sprintf(" — %s", quality)
		}
	} else if quality != "" && quality != "default" {
		sourcePart = fmt.Sprintf("Source: %s", quality)
	} else {
		sourcePart = "Source:"
	}
	sourceLine := DimStyle.Render(fmt.Sprintf("%s  |  %s", sourcePart, mode))
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, sourceLine))
	sb.WriteString("\n")

	contentStr := sb.String()

	versionStr := "v" + Version
	helpText := "[?] for help"
	helpWidth := len([]rune(helpText))
	versionWidth := len([]rune(versionStr))
	gaps := m.width - helpWidth - versionWidth - 1
	if gaps < 1 {
		gaps = 1
	}
	helpLine := HelpStyle.Render(helpText + strings.Repeat(" ", gaps) + versionStr)

	separatorLine := DimStyle.Render(strings.Repeat("─", m.width))

	if m.height == 0 {
		return contentStr + separatorLine + "\n" + helpLine
	}

	footerHeight := 3
	contentHeight := strings.Count(contentStr, "\n") + 1
	availableHeight := m.height - footerHeight
	if availableHeight < contentHeight {
		availableHeight = contentHeight
	}
	topPad := (availableHeight - contentHeight) / 2
	bottomPad := availableHeight - contentHeight - topPad

	var result strings.Builder
	result.WriteString(strings.Repeat("\n", topPad))
	result.WriteString(contentStr)
	result.WriteString(strings.Repeat("\n", bottomPad))
	result.WriteString(separatorLine)
	result.WriteString("\n")
	result.WriteString(helpLine)
	return result.String()
}

func (m Model) viewSelectDubSub() string {
	var sb strings.Builder

	// Title
	title := ""
	if m.selectedAnime != nil {
		title = m.selectedAnime.Title
	}
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, TitleStyle.Render(title)))
	sb.WriteString("\n\n")

	if m.pendingEpisode != nil {
		epNum := strings.TrimPrefix(m.pendingEpisode.Number, "EP ")
		epNum = strings.TrimPrefix(epNum, "Episode ")
		epLine := fmt.Sprintf("EP %s - %s", epNum, m.pendingEpisode.Title)
		sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, epLine))
		sb.WriteString("\n\n")
	}

	// Sub/Dub selection box
	subStyle := SelectedListItemStyle.Width(40)
	dubStyle := ListItemStyle.Width(40)

	if !m.dub { // sub selected
		subStyle = SelectedListItemStyle.Width(40)
		dubStyle = ListItemStyle.Width(40)
	} else { // dub selected
		subStyle = ListItemStyle.Width(40)
		dubStyle = SelectedListItemStyle.Width(40)
	}

	subLabel := "SUB"
	dubLabel := "DUB"
	if !m.pendingDubAvail {
		dubLabel = "DUB (not available)"
	}

	subBtn := subStyle.Render("▸ " + subLabel)
	dubBtn := dubStyle.Render("  " + dubLabel)

	btnLine := lipgloss.JoinHorizontal(lipgloss.Center, subBtn, "  ", dubBtn)
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, btnLine))
	sb.WriteString("\n\n")

	// Hint
	hint := DimStyle.Render("←/→ or s/d to select  |  enter to play  |  esc to cancel")
	sb.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, hint))

	// Center vertically
	contentStr := sb.String()
	contentHeight := strings.Count(contentStr, "\n") + 1
	topPad := (m.height - contentHeight) / 2
	if topPad < 0 {
		topPad = 0
	}
	var result strings.Builder
	result.WriteString(strings.Repeat("\n", topPad))
	result.WriteString(contentStr)
	return result.String()
}

func (m Model) viewError(errMsg string) string {
	var sb strings.Builder

	header := lipgloss.NewStyle().
		Bold(true).
		Render(BrandName())
	sb.WriteString(header)
	sb.WriteString("\n\n")

	errorBox := ErrorBoxStyle.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ErrorColor).
		Padding(1, 2).
		Width(min(80, m.width-10)).
		Render(errMsg)

	errorBox = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, errorBox)
	sb.WriteString(errorBox)
	sb.WriteString("\n")

	contentStr := sb.String()

	versionStr := "v" + Version
	help := "[?] for help"
	helpWidth := len([]rune(help))
	versionWidth := len([]rune(versionStr))
	gaps := m.width - helpWidth - versionWidth - 1
	if gaps < 1 {
		gaps = 1
	}
	helpLine := HelpStyle.Render(help + strings.Repeat(" ", gaps) + versionStr)

	separatorLine := DimStyle.Render(strings.Repeat("─", m.width))

	if m.height == 0 {
		return contentStr + separatorLine + "\n" + helpLine
	}

	footerHeight := 3 // separator + blank(PaddingTop) + help
	contentHeight := strings.Count(contentStr, "\n") + 1

	var result strings.Builder
	result.WriteString(contentStr)
	if contentHeight+footerHeight < m.height {
		result.WriteString(strings.Repeat("\n", m.height-contentHeight-footerHeight))
	}
	result.WriteString(separatorLine)
	result.WriteString("\n")
	result.WriteString(helpLine)
	return result.String()
}

func truncate(s string, maxLen int) string {
	// Collapse all whitespace (newlines, multiple spaces) to single spaces
	s = strings.Join(strings.Fields(s), " ")

	s = strings.TrimSpace(s)

	// Use rune count for proper Unicode handling
	runes := []rune(s)
	if len(runes) <= maxLen {
		return string(runes)
	}

	return string(runes[:maxLen-3]) + "..."
}

func formatStatus(status string) string {
	switch status {
	case "FINISHED":
		return "Finished Airing"
	case "RELEASING":
		return "Currently Airing"
	case "NOT_YET_RELEASED":
		return "Not Yet Aired"
	case "CANCELLED":
		return "Cancelled"
	case "HIATUS":
		return "On Hiatus"
	default:
		return status
	}
}

// overlayHelp overlays the help popup at the bottom of the rendered content,
// like opencode's Ctrl+P overlay. The content above stays fully visible.
func overlayHelp(content, popup string, termWidth int) string {
	lines := strings.Split(content, "\n")
	popupLines := strings.Split(popup, "\n")
	popupHeight := len(popupLines)

	popupWidth := 0
	for _, pl := range popupLines {
		w := lipgloss.Width(pl)
		if w > popupWidth {
			popupWidth = w
		}
	}

	// Position at the bottom, leaving a 1-line margin
	startY := len(lines) - popupHeight - 1
	if startY < 0 {
		startY = 0
	}
	// Clip out the lines the popup will occupy
	lines = lines[:startY]

	leftPad := (termWidth - popupWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// Add margin line
	lines = append(lines, "")

	// Add each popup line centered horizontally, padded to full width
	// so the original footer content underneath is completely overwritten.
	for _, pl := range popupLines {
		line := strings.Repeat(" ", leftPad) + pl
		visible := lipgloss.Width(line)
		if rightPad := termWidth - visible; rightPad > 0 {
			line += strings.Repeat(" ", rightPad)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderHelpPopup() string {
	var bindings []string

	switch m.screen {
	case ScreenHome:
		bindings = []string{
			"  Home",
			"    enter  [search]",
			"    /      [focus search]",
			"",
			"  General",
			"    ?      [toggle help]",
			"    " + modKey() + "+c  [quit]",
		}
	case ScreenResults:
		bindings = []string{
			"  Results",
			"    ↑/k    [move up]",
			"    ↓/j    [move down]",
			"    gg     [top]",
			"    G      [bottom]",
			"    ctrl+u [page up]",
			"    ctrl+d [page down]",
			"",
			"  Actions",
			"    enter  [select anime]",
			"    /      [new search]",
			"    esc    [back]",
			"",
			"  General",
			"    ?      [toggle help]",
			"    " + modKey() + "+c  [quit]",
		}
	case ScreenEpisodes:
		bindings = []string{
			"  Episodes",
			"    ↑/k    [move up]",
			"    ↓/j    [move down]",
			"",
			"  Actions",
			"    enter  [play episode]",
			"    space  [toggle synopsis]",
			"    d      [dub/sub]",
			"    esc    [back]",
			"",
			"  General",
			"    ?      [toggle help]",
			"    " + modKey() + "+c  [quit]",
		}
	case ScreenWatching:
		bindings = []string{
			"  Player",
			"    ←/h    [prev episode]",
			"    →/l    [next episode]",
			"    space  [replay]",
			"    r      [replay]",
			"",
			"  Options",
			"    s      [cycle source]",
			"    d      [dub/sub]",
			"    esc    [back]",
			"",
			"  General",
			"    ?      [toggle help]",
			"    " + modKey() + "+c  [quit]",
		}
	case ScreenSearching, ScreenLoadingEpisode:
		bindings = []string{
			"  Loading",
			"    esc    [cancel]",
			"",
			"  General",
			"    " + modKey() + "+c  [quit]",
		}
	}

	content := strings.Join(bindings, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Background(lipgloss.AdaptiveColor{Light: "255", Dark: "235"}).
		Padding(1, 2).
		Width(min(48, m.width-8)).
		Render(content)

	return box
}
