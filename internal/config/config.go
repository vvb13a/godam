package config

import "strings"

type Config struct {
	MaxUploadSizeBytes int64
	AllowedMimeTypes   map[string]bool
	StoragePath        string
	DatabaseDSN        string
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
		StoragePath: "./data/storage",
		DatabaseDSN: "./data/dam.db",
	}
}

func (c *Config) IsAllowedMime(mime string) bool {
	clean := strings.Split(mime, ";")[0]
	return c.AllowedMimeTypes[strings.TrimSpace(clean)]
}
