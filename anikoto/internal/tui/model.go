package tui

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anitui/anitui/internal/models"
	"github.com/anitui/anitui/internal/player"
	"github.com/anitui/anitui/internal/scraper"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Screen int

const (
	ScreenHome Screen = iota
	ScreenSearching
	ScreenResults
	ScreenEpisodes
	ScreenLoadingEpisode
	ScreenWatching
)

const pageSize = 10

var Version = "1.0.0-anikoto"

var debugLog *log.Logger

func init() {
	if os.Getenv("ANIKOTO_DEBUG") == "" {
		debugLog = log.New(io.Discard, "", 0)
		return
	}

	logPath := "anikoto-tui-debug.log"
	if tempDir := os.TempDir(); tempDir != "" {
		logPath = os.ExpandEnv(fmt.Sprintf("%s/anikoto-tui-debug.log", tempDir))
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		debugLog = log.New(io.Discard, "", 0)
		return
	}
	debugLog = log.New(f, "", log.LstdFlags)
}

type searchResultsMsg struct {
	results []models.Anime
	err     error
}

type episodesMsg struct {
	episodes []models.Episode
	err      error
}

type videoSourcesMsg struct {
	sources []models.VideoSource
	err     error
}

type playDoneMsg struct {
	err error
}

type tickMsg struct{}

type watchingState struct {
	animeTitle   string
	episodeIndex  int
	episodesLen   int
	sources       []models.VideoSource
	sourceIndex   int
	dub           bool
}

type Model struct {
	screen Screen
	input  textinput.Model
	width  int
	height int

	scrapers      *scraper.UnifiedScraper
	currentSource scraper.Scraper

	query       string
	results     []models.Anime
	cursor      int
	errorMsg    string
	loadingText string

	episodes      []models.Episode
	selectedAnime *models.Anime
	episodeCursor int

	showFullSynopsis bool

	showHelp bool

	spinIndex int

	lastKey string
	dub     bool

	watching *watchingState

	credit      creditBounds
	creditHover bool

	loadingSince time.Time
	pendingMsg   tea.Msg
	minLoadTime  time.Duration
}

func NewModel(scrapers *scraper.UnifiedScraper) Model {
	ti := textinput.New()
	ti.Placeholder = "Search anime..."
	ti.CharLimit = 100
	ti.Width = 60
	ti.Prompt = "> "
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "4", Dark: "12"})
	ti.Focus()

	return Model{
		screen:      ScreenHome,
		input:       ti,
		scrapers:    scrapers,
		minLoadTime: 600 * time.Millisecond,
		dub:         scraper.LoadDubPref(),
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.credit = m.creditPosition()
		return m, nil

	case tea.MouseMsg:
		if m.screen != ScreenHome || m.showHelp {
			return m, nil
		}
		hover := msg.X >= m.credit.x && msg.X < m.credit.x+m.credit.w &&
			msg.Y >= m.credit.y && msg.Y < m.credit.y+m.credit.h
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && hover {
			debugLog.Printf("[TUI] credit clicked at (%d,%d)", msg.X, msg.Y)
			return m, openURLCmd(creditURL)
		}
		if msg.Action == tea.MouseActionMotion || msg.Action == tea.MouseActionPress {
			if hover != m.creditHover {
				m.creditHover = hover
				debugLog.Printf("[TUI] credit hover=%v at (%d,%d)", hover, msg.X, msg.Y)
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case searchResultsMsg:
		if m.shouldDelay() {
			m.pendingMsg = msg
			return m, nil
		}
		m.pendingMsg = nil
		return m.applySearchResults(msg)

	case episodesMsg:
		if m.shouldDelay() {
			m.pendingMsg = msg
			return m, nil
		}
		m.pendingMsg = nil
		return m.applyEpisodes(msg)

	case videoSourcesMsg:
		debugLog.Printf("[TUI] videoSourcesMsg: err=%v sources=%d screen=%d", msg.err, len(msg.sources), m.screen)
		if msg.err != nil || len(msg.sources) == 0 {
			m.pendingMsg = nil
			return m.applyVideoSources(msg)
		}
		if m.shouldDelay() {
			m.pendingMsg = msg
			return m, nil
		}
		m.pendingMsg = nil
		return m.applyVideoSources(msg)

	case playDoneMsg:
		debugLog.Printf("[TUI] playDoneMsg: err=%v", msg.err)
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("Player error: %v", msg.err)
		}
		return m, nil

	case tickMsg:
		return m.handleTick()

	}

	return m, nil
}

func (m Model) shouldDelay() bool {
	return time.Since(m.loadingSince) < m.minLoadTime
}

func (m Model) handleTick() (tea.Model, tea.Cmd) {
	m.spinIndex++

	if m.pendingMsg != nil && !m.shouldDelay() {
		msg := m.pendingMsg
		m.pendingMsg = nil
		switch p := msg.(type) {
		case searchResultsMsg:
			return m.applySearchResults(p)
		case episodesMsg:
			return m.applyEpisodes(p)
		case videoSourcesMsg:
			return m.applyVideoSources(p)
		}
	}

	if m.screen == ScreenSearching || m.screen == ScreenLoadingEpisode {
		return m, tickCmd()
	}
	return m, nil
}

func (m Model) applySearchResults(msg searchResultsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMsg = msg.err.Error()
		m.screen = ScreenResults
		return m, nil
	}
	m.results = msg.results
	m.cursor = 0
	m.screen = ScreenResults
	m.errorMsg = ""
	return m, nil
}

func (m Model) applyEpisodes(msg episodesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMsg = msg.err.Error()
		m.screen = ScreenEpisodes
		m.watching = nil
		return m, nil
	}
	m.episodes = msg.episodes

	// Handle dub toggle from watching screen: find matching episode and load video
	if m.watching != nil && m.screen == ScreenLoadingEpisode {
		if len(msg.episodes) == 0 {
			m.errorMsg = "No episodes found for this language"
			m.screen = ScreenEpisodes
			m.watching = nil
			return m, nil
		}
		if m.watching.episodeIndex < len(msg.episodes) {
			m.episodeCursor = m.watching.episodeIndex
		} else {
			m.episodeCursor = 0
		}
		m.watching.dub = m.dub
		episode := m.episodes[m.episodeCursor]
		m.loadingText = fmt.Sprintf("Fetching video for %s...", episode.Title)
		m.spinIndex = 0
		m.loadingSince = time.Now()
		m.pendingMsg = nil
		return m, tea.Batch(m.loadVideoURL(episode.URL), tickCmd())
	}

	m.episodeCursor = 0
	m.screen = ScreenEpisodes
	m.errorMsg = ""
	return m, nil
}

func (m Model) applyVideoSources(msg videoSourcesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMsg = fmt.Sprintf("Failed to load video: %v", msg.err)
		m.screen = ScreenEpisodes
		m.watching = nil
		return m, nil
	}
	if len(msg.sources) == 0 {
		m.errorMsg = "No video sources found for this episode"
		m.screen = ScreenEpisodes
		m.watching = nil
		return m, nil
	}

	debugLog.Printf("[TUI] Got %d sources, launching player. URL=%s", len(msg.sources), msg.sources[0].URL)
	m.watching = &watchingState{
		animeTitle:   func() string { if m.selectedAnime != nil { return m.selectedAnime.Title }; return "" }(),
		episodeIndex: m.episodeCursor,
		episodesLen:  len(m.episodes),
		sources:      msg.sources,
		sourceIndex:  0,
		dub:          m.dub,
	}
	m.screen = ScreenWatching
	return m, m.playEpisode(msg.sources)
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Loading screens are cancelable
	if m.screen == ScreenSearching || m.screen == ScreenLoadingEpisode {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.showHelp = false
			m.pendingMsg = nil
			m.errorMsg = ""
			if m.screen == ScreenSearching {
				m.screen = ScreenHome
				m.input.Reset()
				m.input.Focus()
				m.results = nil
				m.episodes = nil
				m.cursor = 0
				return m, textinput.Blink
			}
			// ScreenLoadingEpisode — cancel video loading
			m.watching = nil
			m.screen = ScreenEpisodes
			return m, nil
		}
	}

	// Help popup toggle — work on all screens
	if msg.String() == "?" {
		m.showHelp = !m.showHelp
		return m, nil
	}

	// Any key closes the help popup and then falls through to normal handling
	if m.showHelp {
		m.showHelp = false
		if msg.String() == "esc" {
			return m, nil
		}
	}

	if m.screen == ScreenHome {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "/":
			m.input.Focus()
			return m, textinput.Blink
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	if m.screen == ScreenWatching {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m.handleEsc()
		case "left", "h":
			if m.watching != nil && m.watching.episodeIndex > 0 {
				m.watching.episodeIndex--
				m.episodeCursor = m.watching.episodeIndex
				m.screen = ScreenLoadingEpisode
				m.loadingText = "Loading previous episode..."
				m.spinIndex = 0
				m.errorMsg = ""
				m.loadingSince = time.Now()
				m.pendingMsg = nil
				episode := m.episodes[m.episodeCursor]
				return m, tea.Batch(m.loadVideoURL(episode.URL), tickCmd())
			}
			return m, nil
		case "right", "l":
			if m.watching != nil && m.watching.episodeIndex < m.watching.episodesLen-1 {
				m.watching.episodeIndex++
				m.episodeCursor = m.watching.episodeIndex
				m.screen = ScreenLoadingEpisode
				m.loadingText = "Loading next episode..."
				m.spinIndex = 0
				m.errorMsg = ""
				m.loadingSince = time.Now()
				m.pendingMsg = nil
				episode := m.episodes[m.episodeCursor]
				return m, tea.Batch(m.loadVideoURL(episode.URL), tickCmd())
			}
			return m, nil
		case " ":
			if m.watching != nil {
				m.screen = ScreenLoadingEpisode
				m.loadingText = "Replaying episode..."
				m.spinIndex = 0
				m.errorMsg = ""
				m.loadingSince = time.Now()
				m.pendingMsg = nil
				episode := m.episodes[m.episodeCursor]
				return m, tea.Batch(m.loadVideoURL(episode.URL), tickCmd())
			}
			return m, nil
		case "r":
			if m.watching != nil {
				m.screen = ScreenLoadingEpisode
				m.loadingText = "Replaying episode..."
				m.spinIndex = 0
				m.errorMsg = ""
				m.loadingSince = time.Now()
				m.pendingMsg = nil
				episode := m.episodes[m.episodeCursor]
				return m, tea.Batch(m.loadVideoURL(episode.URL), tickCmd())
			}
			return m, nil
		case "s":
			if m.watching != nil && len(m.watching.sources) > 0 {
				m.watching.sourceIndex = (m.watching.sourceIndex + 1) % len(m.watching.sources)
				m.screen = ScreenWatching
				return m, tea.Batch(m.playEpisode(m.watching.sources))
			}
			return m, nil
		case "d":
			if m.watching != nil {
				m.dub = !m.dub
				scraper.SaveDubPref(m.dub)
				m.watching.dub = m.dub
				if m.selectedAnime != nil {
					m.screen = ScreenLoadingEpisode
					m.loadingText = fmt.Sprintf("Switching to %s...", map[bool]string{true: "dub", false: "sub"}[m.dub])
					m.spinIndex = 0
					m.errorMsg = ""
					m.loadingSince = time.Now()
					m.pendingMsg = nil
					return m, tea.Batch(m.loadEpisodes(m.selectedAnime.URL), tickCmd())
				}
			}
			return m, nil
	}
	return m, nil
}
	key := msg.String()
	isDoubleG := key == "g" && m.lastKey == "g"
	m.lastKey = key

	switch {
	case key == "ctrl+c":
		return m, tea.Quit

	case key == "esc":
		return m.handleEsc()

	case key == "enter":
		return m.handleEnter()

	case key == "up" || key == "k":
		return m.handleUp()

	case key == "down" || key == "j":
		return m.handleDown()

	case key == "g" && isDoubleG:
		return m.handleG()

	case key == "G":
		return m.handleCapitalG()

	case key == "ctrl+u":
		return m.handleCtrlU()

	case key == "ctrl+d":
		return m.handleCtrlD()

	case key == "/":
		return m.handleSlash()

	case key == " " && m.screen == ScreenEpisodes:
		if m.selectedAnime != nil && m.selectedAnime.Synopsis != "" {
			m.showFullSynopsis = !m.showFullSynopsis
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleEsc() (tea.Model, tea.Cmd) {
	m.showHelp = false
	switch m.screen {
	case ScreenResults:
		m.screen = ScreenHome
		m.results = nil
		m.cursor = 0
		m.errorMsg = ""
		m.input.Reset()
		m.input.Focus()
		return m, textinput.Blink
	case ScreenEpisodes:
		m.screen = ScreenResults
		m.episodes = nil
		m.episodeCursor = 0
		m.errorMsg = ""
		m.cursor = 0
		return m, nil
	case ScreenWatching:
		m.screen = ScreenEpisodes
		m.watching = nil
		m.errorMsg = ""
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenHome:
		query := strings.TrimSpace(m.input.Value())
		if query == "" {
			return m, nil
		}
		m.query = query
		m.screen = ScreenSearching
		m.loadingText = fmt.Sprintf("Searching for '%s'...", query)
		m.spinIndex = 0
		m.errorMsg = ""
		m.loadingSince = time.Now()
		return m, tea.Batch(m.performSearch(query), tickCmd())

	case ScreenResults:
		if len(m.results) == 0 {
			return m, nil
		}
		if m.cursor >= len(m.results) {
			m.cursor = 0
		}
		anime := m.results[m.cursor]
		m.selectedAnime = &anime
		m.screen = ScreenLoadingEpisode
		m.loadingText = fmt.Sprintf("Loading episodes for '%s'...", anime.Title)
		m.spinIndex = 0
		m.errorMsg = ""
		m.loadingSince = time.Now()
		m.pendingMsg = nil

		m.currentSource = m.scrapers.GetScraper(anime.Source)
		if m.currentSource == nil {
			m.currentSource = m.scrapers.GetScraper("anidb.se")
		}
		return m, tea.Batch(m.loadEpisodes(anime.URL), tickCmd())

	case ScreenEpisodes:
		if len(m.episodes) == 0 {
			return m, nil
		}
		if m.episodeCursor >= len(m.episodes) {
			m.episodeCursor = 0
		}
		episode := m.episodes[m.episodeCursor]
		m.screen = ScreenLoadingEpisode
		m.loadingText = fmt.Sprintf("Fetching video for %s...", episode.Title)
		m.spinIndex = 0
		m.errorMsg = ""
		m.loadingSince = time.Now()
		m.pendingMsg = nil
		return m, tea.Batch(m.loadVideoURL(episode.URL), tickCmd())
	}

	return m, nil
}

func (m Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenResults:
		if m.cursor > 0 {
			m.cursor--
		}
	case ScreenEpisodes:
		if m.episodeCursor > 0 {
			m.episodeCursor--
			m.showFullSynopsis = false
		}
	}
	return m, nil
}

func (m Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenResults:
		if m.cursor < len(m.results)-1 {
			m.cursor++
		}
	case ScreenEpisodes:
		if m.episodeCursor < len(m.episodes)-1 {
			m.episodeCursor++
			m.showFullSynopsis = false
		}
	}
	return m, nil
}

func (m Model) handleG() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenResults:
		if len(m.results) > 0 {
			m.cursor = 0
		}
	case ScreenEpisodes:
		if len(m.episodes) > 0 {
			m.episodeCursor = 0
			m.showFullSynopsis = false
		}
	}
	return m, nil
}

func (m Model) handleCapitalG() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenResults:
		if len(m.results) > 0 {
			m.cursor = len(m.results) - 1
		}
	case ScreenEpisodes:
		if len(m.episodes) > 0 {
			m.episodeCursor = len(m.episodes) - 1
			m.showFullSynopsis = false
		}
	}
	return m, nil
}

func (m Model) handleCtrlU() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenResults:
		m.cursor = max(0, m.cursor-pageSize)
	case ScreenEpisodes:
		m.episodeCursor = max(0, m.episodeCursor-pageSize)
		m.showFullSynopsis = false
	}
	return m, nil
}

func (m Model) handleCtrlD() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenResults:
		if len(m.results) > 0 {
			m.cursor = min(len(m.results)-1, m.cursor+pageSize)
		}
	case ScreenEpisodes:
		if len(m.episodes) > 0 {
			m.episodeCursor = min(len(m.episodes)-1, m.episodeCursor+pageSize)
			m.showFullSynopsis = false
		}
	}
	return m, nil
}

func (m Model) handleSlash() (tea.Model, tea.Cmd) {
	if m.screen != ScreenHome {
		m.screen = ScreenHome
		m.results = nil
		m.cursor = 0
		m.errorMsg = ""
		m.input.Reset()
		m.input.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m Model) performSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results := m.scrapers.Search(query, m.dub)
		return searchResultsMsg{results: results}
	}
}

func (m Model) loadEpisodes(url string) tea.Cmd {
	return func() tea.Msg {
		episodes, err := m.currentSource.GetEpisodes(url, m.dub)
		return episodesMsg{episodes: episodes, err: err}
	}
}

func (m Model) loadVideoURL(url string) tea.Cmd {
	return func() tea.Msg {
		sources, err := m.currentSource.GetVideoURL(url, m.dub)
		return videoSourcesMsg{sources: sources, err: err}
	}
}

func (m Model) playEpisode(sources []models.VideoSource) tea.Cmd {
	return func() tea.Msg {
		sourceIdx := 0
		if m.watching != nil && m.watching.sourceIndex < len(sources) {
			sourceIdx = m.watching.sourceIndex
		}
		videoURL := sources[sourceIdx].URL
		debugLog.Printf("[TUI] playEpisode: calling player.Play(%s) (source index %d)", videoURL, sourceIdx)
		err := player.Play(videoURL)
		debugLog.Printf("[TUI] playEpisode: player.Play returned err=%v", err)
		return playDoneMsg{err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}
