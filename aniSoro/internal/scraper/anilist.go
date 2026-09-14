package scraper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const anilistAPI = "https://graphql.anilist.co"
const jikanAPI = "https://api.jikan.moe/v4"

var anilistHTTP = &http.Client{Timeout: 10 * time.Second}
var jikanHTTP = &http.Client{Timeout: 10 * time.Second}

func anilistPost(body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", anilistAPI, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AniTUI/0.1")

	resp, err := anilistHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func nameVariants(name string) []string {
	seen := map[string]bool{}
	var variants []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			variants = append(variants, s)
		}
	}

	add(name)

	if idx := strings.Index(name, "("); idx > 0 {
		add(strings.TrimSpace(name[:idx]))
	}

	if decoded, err := url.QueryUnescape(name); err == nil && decoded != name {
		add(decoded)
	}

	return variants
}

type anilistGraphQLError struct {
	Message string `json:"message"`
}

type anilistEpisode struct {
	Title string `json:"title"`
}

type anilistEpisodeResponse struct {
	Data struct {
		Media struct {
			StreamingEpisodes []anilistEpisode `json:"streamingEpisodes"`
		} `json:"Media"`
	} `json:"data"`
	Errors []anilistGraphQLError `json:"errors"`
}

func fetchEpisodeTitles(animeName string) ([]string, error) {
	query := `query($s:String){Media(search:$s,type:ANIME){streamingEpisodes{title}}}`

	for _, name := range nameVariants(animeName) {
		body := map[string]interface{}{
			"query":     query,
			"variables": map[string]string{"s": name},
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			continue
		}

		resp, err := anilistPost(jsonBody)
		if err != nil {
			continue
		}

		var result anilistEpisodeResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			continue
		}

		if len(result.Errors) > 0 {
			continue
		}

		var titles []string
		for _, ep := range result.Data.Media.StreamingEpisodes {
			titles = append(titles, ep.Title)
		}

		if len(titles) == 0 {
			continue
		}

		for i, j := 0, len(titles)-1; i < j; i, j = i+1, j-1 {
			titles[i], titles[j] = titles[j], titles[i]
		}

		return titles, nil
	}

	return nil, fmt.Errorf("no episode titles found for %q", animeName)
}

type anilistTitleResponse struct {
	Data struct {
		Media struct {
			ID    int    `json:"id"`
			IDMal int    `json:"idMal"`
			Title struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
				Native  string `json:"native"`
			} `json:"title"`
		} `json:"Media"`
	} `json:"data"`
	Errors []anilistGraphQLError `json:"errors"`
}

type anilistDetailResponse struct {
	Data struct {
		Media struct {
			ID     int    `json:"id"`
			IDMal  int    `json:"idMal"`
			Title  struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
				Native  string `json:"native"`
			} `json:"title"`
			Description  string   `json:"description"`
			AverageScore int      `json:"averageScore"`
			SeasonYear   int      `json:"seasonYear"`
			Genres       []string `json:"genres"`
			Episodes     int      `json:"episodes"`
			Format       string   `json:"format"`
			Studios      struct {
				Nodes []struct {
					Name              string `json:"name"`
					IsAnimationStudio bool   `json:"isAnimationStudio"`
				} `json:"nodes"`
			} `json:"studios"`
			Status string `json:"status"`
		} `json:"Media"`
	} `json:"data"`
	Errors []anilistGraphQLError `json:"errors"`
}

type animeDetail struct {
	EnglishTitle string
	Synopsis     string
	Score        int
	Year         int
	Genres       []string
	Episodes     int
	Format       string
	Studio       string
	Status       string
}

func fetchAnimeDetails(animeName string) *animeDetail {
	query := `query($s:String){Media(search:$s,type:ANIME){id idMal title{romaji english native} description averageScore seasonYear genres episodes format studios{ nodes{ name isAnimationStudio } } status}}`
	for _, name := range nameVariants(animeName) {
		body := map[string]interface{}{"query": query, "variables": map[string]string{"s": name}}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			continue
		}
		resp, err := anilistPost(jsonBody)
		if err != nil {
			continue
		}
		var result anilistDetailResponse
		if err := json.Unmarshal(resp, &result); err != nil {
			continue
		}
		if len(result.Errors) > 0 {
			continue
		}
		m := result.Data.Media
		detail := &animeDetail{
			Score:    m.AverageScore,
			Year:     m.SeasonYear,
			Genres:   m.Genres,
			Episodes: m.Episodes,
			Format:   m.Format,
			Status:   m.Status,
		}
		for _, s := range m.Studios.Nodes {
			if s.IsAnimationStudio {
				detail.Studio = s.Name
				break
			}
		}
		synopsis := m.Description
		synopsis = strings.ReplaceAll(synopsis, "<br>", "\n")
		synopsis = strings.ReplaceAll(synopsis, "<br/>", "\n")
		synopsis = strings.ReplaceAll(synopsis, "<br />", "\n")
		var b strings.Builder
		inTag := false
		for _, r := range synopsis {
			if r == '<' {
				inTag = true
				continue
			}
			if r == '>' {
				inTag = false
				continue
			}
			if !inTag {
				b.WriteRune(r)
			}
		}
		detail.Synopsis = strings.TrimSpace(b.String())
		if m.Title.English != "" {
			detail.EnglishTitle = m.Title.English
		} else if m.Title.Romaji != "" {
			detail.EnglishTitle = m.Title.Romaji
		}
		return detail
	}
	return nil
}

func fetchMALID(animeName string) int {
	query := `query($s:String){Media(search:$s,type:ANIME){idMal}}`

	for _, name := range nameVariants(animeName) {
		body := map[string]interface{}{
			"query":     query,
			"variables": map[string]string{"s": name},
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			continue
		}

		resp, err := anilistPost(jsonBody)
		if err != nil {
			continue
		}

		var result struct {
			Data struct {
				Media struct {
					IDMal int `json:"idMal"`
				} `json:"Media"`
			} `json:"data"`
			Errors []anilistGraphQLError `json:"errors"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			continue
		}

		if len(result.Errors) > 0 {
			continue
		}

		if result.Data.Media.IDMal > 0 {
			return result.Data.Media.IDMal
		}
	}

	return 0
}

func isPlaceholder(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	lower := strings.ToLower(s)
	return lower == "untitled" || lower == "tba" || lower == "tbd" || lower == "" || strings.HasPrefix(lower, "episode ")
}

type jikanEpisode struct {
	Title string `json:"title"`
}

func fetchJikanEpisodes(malID int) []string {
	var titles []string
	page := 1

	for {
		url := fmt.Sprintf("%s/anime/%d/episodes?page=%d", jikanAPI, malID, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			break
		}
		req.Header.Set("User-Agent", "AniTUI/0.1")

		resp, err := jikanHTTP.Do(req)
		if err != nil {
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(2 * time.Second)
			continue
		}

		var result struct {
			Data []jikanEpisode `json:"data"`
			Pagination struct {
				HasNextPage bool `json:"has_next_page"`
			} `json:"pagination"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			break
		}

		for _, ep := range result.Data {
			titles = append(titles, ep.Title)
		}

		if !result.Pagination.HasNextPage || page >= 10 {
			break
		}
		page++

		time.Sleep(500 * time.Millisecond)
	}

	return titles
}

func fetchAllEpisodeTitles(animeName string) []string {
	titles, err := fetchEpisodeTitles(animeName)
	if err != nil || noValidTitles(titles) {
		malID := cachedFetchMALID(animeName)
		if malID > 0 {
			return fetchJikanEpisodes(malID)
		}
		return nil
	}

	malID := cachedFetchMALID(animeName)
	if malID > 0 {
		jikanTitles := fetchJikanEpisodes(malID)

		if len(jikanTitles) > len(titles) {
			merged := make([]string, len(jikanTitles))
			for i := range merged {
				switch {
				case i < len(titles) && !isPlaceholder(titles[i]):
					merged[i] = titles[i]
				case i < len(jikanTitles):
					merged[i] = jikanTitles[i]
				}
			}
			return merged
		}

		if noValidTitles(titles) && len(jikanTitles) > 0 {
			return jikanTitles
		}
	}

	return titles
}

func noValidTitles(titles []string) bool {
	valid := 0
	for _, t := range titles {
		if !isPlaceholder(t) {
			valid++
		}
	}
	return valid < len(titles)/2
}
