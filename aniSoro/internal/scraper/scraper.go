package scraper

import (
	"sync"

	"github.com/anitui/anitui/internal/models"
)

type Scraper interface {
	Name() string
	Search(query string, dub bool) ([]models.Anime, error)
	GetEpisodes(animeURL string, dub bool) ([]models.Episode, error)
	GetVideoURL(episodeURL string, dub bool) ([]models.VideoSource, error)
}

type UnifiedScraper struct {
	scrapers []Scraper
}

func NewUnifiedScraper(scrapers ...Scraper) *UnifiedScraper {
	return &UnifiedScraper{scrapers: scrapers}
}

func (u *UnifiedScraper) Search(query string, dub bool) []models.Anime {
	var mu sync.Mutex
	var allResults []models.Anime
	var wg sync.WaitGroup

	for _, sc := range u.scrapers {
		wg.Add(1)
		go func(s Scraper) {
			defer wg.Done()
			results, err := s.Search(query, dub)
			if err != nil {
				return
			}
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(sc)
	}

	wg.Wait()
	return allResults
}

func (u *UnifiedScraper) GetScraper(name string) Scraper {
	for _, s := range u.scrapers {
		if s.Name() == name {
			return s
		}
	}
	return nil
}
