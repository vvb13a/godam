package service

import "strings"

// Config holds the app-level settings that are not stored per asset: upload
// limits, allowed MIME types and where assets are written.
type Config struct {
	MaxUploadSizeBytes int64
	AllowedMimeTypes   map[string]bool
	StoragePath        string
}

func DefaultConfig() *Config {
	return &Config{
		MaxUploadSizeBytes: 100 * 1024 * 1024,
		AllowedMimeTypes: map[string]bool{
			"image/jpeg":      true,
			"image/png":       true,
			"image/webp":      true,
			"image/gif":       true,
			"application/pdf": true,
		},
	}
}

func (c *Config) IsAllowedMime(mime string) bool {
	clean := strings.Split(mime, ";")[0]
	return c.AllowedMimeTypes[strings.TrimSpace(clean)]
}
