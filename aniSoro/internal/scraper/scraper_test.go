package scraper

import (
	"strings"
	"testing"
	"time"
)

func TestFullPipeline(t *testing.T) {
	s := NewAllanimeScraper()

	t.Log("Searching for naruto...")
	t0 := time.Now()
	results, err := s.Search("naruto", false)
	t.Logf("Search: %d results, err=%v, took=%v", len(results), err, time.Since(t0))

	if len(results) == 0 {
		t.Fatal("No search results")
	}

	r := results[0]
	t.Logf("Selected: %s (id=%s)", r.Title, r.URL)

	t.Log("Loading episodes...")
	t1 := time.Now()
	eps, err := s.GetEpisodes(r.URL, false)
	t.Logf("Episodes: %d, err=%v, took=%v", len(eps), err, time.Since(t1))

	if len(eps) == 0 {
		t.Fatal("No episodes")
	}

	ep := eps[0]
	t.Logf("Selected episode: %s", ep.Number)

	t.Log("Fetching video URL...")
	t2 := time.Now()
	sources, err := s.GetVideoURL(ep.URL, false)
	t.Logf("Sources: %d, err=%v, took=%v", len(sources), err, time.Since(t2))

	for i, src := range sources {
		t.Logf("  [%d] %s %s: %s", i, src.Quality, src.Type, src.URL[:minStr(80, src.URL)])
	}

	if len(sources) == 0 {
		t.Log("No video sources found (this is OK for test)")
	}
}

func minStr(a int, s string) int {
	if len(s) < a {
		return len(s)
	}
	return a
}

func TestHexDecoder(t *testing.T) {
	hex := "175948514e4c4f57175b54575b5307515c050f5c0a0c0f0b0f0c0e590a0c0b5b0a0c0f0d0f0b0e0c0a5a0f590a5a0f090e0f0f0a0e0d0e5d0a010f0c0e010e0f0e0a0a5a0e010e080a5a0e000e0f0f0c0f0b0f0a0e010a5a0b0f0b5d0b0c0b0c0b0e0b010e0b0f0e0b5a0b5e0b0a0b090b0d0b080a0c0a590a0c0f0d0f0a0f0c0e0b0e0f0e5a0e0b0f0c0c5e0e0a0a0c0b5b0a0c0e000f0d0e0b0f0c0f080e0b0f0c0a0c0a590a0c0e0a0e0f0f0a0e0b0a0c0b5b0a0c0b0c0b0e0b0c0b080a5a0b0e0b0b0a5a0b0e0b5d0d0a0b0f0b090b5b0b0f0b0b0b5b0b0e0b0e0a000b0e0b0e0b0e0d5b0a0c0f5a"
	result := decodeHexPath(hex)
	t.Logf("Decoded: %s", result)
	if !strings.HasPrefix(result, "/") {
		t.Errorf("Expected path starting with /, got: %s", result)
	}

	hex2 := "504c4c484b0217174c5757544b165e594b4c0c4b485d5d5c164a4b4e481717555d5c515901174e515c5d574b170a57605f487c685c0b40736f5c5f565742174b4d5a1709"
	result2 := decodeHexPath(hex2)
	t.Logf("Decoded: %s", result2)
	if !strings.HasPrefix(result2, "http") {
		t.Errorf("Expected URL starting with http, got: %s", result2)
	}
}
