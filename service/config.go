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
			// Images
			"image/jpeg":    true,
			"image/png":     true,
			"image/webp":    true,
			"image/gif":     true,
			"image/tiff":    true,
			"image/bmp":     true,
			"image/svg+xml": true,
			// Documents
			"application/pdf": true,
			// Fonts
			"font/ttf":                      true,
			"font/otf":                      true,
			"font/sfnt":                     true,
			"font/woff":                     true,
			"font/woff2":                    true,
			"font/collection":               true,
			"application/font-woff":         true,
			"application/font-sfnt":         true,
			"application/vnd.ms-fontobject": true,
			"application/x-font-ttf":        true,
			"application/x-font-otf":        true,
			// Text and code
			"text/plain":             true,
			"text/csv":               true,
			"text/css":               true,
			"text/html":              true,
			"text/markdown":          true,
			"text/xml":               true,
			"text/javascript":        true,
			"application/json":       true,
			"application/xml":        true,
			"application/javascript": true,
			// Audio / video
			"audio/*":         true,
			"video/*":         true,
			"application/ogg": true,
		},
	}
}

func (c *Config) IsAllowedMime(mime string) bool {
	clean := strings.Split(mime, ";")[0]
	return c.AllowedMimeTypes[strings.TrimSpace(clean)]
}
