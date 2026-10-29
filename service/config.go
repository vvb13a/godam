package service

import (
	"strings"
	"time"
)

// Config holds the app-level settings that are not stored per asset: upload
// limits, allowed MIME types, where assets are written, and how conversions are
// produced and served.
type Config struct {
	AllowedMimeTypes map[string]bool
	StoragePath      string

	// Domain is the public application domain (for example example.com).
	// DeliveryDomain is the optional public origin for /img and /src. It may
	// equal Domain or be a separate hostname (for example assets.example.com).
	Domain         string
	DeliveryDomain string

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

	// DeliveryCacheTTL is how long public and private delivery responses may be
	// cached. It applies to /img, /src, and the authenticated preview/content
	// aliases.
	DeliveryCacheTTL time.Duration
}

// DeliveryBaseURL returns the public delivery origin, or an empty string when
// no delivery domain is configured.
func (c *Config) DeliveryBaseURL() string {
	domain := strings.TrimSpace(c.DeliveryDomain)
	if domain == "" {
		domain = strings.TrimSpace(c.Domain)
	}
	if domain == "" {
		return ""
	}
	return "https://" + domain
}

// DeliveryURL joins a path to the public delivery origin.
func (c *Config) DeliveryURL(path string) string {
	base := c.DeliveryBaseURL()
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func DefaultConfig() *Config {
	return &Config{
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
		DeliveryCacheTTL: 24 * time.Hour,
	}
}

func (c *Config) IsAllowedMime(mime string) bool {
	clean := strings.Split(mime, ";")[0]
	return c.AllowedMimeTypes[strings.TrimSpace(clean)]
}
