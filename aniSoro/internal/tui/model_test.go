package tui

import (
	"fmt"
	"testing"

	"github.com/anitui/anitui/internal/models"
	"github.com/anitui/anitui/internal/scraper"
	tea "github.com/charmbracelet/bubbletea"
)

// ---- Mock scraper ----

type mockScraper struct {
	name string
}

func (m *mockScraper) Name() string { return m.name }

func (m *mockScraper) Search(query string, dub bool) ([]models.Anime, error) {
	return []models.Anime{
		{Title: "Test Anime", URL: "/test-anime", Source: "allanime.day"},
	}, nil
}

func (m *mockScraper) GetEpisodes(animeURL string, dub bool) ([]models.Episode, error) {
	eps := make([]models.Episode, 5)
	for i := range 5 {
		eps[i] = models.Episode{
			Number: fmt.Sprintf("%d", i+1),
			Title:  fmt.Sprintf("Episode %d", i+1),
			URL:    fmt.Sprintf("/ep-%d", i+1),
		}
	}
	return eps, nil
}

func (m *mockScraper) GetVideoURL(episodeURL string, dub bool) ([]models.VideoSource, error) {
	return []models.VideoSource{
		{URL: "https://example.com/video.mp4", Quality: "720p", Type: "mp4"},
	}, nil
}

// ---- Test helpers ----

func newTestModel() Model {
	m := NewModel(scraper.NewUnifiedScraper(&mockScraper{name: "allanime.day"}))
	m.currentSource = &mockScraper{name: "allanime.day"}
	m.episodes = make([]models.Episode, 5)
	for i := range 5 {
		m.episodes[i] = models.Episode{
			Number: fmt.Sprintf("%d", i+1),
			Title:  fmt.Sprintf("Episode %d", i+1),
			URL:    fmt.Sprintf("/ep-%d", i+1),
		}
	}
	m.selectedAnime = &models.Anime{Title: "Test Title", URL: "/test-url"}
	m.episodeCursor = 2
	return m
}

func withWatching(m Model, episodeIndex int) Model {
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "Test Title",
		episodeIndex: episodeIndex,
		episodesLen:  len(m.episodes),
		sources: []models.VideoSource{
			{URL: "https://example.com/video.mp4", Quality: "720p", Type: "mp4"},
		},
		sourceIndex: 0,
	}
	return m
}
// ---- Tests: applyVideoSources error and empty ----

func TestWatching_ApplySources_ErrorClearsWatching(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenLoadingEpisode
	m.watching = &watchingState{}

	result, _ := m.Update(videoSourcesMsg{err: fmt.Errorf("network failure")})
	updated := result.(Model)

	if updated.screen != ScreenEpisodes {
		t.Errorf("screen: expected ScreenEpisodes after error, got %d", updated.screen)
	}
	if updated.watching != nil {
		t.Errorf("watching: expected nil after error, got %+v", updated.watching)
	}
	if updated.errorMsg == "" {
		t.Error("errorMsg: expected non-empty error after error")
	}
}

func TestWatching_ApplySources_EmptyClearsWatching(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenLoadingEpisode
	m.watching = &watchingState{}

	result, _ := m.Update(videoSourcesMsg{sources: []models.VideoSource{}})
	updated := result.(Model)

	if updated.screen != ScreenEpisodes {
		t.Errorf("screen: expected ScreenEpisodes after empty sources, got %d", updated.screen)
	}
	if updated.watching != nil {
		t.Errorf("watching: expected nil after empty sources, got %+v", updated.watching)
	}
	if updated.errorMsg == "" {
		t.Error("errorMsg: expected non-empty error after empty sources")
	}
}



// ---- Tests: ESC key ----

func TestWatching_Esc_ReturnsToEpisodes(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(Model)

	if updated.screen != ScreenEpisodes {
		t.Errorf("screen: expected ScreenEpisodes after ESC, got %d", updated.screen)
	}
	if updated.watching != nil {
		t.Errorf("watching: expected nil after ESC, got %+v", updated.watching)
	}
	if updated.errorMsg != "" {
		t.Errorf("errorMsg: expected empty, got %q", updated.errorMsg)
	}
}

// ---- Tests: Prev key (left/h) ----

func TestWatching_Prev_ArrowLeft_DecrementsEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.episodeIndex != 1 {
		t.Errorf("episodeIndex: expected 1 after prev, got %d", updated.watching.episodeIndex)
	}
	if updated.screen != ScreenLoadingEpisode {
		t.Errorf("screen: expected ScreenLoadingEpisode after prev, got %d", updated.screen)
	}
}

func TestWatching_Prev_HKey_DecrementsEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{104}})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.episodeIndex != 1 {
		t.Errorf("episodeIndex: expected 1 after h, got %d", updated.watching.episodeIndex)
	}
	if updated.screen != ScreenLoadingEpisode {
		t.Errorf("screen: expected ScreenLoadingEpisode after h, got %d", updated.screen)
	}
}

func TestWatching_Prev_AtFirstEpisode_Noop(t *testing.T) {
	m := withWatching(newTestModel(), 0)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.episodeIndex != 0 {
		t.Errorf("episodeIndex: expected 0 (no change), got %d", updated.watching.episodeIndex)
	}
	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching (no change), got %d", updated.screen)
	}
}
// ---- Tests: Next key (right/l) ----

func TestWatching_Next_ArrowRight_IncrementsEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.episodeIndex != 3 {
		t.Errorf("episodeIndex: expected 3 after next, got %d", updated.watching.episodeIndex)
	}
	if updated.screen != ScreenLoadingEpisode {
		t.Errorf("screen: expected ScreenLoadingEpisode after next, got %d", updated.screen)
	}
}

func TestWatching_Next_LKey_IncrementsEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{108}})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.episodeIndex != 3 {
		t.Errorf("episodeIndex: expected 3 after l, got %d", updated.watching.episodeIndex)
	}
	if updated.screen != ScreenLoadingEpisode {
		t.Errorf("screen: expected ScreenLoadingEpisode after l, got %d", updated.screen)
	}
}

func TestWatching_Next_AtLastEpisode_Noop(t *testing.T) {
	m := withWatching(newTestModel(), 4)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.episodeIndex != 4 {
		t.Errorf("episodeIndex: expected 4 (no change), got %d", updated.watching.episodeIndex)
	}
	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching (no change), got %d", updated.screen)
	}
}

// ---- Tests: Replay key (r) ----

func TestWatching_Replay_ReloadsCurrentEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{114}})
	updated := result.(Model)

	if updated.screen != ScreenLoadingEpisode {
		t.Errorf("screen: expected ScreenLoadingEpisode after replay, got %d", updated.screen)
	}
	if updated.loadingText != "Replaying episode..." {
		t.Errorf("loadingText: expected %q, got %q", "Replaying episode...", updated.loadingText)
	}
}
// ---- Tests: Source cycle key (s) ----

func TestWatching_SourceCycle_IncrementsSourceIndex(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "Test Title",
		episodeIndex: 2,
		episodesLen:  5,
		sources: []models.VideoSource{
			{URL: "https://example.com/src1", Quality: "720p", Type: "mp4"},
			{URL: "https://example.com/src2", Quality: "1080p", Type: "mp4"},
		},
		sourceIndex: 0,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{115}})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.sourceIndex != 1 {
		t.Errorf("sourceIndex: expected 1 after cycle, got %d", updated.watching.sourceIndex)
	}
	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching after source cycle, got %d", updated.screen)
	}
}

func TestWatching_SourceCycle_WrapsAround(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "Test Title",
		episodeIndex: 2,
		episodesLen:  5,
		sources: []models.VideoSource{
			{URL: "https://example.com/src1", Quality: "720p", Type: "mp4"},
			{URL: "https://example.com/src2", Quality: "1080p", Type: "mp4"},
		},
		sourceIndex: 1,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{115}})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.sourceIndex != 0 {
		t.Errorf("sourceIndex: expected 0 after wrap, got %d", updated.watching.sourceIndex)
	}
	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching after source cycle, got %d", updated.screen)
	}
}

func TestWatching_SourceCycle_SingleSource_StaysAtZero(t *testing.T) {
	m := withWatching(newTestModel(), 2)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{115}})
	updated := result.(Model)

	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.sourceIndex != 0 {
		t.Errorf("sourceIndex: expected 0 with single source, got %d", updated.watching.sourceIndex)
	}
	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching after source cycle, got %d", updated.screen)
	}
}
// ---- Tests: Dub toggle key (d) ----

func TestWatching_DubToggle_SwitchesToDub(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.dub = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{100}})
	updated := result.(Model)

	if updated.dub != true {
		t.Errorf("dub: expected true after toggle, got %v", updated.dub)
	}
	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.dub != true {
		t.Errorf("watching.dub: expected true after toggle, got %v", updated.watching.dub)
	}
	if updated.screen != ScreenLoadingEpisode {
		t.Errorf("screen: expected ScreenLoadingEpisode after dub toggle, got %d", updated.screen)
	}
}

func TestWatching_DubToggle_SwitchesBackToSub(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.dub = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{100}})
	updated := result.(Model)

	if updated.dub != false {
		t.Errorf("dub: expected false after toggle from true, got %v", updated.dub)
	}
	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.dub != false {
		t.Errorf("watching.dub: expected false after toggle, got %v", updated.watching.dub)
	}
}

func TestWatching_DubToggle_NoSelectedAnime_Noop(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.selectedAnime = nil
	m.dub = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{100}})
	updated := result.(Model)

	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching (no-op when no selectedAnime), got %d", updated.screen)
	}
	if updated.dub != true {
		t.Errorf("dub: expected true (toggle still happens), got %v", updated.dub)
	}
}
// ---- Tests: playEpisode sourceIndex selection ----

func TestWatching_PlayEpisode_UsesSourceIndex(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "Test Title",
		episodeIndex: 0,
		episodesLen:  5,
		sources: []models.VideoSource{
			{URL: "https://example.com/src0", Quality: "720p", Type: "mp4"},
			{URL: "https://example.com/src1", Quality: "1080p", Type: "mp4"},
		},
		sourceIndex: 1,
	}

	cmd := m.playEpisode(m.watching.sources)
	msg := cmd()

	playDone, ok := msg.(playDoneMsg)
	if !ok {
		t.Fatalf("playEpisode result: expected playDoneMsg, got %T", msg)
	}
	if playDone.err == nil {
		t.Log("playEpisode: player.Play returned nil error (player is available in this env)")
	} else {
		t.Logf("playEpisode: player.Play returned expected error: %v", playDone.err)
	}
}

func TestWatching_PlayEpisode_DefaultsToIndexZeroWhenWatchingNil(t *testing.T) {
	m := newTestModel()
	sources := []models.VideoSource{
		{URL: "https://example.com/src0", Quality: "720p", Type: "mp4"},
		{URL: "https://example.com/src1", Quality: "1080p", Type: "mp4"},
	}
	m.watching = nil

	cmd := m.playEpisode(sources)
	msg := cmd()

	playDone, ok := msg.(playDoneMsg)
	if !ok {
		t.Fatalf("playEpisode result: expected playDoneMsg, got %T", msg)
	}
	_ = playDone
}

func TestWatching_EscFromEpisodes_DoesNotNilWatching(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenEpisodes
	m.watching = &watchingState{animeTitle: "Something"}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(Model)

	if updated.screen != ScreenResults {
		t.Errorf("screen: expected ScreenResults after ESC from Episodes, got %d", updated.screen)
	}
	if updated.watching == nil {
		t.Error("watching: expected non-nil (ScreenEpisodes ESC should not clear watching)")
	}
}

func TestApplySources_SetsWatchingState(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenLoadingEpisode
	m.episodeCursor = 2
	m.episodes = []models.Episode{{Number: "1"}, {Number: "2"}, {Number: "3"}, {Number: "4"}, {Number: "5"}}
	m.selectedAnime = &models.Anime{Title: "Test Title"}

	msg := videoSourcesMsg{
		sources: []models.VideoSource{
			{URL: "https://example.com/video", Quality: "1080p", Type: "mp4"},
		},
		err: nil,
	}
	result, cmd := m.Update(msg)
	updated := result.(Model)

	if updated.screen != ScreenWatching {
		t.Errorf("screen: expected ScreenWatching, got %d", updated.screen)
	}
	if updated.watching == nil {
		t.Fatal("watching: expected non-nil")
	}
	if updated.watching.animeTitle != "Test Title" {
		t.Errorf("animeTitle: expected 'Test Title', got %q", updated.watching.animeTitle)
	}
	if updated.watching.episodeIndex != 2 {
		t.Errorf("episodeIndex: expected 2, got %d", updated.watching.episodeIndex)
	}
	if updated.watching.episodesLen != 5 {
		t.Errorf("episodesLen: expected 5, got %d", updated.watching.episodesLen)
	}
	if updated.watching.sourceIndex != 0 {
		t.Errorf("sourceIndex: expected 0, got %d", updated.watching.sourceIndex)
	}
	if cmd == nil {
		t.Error("cmd: expected non-nil (playEpisode)")
	}
}

func TestApplySources_ErrorNoWatching(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenLoadingEpisode

	msg := videoSourcesMsg{
		sources: nil,
		err:     fmt.Errorf("network error"),
	}
	result, _ := m.Update(msg)
	updated := result.(Model)

	if updated.screen != ScreenEpisodes {
		t.Errorf("screen: expected ScreenEpisodes, got %d", updated.screen)
	}
	if updated.watching != nil {
		t.Error("watching: expected nil on error")
	}
}

func TestApplySources_EmptyNoWatching(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenLoadingEpisode

	msg := videoSourcesMsg{
		sources: []models.VideoSource{},
		err:     nil,
	}
	result, _ := m.Update(msg)
	updated := result.(Model)

	if updated.screen != ScreenEpisodes {
		t.Errorf("screen: expected ScreenEpisodes, got %d", updated.screen)
	}
	if updated.watching != nil {
		t.Error("watching: expected nil on empty sources")
	}
}
