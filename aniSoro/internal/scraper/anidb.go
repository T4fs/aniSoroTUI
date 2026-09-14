package scraper

import (
	"encoding/base64"
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
	"time"

	"github.com/anitui/anitui/internal/models"
)

const (
	anidbBase = "https://anidb.se"
	anidbUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
)

var (
	anidbEpNumRe = regexp.MustCompile(`-episode-(\d+)-`)
	anidbEplRe   = regexp.MustCompile(`<li data-index="\d+">\s*<a href="([^"]+)">[\s\S]*?<div class="epl-num">([^<]*)</div>\s*<div class="epl-title">([^<]*)</div>`)
	anidbMirrorRe = regexp.MustCompile(`<option value="([^"]+)"\s+data-index="\d+"[^>]*>\s*([^<]*)</option>`)
	anidbDataSrcRe = regexp.MustCompile(`data-src="([^"]+)"`)
	anidbIframeRe  = regexp.MustCompile(`<iframe[^>]*src="([^"]+)"`)
	anidbFileRe    = regexp.MustCompile(`file:\s*'([^']*)'`)
)

type AnidbScraper struct {
	client *http.Client
}

func NewAnidbScraper() *AnidbScraper {
	return &AnidbScraper{
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *AnidbScraper) Name() string {
	return "anidb.se"
}

func (s *AnidbScraper) get(rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", anidbUA)
		req.Header.Set("Accept", "text/html,application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 600 * time.Millisecond)
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("anidb.se returned status %d", resp.StatusCode)
			resp.Body.Close()
			time.Sleep(time.Duration(attempt+1) * 800 * time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("anidb.se returned status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(body), "Just a moment") {
			return nil, fmt.Errorf("anidb.se blocked the request (cloudflare challenge)")
		}
		return body, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("anidb.se request failed")
	}
	return nil, lastErr
}

func (s *AnidbScraper) postSearch(query string) ([]byte, error) {
	form := url.Values{}
	form.Set("action", "ts_ac_do_search")
	form.Set("ts_ac_query", query)

	req, err := http.NewRequest(http.MethodPost, anidbBase+"/wp-admin/admin-ajax.php", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", anidbUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json,text/javascript,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anidb.se search returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(body), "Just a moment") {
		return nil, fmt.Errorf("anidb.se blocked the request (cloudflare challenge)")
	}
	return body, nil
}

type anidbSearchJSON struct {
	Anime []struct {
		All []struct {
			PostTitle  string `json:"post_title"`
			PostLink   string `json:"post_link"`
			PostGenres string `json:"post_genres"`
			PostType   string `json:"post_type"`
			PostLatest string `json:"post_latest"`
		} `json:"all"`
	} `json:"anime"`
}

func (s *AnidbScraper) Search(query string, dub bool) ([]models.Anime, error) {
	body, err := s.postSearch(query)
	if err != nil {
		return nil, err
	}

	var parsed anidbSearchJSON
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}

	results := make([]models.Anime, 0, 10)
	seen := make(map[string]bool)
	for _, group := range parsed.Anime {
		for _, a := range group.All {
			title := strings.TrimSpace(a.PostTitle)
			slug := anidbSlugFromLink(a.PostLink)
			if title == "" || slug == "" || seen[slug] {
				continue
			}
			seen[slug] = true

			latest := 0
			if n, err := strconv.Atoi(a.PostLatest); err == nil {
				latest = n
			}

			var genres []string
			for _, g := range strings.Split(a.PostGenres, ",") {
				if g = strings.TrimSpace(g); g != "" {
					genres = append(genres, g)
				}
			}

			anime := models.Anime{
				Title:        title,
				URL:          slug,
				Source:       s.Name(),
				EpisodeCount: latest,
				Type:         a.PostType,
				Genres:       genres,
			}
			if latest > 0 {
				anime.Description = fmt.Sprintf("Up to EP %d", latest)
			}
			results = append(results, anime)
		}
	}

	if len(results) > 15 {
		results = results[:15]
	}
	return results, nil
}

func anidbSlugFromLink(link string) string {
	prefix := anidbBase + "/anime/"
	if i := strings.Index(link, prefix); i >= 0 {
		slug := link[i+len(prefix):]
		slug = strings.TrimSuffix(slug, "/")
		if slug != "" {
			return slug
		}
	}
	return ""
}

func (s *AnidbScraper) GetEpisodes(animeURL string, dub bool) ([]models.Episode, error) {
	if dub {
		return nil, fmt.Errorf("no dub available for this anime (anidb.se is sub-only)")
	}

	slug := animeURL
	if strings.HasPrefix(animeURL, anidbBase+"/anime/") {
		slug = anidbSlugFromLink(animeURL)
	}
	if slug == "" {
		return nil, fmt.Errorf("could not extract anime slug from %q", animeURL)
	}

	body, err := s.get(anidbBase + "/anime/" + url.PathEscape(slug))
	if err != nil {
		return nil, err
	}
	page := string(body)

	episodes := make([]models.Episode, 0, 8)
	seen := make(map[string]bool)
	for _, m := range anidbEplRe.FindAllStringSubmatch(page, -1) {
		href := html.UnescapeString(m[1])
		num := strings.TrimSpace(m[2])
		title := strings.TrimSpace(html.UnescapeString(m[3]))
		if href == "" || seen[href] {
			continue
		}
		if num == "" {
			if nm := anidbEpNumRe.FindStringSubmatch(href); nm != nil {
				num = nm[1]
			}
		}
		if num == "" {
			continue
		}
		seen[href] = true
		episodes = append(episodes, models.Episode{
			Number: "EP " + num,
			Title:  title,
			URL:    href,
		})
	}

	if len(episodes) == 0 {
		return nil, fmt.Errorf("no episodes found for %q", slug)
	}

	sort.Slice(episodes, func(i, j int) bool {
		ni, _ := strconv.ParseFloat(strings.TrimPrefix(episodes[i].Number, "EP "), 64)
		nj, _ := strconv.ParseFloat(strings.TrimPrefix(episodes[j].Number, "EP "), 64)
		return ni < nj
	})
	return episodes, nil
}

func (s *AnidbScraper) GetVideoURL(episodeURL string, dub bool) ([]models.VideoSource, error) {
	if dub {
		return nil, fmt.Errorf("no dub available for this anime (anidb.se is sub-only)")
	}

	body, err := s.get(episodeURL)
	if err != nil {
		return nil, err
	}
	page := string(body)

	var sources []models.VideoSource
	seen := make(map[string]bool)
	for _, m := range anidbMirrorRe.FindAllStringSubmatch(page, -1) {
		encoded := m[1]
		serverName := strings.TrimSpace(m[2])
		if encoded == "" {
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		embed := string(decoded)

		mediaURL := ""
		if ds := anidbDataSrcRe.FindStringSubmatch(embed); ds != nil {
			mediaURL = html.UnescapeString(ds[1])
		} else if fr := anidbIframeRe.FindStringSubmatch(embed); fr != nil {
			iframeURL := html.UnescapeString(fr[1])
			if !strings.HasPrefix(iframeURL, "http") {
				iframeURL = anidbBase + iframeURL
			}
			if ib, err := s.get(iframeURL); err == nil {
				if ds := anidbDataSrcRe.FindStringSubmatch(string(ib)); ds != nil {
					mediaURL = html.UnescapeString(ds[1])
				} else if fl := anidbFileRe.FindStringSubmatch(string(ib)); fl != nil {
					mediaURL = html.UnescapeString(fl[1])
				}
			}
		}

		if mediaURL == "" || seen[mediaURL] {
			continue
		}
		seen[mediaURL] = true

		if strings.Contains(mediaURL, ".m3u8") {
			pl, err := s.get(mediaURL)
			if err != nil {
				continue
			}
			variants := parseAnidbVariants(string(pl), mediaURL)
			if len(variants) > 0 {
				sources = append(sources, variants...)
			} else {
				sources = append(sources, models.VideoSource{URL: mediaURL, Quality: serverName, Type: "hls"})
			}
		} else {
			sources = append(sources, models.VideoSource{
				URL:     mediaURL,
				Quality: serverName,
				Type:    "mp4",
			})
		}
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no playable video server found on this episode page")
	}

	dedup := make([]models.VideoSource, 0, len(sources))
	for _, src := range sources {
		key := src.URL + "|" + src.Quality
		if seen[key] {
			continue
		}
		seen[key] = true
		dedup = append(dedup, src)
	}
	return dedup, nil
}

var anidbVariantRe = regexp.MustCompile(`(?m)^#EXT-X-STREAM-INF:[^\n]*RESOLUTION=(\d+)x(\d+)[^\n]*\n\s*(https?://\S+)`)

func parseAnidbVariants(playlist, masterURL string) []models.VideoSource {
	sources := make([]models.VideoSource, 0, 4)
	for _, m := range anidbVariantRe.FindAllStringSubmatch(playlist, -1) {
		height := m[2]
		sources = append(sources, models.VideoSource{
			URL:     strings.TrimSpace(m[3]),
			Quality: height + "p",
			Type:    "hls",
		})
	}

	sort.Slice(sources, func(i, j int) bool {
		hi, _ := strconv.Atoi(strings.TrimSuffix(sources[i].Quality, "p"))
		hj, _ := strconv.Atoi(strings.TrimSuffix(sources[j].Quality, "p"))
		return hi > hj
	})

	return sources
}