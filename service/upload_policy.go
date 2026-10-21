package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/vvb13a/godam/domain"
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

// uploadPolicy is the effective upload configuration for the instance.
type uploadPolicy struct {
	allowed map[string]bool
	limits  domain.UploadLimits
}

// limitFor returns the byte limit for a MIME type's category. Zero means the
// category has no size limit.
func (p uploadPolicy) limitFor(mimeType string) int64 {
	switch uploadCategory(mimeType) {
	case "image":
		return p.limits.ImageBytes
	case "audio":
		return p.limits.AudioBytes
	case "video":
		return p.limits.VideoBytes
	case "pdf":
		return p.limits.PDFBytes
	}
	return 0
}

// streamLimit is the largest possible limit across categories. Request bodies
// are capped with it before their content type is known; the category-specific
// limit is enforced once the type is sniffed.
func (p uploadPolicy) streamLimit() int64 {
	var limit int64
	for _, candidate := range []int64{
		p.limits.ImageBytes,
		p.limits.AudioBytes,
		p.limits.VideoBytes,
		p.limits.PDFBytes,
	} {
		if candidate > limit {
			limit = candidate
		}
	}
	return limit
}

func uploadCategory(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case mimeType == "application/pdf":
		return "pdf"
	}
	return ""
}

// uploadPolicy resolves the effective allowed MIME types and size limits for
// the instance, falling back to the global config defaults when unset.
func (s *AssetService) uploadPolicy(ctx context.Context) uploadPolicy {
	policy := uploadPolicy{}
	if s.cfg != nil {
		policy.allowed = s.cfg.AllowedMimeTypes
	}
	var inst store.Instance
	if err := s.db.WithContext(ctx).
		Select("allowed_mime_types", "upload_limits").
		First(&inst).Error; err == nil {
		if inst.AllowedMimeTypes != "" {
			var list []string
			if json.Unmarshal([]byte(inst.AllowedMimeTypes), &list) == nil && len(list) > 0 {
				set := make(map[string]bool, len(list))
				for _, m := range list {
					if m = strings.TrimSpace(m); m != "" {
						set[m] = true
					}
				}
				policy.allowed = set
			}
		}
		if inst.UploadLimits != "" {
			var limits domain.UploadLimits
			if json.Unmarshal([]byte(inst.UploadLimits), &limits) == nil {
				policy.limits = limits
			}
		}
	}
	return policy
}

// UploadStreamLimit reports the largest accepted request body for this
// library, or 0 when uploads are unlimited. The API layer uses it to cap
// requests before their content type is known.
func (s *AssetService) UploadStreamLimit(ctx context.Context) int64 {
	return s.uploadPolicy(ctx).streamLimit()
}
