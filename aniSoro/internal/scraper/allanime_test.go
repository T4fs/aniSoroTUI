package scraper

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mockAllanimeGraphQLResponse(subNums []string) []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"show": map[string]interface{}{
				"_id":  "test-id",
				"name": "Test Anime",
				"availableEpisodesDetail": map[string]interface{}{
					"sub": subNums,
					"dub": []string{},
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func mockAnilistEmptyResponse() []byte {
	return []byte(`{"data":{"Media":{"streamingEpisodes":[],"idMal":0}}}`)
}

func TestAllanimeGetEpisodesNumberSanitization(t *testing.T) {
	tests := []struct {
		name    string
		subNums []string
		want    []string
	}{
		{
			name:    "api returns EP-prefixed numbers",
			subNums: []string{"EP 1", "EP 2", "EP 3", "EP 10", "EP 11"},
			want:    []string{"EP 1", "EP 2", "EP 3", "EP 10", "EP 11"},
		},
		{
			name:    "api returns plain numeric strings",
			subNums: []string{"1", "2", "3"},
			want:    []string{"EP 1", "EP 2", "EP 3"},
		},
		{
			name:    "api returns mixed prefix and plain numbers",
			subNums: []string{"EP 1", "2", "EP 3"},
			want:    []string{"EP 1", "EP 2", "EP 3"},
		},
		{
			name:    "numbers arrive out of order and get sorted",
			subNums: []string{"EP 10", "EP 2", "EP 1"},
			want:    []string{"EP 1", "EP 2", "EP 10"},
		},
		{
			name:    "api returns Episode-prefixed (word) numbers",
			subNums: []string{"Episode 1", "Episode 2"},
			want:    []string{"EP 1", "EP 2"},
		},
		{
			name:    "single digit episodes",
			subNums: []string{"EP 1", "EP 2", "EP 3", "EP 4", "EP 5"},
			want:    []string{"EP 1", "EP 2", "EP 3", "EP 4", "EP 5"},
		},
		{
			name:    "handles empty string in the list",
			subNums: []string{"EP 1", ""},
			want:    []string{"EP ", "EP 1"},
		},
		{
			name:    "dub translation type also gets sanitized",
			subNums: []string{"EP 1", "EP 2"},
			want:    []string{"EP 1", "EP 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origAnilist := anilistHTTP
			origJikan := jikanHTTP
			t.Cleanup(func() {
				anilistHTTP = origAnilist
				jikanHTTP = origJikan
			})

			allanimeBody := mockAllanimeGraphQLResponse(tt.subNums)
			anilistBody := mockAnilistEmptyResponse()

			transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Host, "allanime.day") {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(allanimeBody)),
						Header:     make(http.Header),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(anilistBody)),
					Header:     make(http.Header),
				}, nil
			})

			anilistHTTP = &http.Client{Transport: transport}
			jikanHTTP = &http.Client{Transport: transport}

			s := &AllanimeScraper{
				client: &http.Client{Transport: transport},
			}

			isDub := strings.HasPrefix(tt.name, "dub")
			episodes, err := s.GetEpisodes("test-id", isDub)
			if err != nil {
				t.Fatalf("GetEpisodes() error: %v", err)
			}

			if len(episodes) != len(tt.want) {
				t.Fatalf("got %d episodes, want %d", len(episodes), len(tt.want))
			}

			for i, w := range tt.want {
				got := episodes[i].Number
				if got != w {
					t.Errorf("episodes[%d].Number = %q, want %q", i, got, w)
				}
				if strings.Contains(got, "EP EP ") {
					t.Errorf("episodes[%d].Number has double prefix: %q", i, got)
				}
				sanitizedNum := strings.TrimPrefix(got, "EP ")
				if !strings.Contains(episodes[i].URL, "|"+sanitizedNum+"|") {
					t.Errorf("episodes[%d].URL = %q does not contain sanitized number %q", i, episodes[i].URL, sanitizedNum)
				}
			}

			for i := 1; i < len(episodes); i++ {
				prev := strings.TrimPrefix(episodes[i-1].Number, "EP ")
				cur := strings.TrimPrefix(episodes[i].Number, "EP ")
				pn, _ := strconv.ParseFloat(prev, 64)
				cn, _ := strconv.ParseFloat(cur, 64)
				if pn > cn && cur != "" {
					t.Errorf("episodes not sorted: %s (idx %d) > %s (idx %d)", episodes[i-1].Number, i-1, episodes[i].Number, i)
				}
			}
		})
	}
}
