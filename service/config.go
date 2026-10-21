package service

import "strings"

// Config holds the app-level settings that are not stored per asset: upload
// limits, allowed MIME types, where assets are written, and how conversions are
// produced and served.
type Config struct {
	MaxUploadSizeBytes int64
	AllowedMimeTypes   map[string]bool
	StoragePath        string

	// Domain is the public application domain (for example example.com).
	// AssetsDomain is the conversion subdomain (assets.example.com).
	Domain       string
	AssetsDomain string

	// ConversionDir is the local conversion cache root shared with Caddy.
	// ConversionDriver selects the storage implementation ("local" for now).
	ConversionDir    string
	ConversionDriver string

	// ImgProxyURL is the internal imgproxy base URL. InternalURL is how imgproxy
	// reaches godam's source route. Key and Salt are hex-encoded.
	ImgProxyURL  string
	ImgProxyKey  string
	ImgProxySalt string
	InternalURL  string
}

// DeriveAssetsDomain returns the assets subdomain for an application domain.
func DeriveAssetsDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return ""
	}
	if strings.HasPrefix(domain, "assets.") {
		return domain
	}
	return "assets." + domain
}

// AssetsBaseURL returns the public origin for conversions, or an empty string
// when the domain is not configured.
func (c *Config) AssetsBaseURL() string {
	if c.AssetsDomain == "" {
		return ""
	}
	return "https://" + c.AssetsDomain
}

func DefaultConfig() *Config {
	return &Config{
		MaxUploadSizeBytes: 100 * 1024 * 1024,
		AllowedMimeTypes: map[string]bool{
			// Images
			"image/jpeg": true,
			"image/png":  true,
			"image/webp": true,
			"image/gif":  true,
			// Documents
			"application/pdf": true,
			// Audio
			"audio/mpeg": true,
			"audio/wav":  true,
			"audio/mp4":  true,
			"audio/webm": true,
			// Video
			"video/mp4":       true,
			"video/webm":      true,
			"video/quicktime": true,
		},
	}
}

func (c *Config) IsAllowedMime(mime string) bool {
	clean := strings.Split(mime, ";")[0]
	return c.AllowedMimeTypes[strings.TrimSpace(clean)]
}
