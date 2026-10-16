package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/vvb13a/godam/store"
)

// sniffMime returns the best-effort MIME type: a content sniff of the actual
// bytes when conclusive, otherwise the caller-provided type. Charset parameters
// are stripped so allow-list matching is stable.
func sniffMime(provided string, data []byte) string {
	if detected := mimetype.Detect(data); detected != nil {
		if d := strings.Split(detected.String(), ";")[0]; d != "" && d != "application/octet-stream" {
			return d
		}
	}
	return strings.Split(provided, ";")[0]
}

// mimeAllowed reports whether mime is permitted by an allow-set of exact types
// and "type/*" wildcards.
func mimeAllowed(mime string, allowed map[string]bool) bool {
	if allowed[mime] {
		return true
	}
	if i := strings.IndexByte(mime, '/'); i > 0 {
		return allowed[mime[:i]+"/*"]
	}
	return false
}

// uploadPolicy resolves the effective allowed MIME types and max upload size for
// a library, falling back to the global config defaults when unset.
func (s *AssetService) uploadPolicy(ctx context.Context, libraryID string) (map[string]bool, int64) {
	var allowed map[string]bool
	var maxBytes int64
	if s.cfg != nil {
		allowed = s.cfg.AllowedMimeTypes
		maxBytes = s.cfg.MaxUploadSizeBytes
	}
	var lib store.Library
	if err := s.db.WithContext(ctx).
		Select("allowed_mime_types", "max_upload_bytes").
		First(&lib, "id = ?", libraryID).Error; err == nil {
		if lib.AllowedMimeTypes != "" {
			var list []string
			if json.Unmarshal([]byte(lib.AllowedMimeTypes), &list) == nil && len(list) > 0 {
				set := make(map[string]bool, len(list))
				for _, m := range list {
					if m = strings.TrimSpace(m); m != "" {
						set[m] = true
					}
				}
				allowed = set
			}
		}
		if lib.MaxUploadBytes > 0 {
			maxBytes = lib.MaxUploadBytes
		}
	}
	return allowed, maxBytes
}
