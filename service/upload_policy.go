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
	if allowed[mime] || allowed["*/*"] || allowed["*"] {
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
	if actor := domain.ActorFromContext(ctx); actor != nil && actor.Token && len(actor.Scope.UploadSettings) > 0 {
		policy.allowed = narrowAllowedMimeTypes(
			policy.allowed,
			uploadSettingMimePatterns(actor.Scope.UploadSettings),
		)
		policy.limits = narrowUploadLimits(policy.limits, actor.Scope.UploadSettings)
	}
	return policy
}

// uploadSettingMimePatterns expands token upload settings into MIME patterns.
// An empty format or "*" covers every library format.
func uploadSettingMimePatterns(settings []domain.UploadSetting) []string {
	patterns := make([]string, 0, len(settings))
	for _, setting := range settings {
		if len(setting.MimeTypes) > 0 {
			patterns = append(patterns, setting.MimeTypes...)
			continue
		}
		switch setting.Format {
		case "image":
			patterns = append(patterns, "image/*")
		case "audio":
			patterns = append(patterns, "audio/*")
		case "video":
			patterns = append(patterns, "video/*")
		case "pdf":
			patterns = append(patterns, "application/pdf")
		default:
			patterns = append(patterns, "*/*")
		}
	}
	return patterns
}

// narrowUploadLimits applies token size limits on top of the library limits.
// A token can only lower a limit; a zero token limit inherits the library.
func narrowUploadLimits(base domain.UploadLimits, settings []domain.UploadSetting) domain.UploadLimits {
	var token domain.UploadLimits
	for _, setting := range settings {
		if setting.MaxBytes <= 0 {
			continue
		}
		for _, category := range uploadSettingCategories(setting) {
			switch category {
			case "image":
				token.ImageBytes = lowerLimit(token.ImageBytes, setting.MaxBytes)
			case "audio":
				token.AudioBytes = lowerLimit(token.AudioBytes, setting.MaxBytes)
			case "video":
				token.VideoBytes = lowerLimit(token.VideoBytes, setting.MaxBytes)
			case "pdf":
				token.PDFBytes = lowerLimit(token.PDFBytes, setting.MaxBytes)
			}
		}
	}
	return domain.UploadLimits{
		ImageBytes: lowerLimit(base.ImageBytes, token.ImageBytes),
		AudioBytes: lowerLimit(base.AudioBytes, token.AudioBytes),
		VideoBytes: lowerLimit(base.VideoBytes, token.VideoBytes),
		PDFBytes:   lowerLimit(base.PDFBytes, token.PDFBytes),
	}
}

// uploadSettingCategories returns the upload categories a token setting
// applies to. Explicit MIME types take precedence over the format shorthand.
func uploadSettingCategories(setting domain.UploadSetting) []string {
	if len(setting.MimeTypes) > 0 {
		seen := make(map[string]bool, 4)
		categories := make([]string, 0, 4)
		for _, mimeType := range setting.MimeTypes {
			if mimeType == "*" || mimeType == "*/*" {
				return []string{"image", "audio", "video", "pdf"}
			}
			category := uploadCategory(mimeType)
			if category == "" || seen[category] {
				continue
			}
			seen[category] = true
			categories = append(categories, category)
		}
		if len(categories) > 0 {
			return categories
		}
	}
	switch setting.Format {
	case "image", "audio", "video", "pdf":
		return []string{setting.Format}
	}
	return []string{"image", "audio", "video", "pdf"}
}

// lowerLimit returns the stricter of two limits. Zero means unlimited.
func lowerLimit(base, token int64) int64 {
	if token <= 0 {
		return base
	}
	if base <= 0 || token < base {
		return token
	}
	return base
}

// narrowAllowedMimeTypes intersects the library allow-list with the token's
// allow-list. Patterns may be exact types or "type/*" wildcards, so a token for
// "image/*" keeps a library rule for "image/png" and vice versa.
func narrowAllowedMimeTypes(base map[string]bool, tokenPatterns []string) map[string]bool {
	allowed := make(map[string]bool, len(base))
	for basePattern := range base {
		for _, tokenPattern := range tokenPatterns {
			if mimePatternsIntersect(basePattern, tokenPattern) {
				allowed[basePattern] = true
				break
			}
		}
	}
	return allowed
}

func mimePatternsIntersect(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == "*" || a == "*/*" || b == "*" || b == "*/*" {
		return true
	}
	if a == b {
		return true
	}
	aWild := strings.HasSuffix(a, "/*")
	bWild := strings.HasSuffix(b, "/*")
	switch {
	case aWild && bWild:
		return strings.TrimSuffix(a, "/*") == strings.TrimSuffix(b, "/*")
	case aWild:
		return strings.HasPrefix(b, strings.TrimSuffix(a, "*"))
	case bWild:
		return strings.HasPrefix(a, strings.TrimSuffix(b, "*"))
	}
	return false
}

// UploadStreamLimit reports the largest accepted request body for this
// library, or 0 when uploads are unlimited. The API layer uses it to cap
// requests before their content type is known.
func (s *AssetService) UploadStreamLimit(ctx context.Context) int64 {
	return s.uploadPolicy(ctx).streamLimit()
}
