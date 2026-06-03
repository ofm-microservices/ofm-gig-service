package config

import "fmt"

func validatePreviewPaginationConfig(cfg PreviewPaginationConfig) error {
	switch {
	case cfg.PageSize <= 0:
		return WrapParseEnvConfigError(fmt.Errorf("GIG_PREVIEW_PAGE_SIZE must be greater than zero"))
	case cfg.WindowSize <= 0:
		return WrapParseEnvConfigError(fmt.Errorf("GIG_PREVIEW_WINDOW_SIZE must be greater than zero"))
	case cfg.WindowSize%cfg.PageSize != 0:
		return WrapParseEnvConfigError(fmt.Errorf("GIG_PREVIEW_WINDOW_SIZE must be divisible by GIG_PREVIEW_PAGE_SIZE"))
	case cfg.WindowTTL < 0:
		return WrapParseEnvConfigError(fmt.Errorf("GIG_PREVIEW_WINDOW_TTL must not be negative"))
	}
	return nil
}
