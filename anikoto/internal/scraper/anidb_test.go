package scraper

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAnidbPipeline(t *testing.T) {
	s := NewAnidbScraper()

	t.Log("Searching for one piece...")
	t0 := time.Now()
	results, err := s.Search("one piece", false)
	t.Logf("Search: %d results, err=%v, took=%v", len(results), err, time.Since(t0))
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("No search results")
	}

	found := false
	for _, r := range results {
		t.Logf("Result: %q | %s | eps=%d type=%s genres=%v", r.Title, r.URL, r.EpisodeCount, r.Type, r.Genres)
		if r.Title == "One Piece" {
			found = true
		}
	}
	if !found {
		t.Log("One Piece not in results (not failing)")
	}

	r := results[0]
	t.Log("Loading episodes...")
	t1 := time.Now()
	eps, err := s.GetEpisodes(r.URL, false)
	t.Logf("Episodes: %d, err=%v, took=%v", len(eps), err, time.Since(t1))
	if err != nil {
		t.Fatalf("GetEpisodes error: %v", err)
	}
	if len(eps) == 0 {
		t.Fatal("No episodes")
	}
	t.Logf("First episode: %s | %s", eps[0].Number, eps[0].URL)

	t.Log("Fetching video URL...")
	t2 := time.Now()
	sources, err := s.GetVideoURL(eps[0].URL, false)
	t.Logf("Sources: %d, err=%v, took=%v", len(sources), err, time.Since(t2))
	if err != nil {
		t.Fatalf("GetVideoURL error: %v", err)
	}
	for i, src := range sources {
		t.Logf("  [%d] %s %s: %s", i, src.Quality, src.Type, src.URL)
	}
	if len(sources) == 0 {
		t.Fatal("No video sources found")
	}

	head, err := http.Head(sources[0].URL)
	if err == nil {
		head.Body.Close()
		t.Logf("Best source HEAD: %d (content-type %s)", head.StatusCode, head.Header.Get("Content-Type"))
	}
}

func TestAnidbDubUnavailable(t *testing.T) {
	s := NewAnidbScraper()

	_, err := s.GetEpisodes("one-piece", true)
	if err == nil {
		t.Fatal("expected a no-dub error, got nil")
	}
	if !strings.Contains(err.Error(), "no dub") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = s.GetVideoURL("https://anidb.se/one-piece-episode-1177-english-subbed/", true)
	if err == nil {
		t.Fatal("expected a no-dub error for video, got nil")
	}
	if !strings.Contains(err.Error(), "no dub") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnidbParseVariants(t *testing.T) {
	playlist := `#EXTM3U
#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=922484,RESOLUTION=1920x1080,FRAME-RATE=23.974,CODECS="avc1.640028,mp4a.40.29"
https://hls.anidb.app/stream/abc/index-f1-v1-a1.m3u8
#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=540880,RESOLUTION=1280x720,FRAME-RATE=23.974,CODECS="avc1.640028,mp4a.40.29"
https://hls.anidb.app/stream/abc/index-f2-v1-a1.m3u8
#EXT-X-STREAM-INF:PROGRAM-ID=1,BANDWIDTH=229348,RESOLUTION=640x360,FRAME-RATE=23.974,CODECS="avc1.640028,mp4a.40.29"
https://hls.anidb.app/stream/abc/index-f3-v1-a1.m3u8
#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=160806,RESOLUTION=1920x1080,CODECS="avc1.640028",URI="https://hls.anidb.app/stream/abc/iframes-f1-v1-a1.m3u8"
#EXT-X-ENDLIST`

	sources := parseAnidbVariants(playlist, "https://hls.anidb.app/stream/abc/master.m3u8")
	if len(sources) != 3 {
		t.Fatalf("expected 3 variants, got %d: %+v", len(sources), sources)
	}
	if sources[0].Quality != "1080p" {
		t.Errorf("expected best source 1080p, got %s", sources[0].Quality)
	}
	if sources[2].Quality != "360p" {
		t.Errorf("expected worst source 360p, got %s", sources[2].Quality)
	}
	if sources[1].Type != "hls" {
		t.Errorf("expected type hls, got %s", sources[1].Type)
	}
}