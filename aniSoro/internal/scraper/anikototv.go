package scraper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anitui/anitui/internal/models"
)

const (
	anikototvBase = "https://anikototv.to"
	megaplayBase  = "https://megaplay.buzz"
	anikototvUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
)

type AnikototvScraper struct {
	client *http.Client
}

var genreRe = regexp.MustCompile(`/genre/[a-z0-9-]+"[^>]*>\s*([^<]+)`)

func NewAnikototvScraper() *AnikototvScraper {
	return &AnikototvScraper{
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *AnikototvScraper) Name() string {
	return "anikototv.to"
}

func (s *AnikototvScraper) Search(query string, dub bool) ([]models.Anime, error) {
	body, err := s.get(anikototvBase + "/filter?keyword=" + url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	page := string(body)

	chunks := strings.Split(page, `<div class="item ">`)

	animeList := make([]models.Anime, 0, len(chunks))
	for _, item := range chunks[1:] {

		slug := firstMatch(item, `href="https://anikototv\.to/watch/([^"/]+)`)
		if slug == "" {
			continue
		}
		title := strings.TrimSpace(html.UnescapeString(firstMatch(item, `alt="([^"]+)"`)))
		if title == "" {
			continue
		}

		var subCount, dubCount int
		if v := firstMatch(item, `<span class="ep-status sub"><span>\s*(\d+)`); v != "" {
			subCount, _ = strconv.Atoi(v)
		}
		if v := firstMatch(item, `<span class="ep-status dub"><span>\s*(\d+)`); v != "" {
			dubCount, _ = strconv.Atoi(v)
		}
		epCount := subCount
		if dubCount > epCount {
			epCount = dubCount
		}

		anime := models.Anime{
			Title:        title,
			URL:          slug,
			Source:       s.Name(),
			EpisodeCount: epCount,
			Type:         strings.TrimSpace(firstMatch(item, `<div class="right">([^<]+)</div>`)),
		}
		if epCount > 0 {
			anime.Description = fmt.Sprintf("%d episodes", epCount)
		}
		if v := firstMatch(item, `<div class="m-item rated">\s*<span>([0-9.]+)</span>`); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				anime.Score = f
			}
		}
		for _, g := range genreRe.FindAllStringSubmatch(item, -1) {
			gName := strings.TrimSpace(html.UnescapeString(g[1]))
			if gName != "" {
				anime.Genres = append(anime.Genres, gName)
			}
		}

		animeList = append(animeList, anime)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i := range animeList {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			detail := s.fetchWatchDetails(animeList[idx].URL)
			if detail == nil {
				return
			}
			if detail.Title != "" {
				animeList[idx].Title = detail.Title
			}
			if detail.Score > 0 {
				animeList[idx].Score = detail.Score
			}
			if detail.Year > 0 {
				animeList[idx].Year = detail.Year
			}
			if detail.EpisodeCount > 0 {
				animeList[idx].EpisodeCount = detail.EpisodeCount
			}
			animeList[idx].Synopsis = detail.Synopsis
			if len(detail.Genres) > 0 {
				animeList[idx].Genres = detail.Genres
			}
			if detail.Type != "" {
				animeList[idx].Type = detail.Type
			}
			animeList[idx].Studio = detail.Studio
			animeList[idx].Status = detail.Status
			if detail.EpisodeCount > 0 {
				animeList[idx].Description = fmt.Sprintf("%d episodes", detail.EpisodeCount)
			}
		}(i)
	}
	wg.Wait()

	return animeList, nil
}

type watchDetail struct {
	Title        string
	Score        float64
	Year         int
	EpisodeCount int
	Synopsis     string
	Genres       []string
	Type         string
	Studio       string
	Status       string
}

func (s *AnikototvScraper) fetchWatchDetails(slug string) *watchDetail {
	body, err := s.get(anikototvBase + "/watch/" + slug + "/ep-1")
	if err != nil {
		return nil
	}
	page := string(body)

	detail := &watchDetail{}

	if m := regexp.MustCompile(`<h1[^>]*class="title[^"]*"[^>]*>([\s\S]*?)</h1>`).FindStringSubmatch(page); m != nil {
		detail.Title = strings.TrimSpace(stripTags(m[1]))
	}
	if m := regexp.MustCompile(`<div class="content">([\s\S]*?)</div>`).FindStringSubmatch(page); m != nil {
		detail.Synopsis = strings.TrimSpace(stripTags(m[1]))
	}
	if m := regexp.MustCompile(`<div>\s*MAL:\s*<span>\s*([0-9.]+)\s*</span>`).FindStringSubmatch(page); m != nil {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			detail.Score = f
		}
	}
	if m := regexp.MustCompile(`<div>\s*Episodes:\s*<span>\s*([0-9]+)\s*</span>`).FindStringSubmatch(page); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			detail.EpisodeCount = n
		}
	}
	if m := regexp.MustCompile(`<div>\s*Type:\s*<span>([\s\S]*?)</span>`).FindStringSubmatch(page); m != nil {
		detail.Type = strings.TrimSpace(stripTags(m[1]))
	}
	if m := regexp.MustCompile(`<div>\s*Status:\s*<span>([\s\S]*?)</span>`).FindStringSubmatch(page); m != nil {
		detail.Status = mapStatus(strings.TrimSpace(stripTags(m[1])))
	}
	if m := regexp.MustCompile(`<div>\s*Premiered:\s*<span>([\s\S]*?)</span>`).FindStringSubmatch(page); m != nil {
		if ym := regexp.MustCompile(`(\d{4})`).FindStringSubmatch(m[1]); ym != nil {
			if y, err := strconv.Atoi(ym[1]); err == nil {
				detail.Year = y
			}
		}
	}
	if m := regexp.MustCompile(`<div>\s*Studios:\s*<span>([\s\S]*?)</span>`).FindStringSubmatch(page); m != nil {
		detail.Studio = strings.TrimSpace(stripTags(m[1]))
	}
	if m := regexp.MustCompile(`<div>\s*Genres:\s*<span>([\s\S]*?)</span>`).FindStringSubmatch(page); m != nil {
		for _, g := range strings.Split(m[1], ",") {
			g = strings.TrimSpace(stripTags(g))
			if g != "" {
				detail.Genres = append(detail.Genres, g)
			}
		}
	}

	return detail
}

func mapStatus(status string) string {
	switch strings.ToLower(status) {
	case "finished airing":
		return "FINISHED"
	case "currently airing":
		return "RELEASING"
	case "not yet aired", "upcoming":
		return "NOT_YET_RELEASED"
	case "on hiatus":
		return "HIATUS"
	case "cancelled":
		return "CANCELLED"
	default:
		return status
	}
}

func (s *AnikototvScraper) GetEpisodes(animeURL string, dub bool) ([]models.Episode, error) {
	watchBody, err := s.get(anikototvBase + "/watch/" + animeURL + "/ep-1")
	if err != nil {
		return nil, err
	}

	idMatch := regexp.MustCompile(`id="watch-main"[^>]*data-id="(\d+)"`).FindSubmatch(watchBody)
	if idMatch == nil {
		return nil, fmt.Errorf("could not find anime id on watch page")
	}
	animeID := string(idMatch[1])

	epBody, err := s.get(anikototvBase + "/ajax/episode/list/" + animeID)
	if err != nil {
		return nil, err
	}

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(epBody, &result); err != nil {
		return nil, fmt.Errorf("parse episodes: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("episode list failed with status %d", result.Status)
	}

	liRe := regexp.MustCompile(`<li title="([^"]*)"[\s\S]*?data-id="([^"]*)"[\s\S]*?data-num="([^"]*)"[\s\S]*?data-sub="([^"]*)"[\s\S]*?data-dub="([^"]*)"[\s\S]*?data-ids="([^"]*)"`)
	matches := liRe.FindAllStringSubmatch(result.Result, -1)

	var episodes []models.Episode
	for _, m := range matches {
		title := strings.TrimSpace(html.UnescapeString(m[1]))
		num := m[3]
		hasSub := m[4] == "1"
		hasDub := m[5] == "1"
		dataIDs := m[6]

		if dub && !hasDub {
			continue
		}
		if !dub && !hasSub {
			continue
		}

		episodes = append(episodes, models.Episode{
			Number: fmt.Sprintf("EP %s", num),
			Title:  title,
			URL:    fmt.Sprintf("%s|%s|%s", animeID, dataIDs, transType(dub)),
		})
	}

	sort.Slice(episodes, func(i, j int) bool {
		ni, _ := strconv.ParseFloat(strings.TrimPrefix(episodes[i].Number, "EP "), 64)
		nj, _ := strconv.ParseFloat(strings.TrimPrefix(episodes[j].Number, "EP "), 64)
		return ni < nj
	})

	return episodes, nil
}

func (s *AnikototvScraper) GetVideoURL(episodeURL string, dub bool) ([]models.VideoSource, error) {
	parts := strings.SplitN(episodeURL, "|", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid episode URL format: %s", episodeURL)
	}
	dataIDs := parts[1]
	dub = parts[2] == "dub"

	servers, err := s.getServers(dataIDs)
	if err != nil {
		return nil, err
	}

	want := "sub"
	if dub {
		want = "dub"
	}
	var chosen []serverInfo
	for _, sv := range servers {
		if sv.typ == want {
			chosen = append(chosen, sv)
		}
	}
	if len(chosen) == 0 {
		chosen = servers
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("no servers found for episode")
	}
	if len(chosen) > 4 {
		chosen = chosen[:4]
	}

	var mu sync.Mutex
	var sources []models.VideoSource
	var wg sync.WaitGroup
	for _, sv := range chosen {
		wg.Add(1)
		go func(sv serverInfo) {
			defer wg.Done()
			streamURL, err := s.resolveServer(sv)
			if err != nil {
				return
			}
			mu.Lock()
			sources = append(sources, models.VideoSource{
				URL:     streamURL,
				Type:    "m3u8",
				Quality: "default",
			})
			mu.Unlock()
		}(sv)
	}
	wg.Wait()

	if len(sources) == 0 {
		return nil, fmt.Errorf("no playable video sources found")
	}
	return sources, nil
}

type serverInfo struct {
	typ    string
	linkID string
	name   string
}

func (s *AnikototvScraper) getServers(dataIDs string) ([]serverInfo, error) {
	body, err := s.get(anikototvBase + "/ajax/server/list?servers=" + url.QueryEscape(dataIDs))
	if err != nil {
		return nil, err
	}

	var result struct {
		Status int    `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse servers: %w", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("server list failed with status %d", result.Status)
	}

	sectionRe := regexp.MustCompile(`<div class="type" data-type="(sub|dub)">([\s\S]*?)</div>`)
	liRe := regexp.MustCompile(`data-link-id="([^"]*)"[\s\S]*?>([^<]*)</li>`)

	var servers []serverInfo
	for _, section := range sectionRe.FindAllStringSubmatch(result.Result, -1) {
		typ := section[1]
		for _, li := range liRe.FindAllStringSubmatch(section[2], -1) {
			servers = append(servers, serverInfo{
				typ:    typ,
				linkID: li[1],
				name:   strings.TrimSpace(li[2]),
			})
		}
	}
	return servers, nil
}

func (s *AnikototvScraper) resolveServer(sv serverInfo) (string, error) {
	body, err := s.get(anikototvBase + "/ajax/server?get=" + url.QueryEscape(sv.linkID))
	if err != nil {
		return "", err
	}

	var result struct {
		Status int `json:"status"`
		Result struct {
			URL string `json:"url"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse server: %w", err)
	}
	if result.Status != 200 || result.Result.URL == "" {
		return "", fmt.Errorf("server %s returned no url", sv.name)
	}

	return s.resolveEmbed(result.Result.URL, sv.typ)
}

func (s *AnikototvScraper) resolveEmbed(embedURL, typ string) (string, error) {
	body, err := s.get(embedURL)
	if err != nil {
		return "", err
	}
	page := string(body)

	id := firstMatch(page, `data-id="(\d+)"`)
	realID := firstMatch(page, `data-realid="(\d+)"`)
	mediaID := firstMatch(page, `data-mediaid="(\d+)"`)
	cid := firstMatch(page, `cid\s*:\s*'([^']+)'`)
	cidu := firstMatch(page, `cidu\s*:\s*'([^']+)'`)

	if id == "" || cid == "" {
		return "", fmt.Errorf("could not parse embed player for %s", embedURL)
	}
	if realID == "" {
		if m := regexp.MustCompile(`/stream/s-\d+/(\d+)/`).FindStringSubmatch(embedURL); m != nil {
			realID = m[1]
		} else {
			realID = id
		}
	}
	if typ == "" {
		if m := regexp.MustCompile(`/(sub|dub)/?$`).FindStringSubmatch(embedURL); m != nil {
			typ = m[1]
		} else {
			typ = "sub"
		}
	}

	apiURL := fmt.Sprintf("%s/stream/getSourcesNew?id=%s&realid=%s&mediaid=%s&cid=%s&cidu=%s&type=%s",
		megaplayBase, id, realID, mediaID, cid, cidu, typ)

	apiBody, err := s.getWithHeaders(apiURL, map[string]string{
		"X-Requested-With": "XMLHttpRequest",
		"Referer":          megaplayBase,
	})
	if err != nil {
		return "", err
	}

	var srcResult struct {
		Sources struct {
			File string `json:"file"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(apiBody, &srcResult); err != nil {
		return "", fmt.Errorf("parse stream sources: %w", err)
	}
	if srcResult.Sources.File == "" {
		return "", fmt.Errorf("no stream file in sources response")
	}
	return srcResult.Sources.File, nil
}

func firstMatch(s, pattern string) string {
	if m := regexp.MustCompile(pattern).FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

var stripTagRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string {
	return html.UnescapeString(html.UnescapeString(stripTagRe.ReplaceAllString(s, "")))
}

func (s *AnikototvScraper) get(urlStr string) ([]byte, error) {
	return s.doRequest("GET", urlStr, nil, map[string]string{
		"X-Requested-With": "XMLHttpRequest",
	})
}

func (s *AnikototvScraper) getWithHeaders(urlStr string, headers map[string]string) ([]byte, error) {
	return s.doRequest("GET", urlStr, nil, headers)
}

func (s *AnikototvScraper) doRequest(method, urlStr string, body []byte, extraHeaders map[string]string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", anikototvUA)
	req.Header.Set("Referer", anikototvBase)
	req.Header.Set("Accept", "application/json, text/html, */*")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}