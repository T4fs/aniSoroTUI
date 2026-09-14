package models

type Anime struct {
	Title        string
	URL          string
	Description  string
	Source       string
	EpisodeCount int
	Score        float64
	Genres       []string
	Year         int
	Synopsis     string
	Type         string // "TV", "Movie", "OVA", "Special", "ONA", "Music"
	Studio       string
	Status       string // "FINISHED", "RELEASING", "NOT_YET_RELEASED", "CANCELLED", "HIATUS"
}
