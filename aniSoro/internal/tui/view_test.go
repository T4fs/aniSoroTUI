package tui

import (
	"strings"
	"testing"

	"github.com/anitui/anitui/internal/models"
	"github.com/charmbracelet/lipgloss"
)

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q\n--- output ---\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q\n--- output ---\n%s", needle, haystack)
	}
}

func TestRenderLogo_BoxWidthSane(t *testing.T) {
	out := RenderLogo(80)
	lines := strings.Split(out, "\n")
	var maxWidth int
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth > 80 {
		t.Fatalf("logo box overflows terminal: rendered width %d > 80\n%s", maxWidth, out)
	}
	if maxWidth < 50 {
		t.Fatalf("logo box unexpectedly narrow: rendered width %d\n%s", maxWidth, out)
	}
	if !strings.Contains(out, "╔══") || !strings.Contains(out, "╚══") {
		t.Fatalf("logo box borders missing\n%s", out)
	}
}

func TestHomeView_WideWindowStaysTight(t *testing.T) {
	m := newTestModel()
	m.width = 160
	m.height = 40
	out := m.View()

	var maxWidth int
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(strings.TrimSpace(line)); w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth > 100 {
		t.Fatalf("home content column must be capped at 100 cols in a 160-wide window (got %d)\n%s", maxWidth, out)
	}

	separator := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "─") && !strings.Contains(line, "╗") && !strings.Contains(line, "╝") {
			separator = strings.TrimSpace(line)
			break
		}
	}
	if w := lipgloss.Width(separator); w > 100 {
		t.Fatalf("separator must be capped at 100 cols (got %d)\n%s", w, out)
	}

	assertContains(t, out, "Watch anime from your terminal")
	assertContains(t, out, "> Search anime")
	assertContains(t, out, "v"+Version)
}

func TestHomeView_ShortWindowPaddingCapped(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	out := m.View()

	lines := strings.Split(out, "\n")
	var maxRun, run int
	for _, line := range lines {
		if lipgloss.Width(strings.TrimRight(line, " ")) == 0 {
			run++
			if run > maxRun {
				maxRun = run
			}
		} else {
			run = 0
		}
	}
	if maxRun > 8 {
		t.Fatalf("vertical padding run of %d blank lines too large for 24-line window\n%s", maxRun, out)
	}
}

func TestWatchingView_ShowsAnimeTitle(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "Test Title")
}

func TestWatchingView_ShowsEpisodeInfo(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "EP 3 - Episode 3")
}

func TestWatchingView_ShowsNowPlaying(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "Now playing in external player...")
}

func TestWatchingView_ShowsSourceQuality(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "Source:")
	assertContains(t, output, "720p")
}

func TestWatchingView_ShowsSubIndicator(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.watching.dub = false
	output := m.View()
	assertContains(t, output, "SUB")
}

func TestWatchingView_ShowsDubIndicator(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.watching.dub = true
	output := m.View()
	assertContains(t, output, "DUB")
}

func TestWatchingView_EpisodeCursorOutOfBounds_NoCrash(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.episodeCursor = 10
	output := m.View()
	assertContains(t, output, "EP  -")
	assertContains(t, output, "Test Title")
}

func TestWatchingView_SourceIndexOutOfBounds_NoCrash(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.watching.sourceIndex = 5
	output := m.View()
	assertContains(t, output, "Source:")
	assertContains(t, output, "Test Title")
}

func TestWatchingView_ShowsLoadingWhenWatchingNil(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenWatching
	m.watching = nil
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "Loading...")
	assertNotContains(t, output, "Now playing")
}

func TestWatchingView_ShowsErrorMessage(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.errorMsg = "Something went wrong"
	output := m.View()
	assertContains(t, output, "Something went wrong")
	assertContains(t, output, "Test Title")
}

func TestWatchingView_Control_PrevShown_WhenNotFirstEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "← prev")
}

func TestWatchingView_Control_PrevHidden_AtFirstEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 0)
	m.width = 80
	m.height = 24
	m.watching.episodeIndex = 0
	output := m.View()
	assertNotContains(t, output, "← prev")
}

func TestWatchingView_Control_NextShown_WhenNotLastEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "next →")
}

func TestWatchingView_Control_NextHidden_AtLastEpisode(t *testing.T) {
	m := withWatching(newTestModel(), 4)
	m.width = 80
	m.height = 24
	m.watching.episodeIndex = 4
	output := m.View()
	assertNotContains(t, output, "next →")
}

func TestWatchingView_Control_ReplayAlwaysShown(t *testing.T) {
	m := withWatching(newTestModel(), 0)
	m.width = 80
	m.height = 24
	m.watching.episodeIndex = 0
	output := m.View()
	assertContains(t, output, "space [replay]")
}

func TestWatchingView_Control_PrevHidden_NextHidden_SingleEpisode(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "Single Ep",
		episodeIndex: 0,
		episodesLen:  1,
		sources: []models.VideoSource{
			{URL: "https://example.com/video.mp4", Quality: "720p", Type: "mp4"},
		},
		sourceIndex: 0,
	}
	output := m.View()
	assertNotContains(t, output, "← Prev")
	assertNotContains(t, output, "Next →")
	assertContains(t, output, "space [replay]")
}

func TestWatchingView_HelpBarMentionedKeys(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "[?]")
	assertContains(t, output, "v"+Version)
}

func TestWatchingView_EmptyEpisodes_NoCrash(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "Empty Eps",
		episodeIndex: 0,
		episodesLen:  0,
		sources: []models.VideoSource{
			{URL: "https://example.com/video.mp4", Quality: "720p", Type: "mp4"},
		},
		sourceIndex: 0,
	}
	m.episodes = nil
	output := m.View()
	assertContains(t, output, "EP  -")
	assertContains(t, output, "Empty Eps")
}

func TestWatchingView_EmptySources_NoCrash(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.screen = ScreenWatching
	m.watching = &watchingState{
		animeTitle:   "No Sources",
		episodeIndex: 0,
		episodesLen:  1,
		sources:      nil,
		sourceIndex:  0,
	}
	m.episodes = []models.Episode{
		{Number: "1", Title: "Ep 1", URL: "/ep-1"},
	}
	m.episodeCursor = 0
	output := m.View()
	assertContains(t, output, "Source:")
	assertContains(t, output, "No Sources")
}

func TestWatchingView_AnimeTitleFallsBackToSelectedAnime(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.watching.animeTitle = ""
	output := m.View()
	assertContains(t, output, "Test Title")
}

func TestWatchingView_NegativeEpisodeCursor_NoCrash(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.episodeCursor = -1
	output := m.View()
	assertContains(t, output, "EP  -")
}

func TestWatchingView_ErrorAboveControls(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 80
	m.height = 24
	m.errorMsg = "Network error occurred"
	output := m.View()
	assertContains(t, output, "Network error occurred")
	assertContains(t, output, "[?]")
}

func TestWatchingView_WidthSet_PanicsNot(t *testing.T) {
	m := withWatching(newTestModel(), 2)
	m.width = 0
	m.height = 24
	_ = m.View()
}

func TestWatchingView_ScreenNotWatching_ReturnsOtherView(t *testing.T) {
	m := newTestModel()
	m.screen = ScreenHome
	m.width = 80
	m.height = 24
	output := m.View()
	assertContains(t, output, "Search anime")
	assertNotContains(t, output, "Now playing")
}
