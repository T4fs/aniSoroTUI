package scraper

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anitui/anitui/internal/models"
	"go.etcd.io/bbolt"
)

var db *bbolt.DB

func init() {
	path := defaultCachePath()
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)

	var err error
	db, err = bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		db = nil
		return
	}

	db.Update(func(tx *bbolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("show_names"))
		tx.CreateBucketIfNotExists([]byte("episode_titles"))
		tx.CreateBucketIfNotExists([]byte("prefs"))
		tx.CreateBucketIfNotExists([]byte("anime_details"))
		return nil
	})

	loadCaches()
	pruneStaleCaches()
}

func defaultCachePath() string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "aniSoro", "cache.db")
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "aniSoro", "cache.db")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".cache", "aniSoro", "cache.db")
	}
	return filepath.Join(os.TempDir(), "aniSoro-cache.db")
}

func loadCaches() {
	if db == nil {
		return
	}

	db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("show_names"))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		showNameCache.Lock()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			showNameCache.data[string(k)] = string(v)
		}
		showNameCache.Unlock()
		return nil
	})

	db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("episode_titles"))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		anilistCache.Lock()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var titles []string
			if json.Unmarshal(v, &titles) == nil {
				anilistCache.data[string(k)] = titles
			}
		}
		anilistCache.Unlock()
		return nil
	})
}

func pruneStaleCaches() {
	if db == nil {
		return
	}
	db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("anime_details"))
		if b == nil {
			return nil
		}
		var staleKeys [][]byte
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry animeDetailEntry
			if json.Unmarshal(v, &entry) != nil {
				staleKeys = append(staleKeys, k)
				continue
			}
			if time.Since(entry.CachedAt) > cacheTTL {
				staleKeys = append(staleKeys, k)
			}
		}
		for _, k := range staleKeys {
			b.Delete(k)
		}
		return nil
	})
}

func saveShowName(showID, name string) {
	showNameCache.Lock()
	showNameCache.data[showID] = name
	showNameCache.Unlock()

	if db == nil {
		return
	}
	db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("show_names"))
		return b.Put([]byte(showID), []byte(name))
	})
}

func saveEpisodeTitles(showID string, titles []string) {
	if db == nil {
		return
	}
	data, err := json.Marshal(titles)
	if err != nil {
		return
	}
	db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("episode_titles"))
		return b.Put([]byte(showID), data)
	})
}

type animeDetailEntry struct {
	Detail   animeDetail `json:"detail"`
	CachedAt time.Time   `json:"cached_at"`
}

func saveAnimeDetail(name string, detail *animeDetail) {
	animeDetailCache.Lock()
	animeDetailCache.data[name] = detail
	animeDetailCache.Unlock()

	if db == nil {
		return
	}
	entry := animeDetailEntry{Detail: *detail, CachedAt: time.Now()}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("anime_details"))
		return b.Put([]byte(name), data)
	})
}

func loadAnimeDetail(name string) *animeDetail {
	// Check in-memory cache first
	animeDetailCache.Lock()
	if d, ok := animeDetailCache.data[name]; ok {
		animeDetailCache.Unlock()
		return d
	}
	animeDetailCache.Unlock()

	// Check bbolt cache
	if db == nil {
		return nil
	}
	var entry animeDetailEntry
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("anime_details"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(name))
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &entry)
	})
	if err != nil || entry.CachedAt.IsZero() {
		return nil
	}

	// Check TTL
	if time.Since(entry.CachedAt) > cacheTTL {
		return nil
	}

	// Promote to in-memory cache
	animeDetailCache.Lock()
	animeDetailCache.data[name] = &entry.Detail
	animeDetailCache.Unlock()

	return &entry.Detail
}

func cachedFetchAnimeDetails(animeName string) *animeDetail {
	if detail := loadAnimeDetail(animeName); detail != nil {
		return detail
	}
	detail := fetchAnimeDetails(animeName)
	if detail != nil {
		saveAnimeDetail(animeName, detail)
	}
	return detail
}

const (
	allanimeRefr   = "https://allmanga.to"
	allanimeBase   = "allanime.day"
	allanimeAPI    = "https://api." + allanimeBase
	allanimeKeyStr = "Xot36i3lK3:v1"
	cacheTTL       = 24 * time.Hour
)

var allanimeKey = sha256Key(allanimeKeyStr)

type AllanimeScraper struct {
	client *http.Client
}

func NewAllanimeScraper() *AllanimeScraper {
	return &AllanimeScraper{
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *AllanimeScraper) Name() string {
	return "allanime.day"
}

func (s *AllanimeScraper) Search(query string, dub bool) ([]models.Anime, error) {
	tt := "sub"
	if dub {
		tt = "dub"
	}
	gql := `query($search:SearchInput,$limit:Int,$page:Int,$translationType:VaildTranslationTypeEnumType,$countryOrigin:VaildCountryOriginEnumType){shows(search:$search,limit:$limit,page:$page,translationType:$translationType,countryOrigin:$countryOrigin){edges{_id name availableEpisodes __typename}}}`
	vars := map[string]interface{}{
		"search": map[string]interface{}{
			"allowAdult":    false,
			"allowUnknown":  false,
			"query":         query,
		},
		"limit":           20,
		"page":            1,
		"translationType": tt,
		"countryOrigin":   "ALL",
	}

	resp, err := s.graphql(vars, gql)
	if err != nil {
		return nil, err
	}

	type edge struct {
		ID                 string `json:"_id"`
		Name               string `json:"name"`
		AvailableEpisodes  struct {
			Sub int `json:"sub"`
			Dub int `json:"dub"`
		} `json:"availableEpisodes"`
	}
	var result struct {
		Data struct {
			Shows struct {
				Edges []edge `json:"edges"`
			} `json:"shows"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse search: %w", err)
	}

	var animeList []models.Anime
	for _, e := range result.Data.Shows.Edges {
		animeList = append(animeList, models.Anime{
			Title:       e.Name,
			URL:         e.ID,
			Description: fmt.Sprintf("%d sub / %d dub episodes", e.AvailableEpisodes.Sub, e.AvailableEpisodes.Dub),
			Source:      s.Name(),
		})
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i := range animeList {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			detail := cachedFetchAnimeDetails(animeList[idx].Title)
			if detail == nil {
				qLower := strings.ToLower(query)
				nLower := strings.ToLower(animeList[idx].Title)
				if !strings.Contains(nLower, qLower) {
					detail = cachedFetchAnimeDetails(query)
				}
			}
			if detail != nil {
				if detail.EnglishTitle != "" {
					animeList[idx].Title = detail.EnglishTitle
				}
				if detail.Episodes > animeList[idx].EpisodeCount {
					animeList[idx].EpisodeCount = detail.Episodes
				}
				animeList[idx].Score = float64(detail.Score) / 10.0
				animeList[idx].Year = detail.Year
				animeList[idx].Synopsis = detail.Synopsis
				animeList[idx].Genres = detail.Genres
				animeList[idx].Type = detail.Format
				animeList[idx].Studio = detail.Studio
				animeList[idx].Status = detail.Status
			}
		}(i)
	}
	wg.Wait()

	for _, a := range animeList {
		saveShowName(a.URL, a.Title)
	}

	return animeList, nil
}

func (s *AllanimeScraper) GetEpisodes(showID string, dub bool) ([]models.Episode, error) {
	gql := `query($showId:String!){show(_id:$showId){_id name availableEpisodesDetail}}`
	vars := map[string]interface{}{
		"showId": showID,
	}

	resp, err := s.graphql(vars, gql)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Show struct {
				Name   string `json:"name"`
				Detail struct {
					Sub []string `json:"sub"`
					Dub []string `json:"dub"`
				} `json:"availableEpisodesDetail"`
			} `json:"show"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse episodes: %w", err)
	}

	epNums := result.Data.Show.Detail.Sub
	if dub {
		epNums = result.Data.Show.Detail.Dub
	}
	if len(epNums) == 0 {
		epNums = result.Data.Show.Detail.Sub
	}

	showName := result.Data.Show.Name
	showNameCache.Lock()
	if cached, ok := showNameCache.data[showID]; ok {
		showName = cached
	}
	showNameCache.Unlock()
	anilistTitles := fetchEpisodeTitlesSync(showID, showName)

	var episodes []models.Episode
	for _, num := range epNums {
		num = strings.TrimPrefix(num, "EP ")
		num = strings.TrimPrefix(num, "Episode ")
		title := fmt.Sprintf("Episode %s", num)
		epIdx := 0
		if n, err := strconv.Atoi(num); err == nil && n > 0 {
			epIdx = n - 1
			if epIdx >= 0 && epIdx < len(anilistTitles) {
				alTitle := anilistTitles[epIdx]
				if idx := strings.Index(alTitle, " - "); idx >= 0 {
					alTitle = alTitle[idx+3:]
				}
				alTitle = strings.TrimSpace(alTitle)
				if alTitle != "" {
					title = alTitle
				}
			}
		}

		episodes = append(episodes, models.Episode{
			Number: fmt.Sprintf("EP %s", num),
			Title:  title,
			URL:    fmt.Sprintf("%s|%s|%s", showID, num, transType(dub)),
		})
	}

	sort.Slice(episodes, func(i, j int) bool {
		ni, _ := strconv.ParseFloat(strings.TrimPrefix(episodes[i].Number, "EP "), 64)
		nj, _ := strconv.ParseFloat(strings.TrimPrefix(episodes[j].Number, "EP "), 64)
		return ni < nj
	})

	return episodes, nil
}

var anilistCache = struct {
	sync.Mutex
	data map[string][]string
}{data: make(map[string][]string)}

var showNameCache = struct {
	sync.Mutex
	data map[string]string
}{data: make(map[string]string)}

var animeDetailCache = struct {
	sync.Mutex
	data map[string]*animeDetail
}{data: make(map[string]*animeDetail)}

var malIDCache = struct {
	sync.Mutex
	data map[string]int
}{data: make(map[string]int)}

func cachedFetchMALID(animeName string) int {
	malIDCache.Lock()
	if id, ok := malIDCache.data[animeName]; ok {
		malIDCache.Unlock()
		return id
	}
	malIDCache.Unlock()

	id := fetchMALID(animeName)
	if id > 0 {
		malIDCache.Lock()
		malIDCache.data[animeName] = id
		malIDCache.Unlock()
	}
	return id
}

func fetchEpisodeTitlesSync(showID, animeName string) []string {
	anilistCache.Lock()
	cached, ok := anilistCache.data[showID]
	anilistCache.Unlock()

	if ok && !noValidTitles(cached) {
		return cached
	}

	titles := fetchAllEpisodeTitles(animeName)
	if len(titles) == 0 {
		return nil
	}

	anilistCache.Lock()
	anilistCache.data[showID] = titles
	anilistCache.Unlock()

	saveEpisodeTitles(showID, titles)

	return titles
}

func (s *AllanimeScraper) GetVideoURL(episodeURL string, dub bool) ([]models.VideoSource, error) {
	parts := strings.SplitN(episodeURL, "|", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid episode URL format: %s", episodeURL)
	}
	showID := parts[0]
	epNum := parts[1]
	if len(parts) >= 3 {
		dub = parts[2] == "dub"
	}

	providerPaths, err := s.getProviderPaths(showID, epNum, dub)
	if err != nil {
		return nil, err
	}

	type result struct {
		sources []models.VideoSource
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan result)
	fastClient := &http.Client{Timeout: 8 * time.Second}

	for _, pp := range providerPaths {
		go func(ctx context.Context, providerStr string) {
			links := s.getLinksWithClient(fastClient, providerStr)
			if len(links) == 0 {
				return
			}
			select {
			case ch <- result{sources: links}:
			case <-ctx.Done():
			}
		}(ctx, pp)
	}

	var sources []models.VideoSource
	timeout := time.After(10 * time.Second)

	for range providerPaths {
		select {
		case r := <-ch:
			if r.sources != nil {
				sources = append(sources, r.sources...)
			}
			if len(sources) > 0 {
				return sources, nil
			}
		case <-timeout:
			if len(sources) > 0 {
				return sources, nil
			}
			return nil, fmt.Errorf("timed out fetching video for episode %s", epNum)
		}
	}

	if len(sources) > 0 {
		return sources, nil
	}
	return nil, fmt.Errorf("no video sources found for episode %s", epNum)
}

func (s *AllanimeScraper) getProviderPaths(showID, epNum string, dub bool) ([]string, error) {
	tt := "sub"
	if dub {
		tt = "dub"
	}
	vars := map[string]interface{}{
		"showId":            showID,
		"translationType":   tt,
		"episodeString":     epNum,
	}

	exts := fmt.Sprintf(`{"persistedQuery":{"version":1,"sha256Hash":"%s"}}`,
		"d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec")

	varJSON, err := json.Marshal(vars)
	if err != nil {
		return nil, fmt.Errorf("marshal vars: %w", err)
	}
	resp, err := s.get(allanimeAPI+"/api?variables="+url.QueryEscape(string(varJSON))+"&extensions="+url.QueryEscape(exts))
	if err != nil {
		varsQuery := map[string]interface{}{
			"showId":            showID,
			"translationType":   tt,
			"episodeString":     epNum,
		}
		gql := `query($showId:String!,$translationType:VaildTranslationTypeEnumType!,$episodeString:String!){episode(showId:$showId,translationType:$translationType,episodeString:$episodeString){episodeString sourceUrls}}`
		resp, err = s.graphql(varsQuery, gql)
		if err != nil {
			return nil, fmt.Errorf("episode fetch: %w", err)
		}
	}

	var result struct {
		Data struct {
			Tobeparsed string `json:"tobeparsed"`
			Episode    struct {
				SourceUrls []struct {
					SourceName string `json:"sourceName"`
					SourceURL  string `json:"sourceUrl"`
				} `json:"sourceUrls"`
			} `json:"episode"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("parse episode: %w", err)
	}

	if result.Data.Tobeparsed != "" {
		rawURLs, err := s.decodeTobeparsed(result.Data.Tobeparsed)
		if err != nil {
			return nil, fmt.Errorf("decode episode: %w", err)
		}
		var paths []string
		for _, r := range rawURLs {
			parts := strings.SplitN(r, ":", 2)
			if len(parts) != 2 {
				continue
			}
			decoded := decodeHexPath(parts[1])
			paths = append(paths, fmt.Sprintf("%s:%s", parts[0], decoded))
		}
		return paths, nil
	}

	var paths []string
	for _, su := range result.Data.Episode.SourceUrls {
		name := su.SourceName
		u := su.SourceURL
		if strings.HasPrefix(u, "--") {
			u = u[2:]
		}
		if !strings.Contains(u, "http") {
			u = decodeHexPath(u)
		}
		paths = append(paths, fmt.Sprintf("%s:%s", name, u))
	}

	return paths, nil
}

func (s *AllanimeScraper) decodeTobeparsed(encoded string) ([]string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(data) < 30 {
		return nil, fmt.Errorf("data too short: %d bytes", len(data))
	}

	iv := data[1:13]
	ivHex := hex.EncodeToString(iv)
	ctrHex := ivHex + "00000002"

	ctrBytes, err := hex.DecodeString(ctrHex)
	if err != nil {
		return nil, fmt.Errorf("ctr decode: %w", err)
	}

	ctLen := len(data) - 13 - 16
	if ctLen <= 0 {
		return nil, fmt.Errorf("invalid ciphertext length: %d", ctLen)
	}

	ct := data[13 : 13+ctLen]

	block, err := aes.NewCipher(allanimeKey)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}

	stream := cipher.NewCTR(block, ctrBytes)
	plain := make([]byte, len(ct))
	stream.XORKeyStream(plain, ct)

	return s.parseTobeparsedJSON(plain)
}

func (s *AllanimeScraper) parseTobeparsedJSON(plain []byte) ([]string, error) {
	text := string(plain)
	var urls []string
	for {
		idx := strings.Index(text, `"sourceUrl"`)
		if idx == -1 {
			break
		}

		chunk := text[idx:]
		var srcURL, srcName string

		if i := strings.Index(chunk, `"sourceUrl":"--`); i != -1 {
			start := i + len(`"sourceUrl":"--`)
			end := strings.Index(chunk[start:], `"`)
			if end != -1 {
				srcURL = chunk[start : start+end]
			}
		}

		if i := strings.Index(chunk, `"sourceName":"`); i != -1 {
			start := i + len(`"sourceName":"`)
			end := strings.Index(chunk[start:], `"`)
			if end != -1 {
				srcName = chunk[start : start+end]
			}
		}

		if srcURL != "" {
			urls = append(urls, fmt.Sprintf("%s:%s", srcName, srcURL))
		}

		text = chunk[100:]
	}

	return urls, nil
}

func (s *AllanimeScraper) getLinksWithClient(client *http.Client, providerStr string) []models.VideoSource {
	parts := strings.SplitN(providerStr, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	providerPath := parts[1]

	if strings.HasPrefix(providerPath, "http") {
		return []models.VideoSource{{
			URL:     providerPath,
			Type:    "direct",
			Quality: "default",
		}}
	}

	u := fmt.Sprintf("https://%s%s", allanimeBase, providerPath)
	body, err := doGet(client, u)
	if err != nil {
		return nil
	}

	return s.parseAPIStreamSources(string(body))
}

func (s *AllanimeScraper) parseAPIStreamSources(response string) []models.VideoSource {
	var sources []models.VideoSource

	var result struct {
		Links []struct {
			Link          string `json:"link"`
			ResolutionStr string `json:"resolutionStr"`
		} `json:"links"`
		SourceURL string `json:"sourceUrl"`
		M3u8      string `json:"m3u8"`
		Hls       []struct {
			URL         string `json:"url"`
			HardsubLang string `json:"hardsub_lang"`
		} `json:"hls"`
		Subtitles []struct {
			Lang    string `json:"lang"`
			Label   string `json:"label"`
			Default string `json:"default"`
			Src     string `json:"src"`
		} `json:"subtitles"`
		Referer string `json:"Referer"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil
	}

	if len(result.Hls) > 0 {
		for _, h := range result.Hls {
			sources = append(sources, models.VideoSource{
				URL:     h.URL,
				Type:    "m3u8",
				Quality: "default",
			})
		}
	}

	for _, link := range result.Links {
		if link.Link != "" {
			if !strings.HasPrefix(link.Link, "http") {
				if strings.HasPrefix(link.Link, "//") {
					link.Link = "https:" + link.Link
				} else {
					link.Link = "https://" + link.Link
				}
			}
			q := link.ResolutionStr
			if q == "" {
				q = "default"
			}
			sources = append(sources, models.VideoSource{
				URL:     link.Link,
				Type:    "m3u8",
				Quality: q,
			})
		}
	}

	if result.M3u8 != "" {
		sources = append(sources, models.VideoSource{
			URL:     result.M3u8,
			Type:    "m3u8",
			Quality: "default",
		})
	}

	if result.SourceURL != "" {
		sources = append(sources, models.VideoSource{
			URL:     result.SourceURL,
			Type:    "direct",
			Quality: "default",
		})
	}

	return sources
}

var hexMap = map[string]string{
	"79": "A", "7a": "B", "7b": "C", "7c": "D", "7d": "E", "7e": "F", "7f": "G", "70": "H", "71": "I", "72": "J", "73": "K", "74": "L", "75": "M", "76": "N", "77": "O",
	"68": "P", "69": "Q", "6a": "R", "6b": "S", "6c": "T", "6d": "U", "6e": "V", "6f": "W", "60": "X", "61": "Y", "62": "Z",
	"59": "a", "5a": "b", "5b": "c", "5c": "d", "5d": "e", "5e": "f", "5f": "g", "50": "h", "51": "i", "52": "j", "53": "k", "54": "l", "55": "m", "56": "n", "57": "o",
	"48": "p", "49": "q", "4a": "r", "4b": "s", "4c": "t", "4d": "u", "4e": "v", "4f": "w", "40": "x", "41": "y", "42": "z",
	"08": "0", "09": "1", "0a": "2", "0b": "3", "0c": "4", "0d": "5", "0e": "6", "0f": "7", "00": "8", "01": "9",
	"15": "-", "16": ".", "67": "_", "46": "~", "02": ":", "17": "/", "07": "?", "1b": "#",
	"63": "[", "65": "]", "78": "@", "19": "!", "1c": "$", "1e": "&",
	"10": "(", "11": ")", "12": "*", "13": "+", "14": ",",
	"03": ";", "05": "=", "1d": "%",
}

func decodeHexPath(hexStr string) string {
	if len(hexStr)%2 != 0 {
		return hexStr
	}
	var result strings.Builder
	for i := 0; i < len(hexStr); i += 2 {
		pair := strings.ToLower(hexStr[i : i+2])
		if ch, ok := hexMap[pair]; ok {
			result.WriteString(ch)
		}
	}
	path := result.String()
	if strings.Contains(path, "/clock") {
		path = strings.Replace(path, "/clock", "/clock.json", 1)
	}
	return path
}

func (s *AllanimeScraper) graphql(vars map[string]interface{}, query string) ([]byte, error) {
	body := map[string]interface{}{
		"variables": vars,
		"query":     query,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal graphql: %w", err)
	}
	return s.doRequest("POST", allanimeAPI+"/api", jsonBody)
}

func (s *AllanimeScraper) get(urlStr string) ([]byte, error) {
	return s.doRequest("GET", urlStr, nil)
}

func (s *AllanimeScraper) doRequest(method, urlStr string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Referer", allanimeRefr)
	req.Header.Set("Accept", "application/json, */*")
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
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

func doGet(client *http.Client, urlStr string) ([]byte, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Referer", allanimeRefr)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func transType(dub bool) string {
	if dub {
		return "dub"
	}
	return "sub"
}

func sha256Key(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func SaveDubPref(dub bool) {
	if db == nil {
		return
	}
	val := "0"
	if dub {
		val = "1"
	}
	db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("prefs"))
		if b == nil {
			return nil
		}
		return b.Put([]byte("dub"), []byte(val))
	})
}

func LoadDubPref() bool {
	if db == nil {
		return false
	}
	var val string
	db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("prefs"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte("dub"))
		if v != nil {
			val = string(v)
		}
		return nil
	})
	return val == "1"
}
