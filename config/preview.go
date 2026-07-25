package config

import "time"

// PreviewPaginationConfig controls the fixed page/window sizing used by the
// freelancer preview read path.
type PreviewPaginationConfig struct {
	PageSize   int           `env:"GIG_PREVIEW_PAGE_SIZE" envDefault:"1"`
	WindowSize int           `env:"GIG_PREVIEW_WINDOW_SIZE" envDefault:"2"`
	WindowTTL  time.Duration `env:"GIG_PREVIEW_WINDOW_TTL" envDefault:"15m"`
}
