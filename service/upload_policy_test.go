package service

import (
	"testing"

	"github.com/vvb13a/godam/domain"
)

func TestUploadPolicyLimits(t *testing.T) {
	policy := uploadPolicy{
		limits: domain.UploadLimits{
			ImageBytes: 5,
			VideoBytes: 20,
		},
	}

	cases := []struct {
		mime string
		want int64
	}{
		{"image/png", 5},
		{"audio/mpeg", 0},
		{"video/mp4", 20},
		{"application/pdf", 0},
		{"application/octet-stream", 0},
	}
	for _, c := range cases {
		if got := policy.limitFor(c.mime); got != c.want {
			t.Errorf("limitFor(%q) = %d, want %d", c.mime, got, c.want)
		}
	}

	if got := policy.streamLimit(); got != 20 {
		t.Errorf("streamLimit() = %d, want 20", got)
	}
}

func TestUploadSettingsNarrowMimeTypes(t *testing.T) {
	base := map[string]bool{
		"image/png":       true,
		"image/webp":      true,
		"application/pdf": true,
		"video/mp4":       true,
	}
	settings := []domain.UploadSetting{
		{Format: "image", MimeTypes: []string{"image/png"}},
		{Format: "video"},
	}
	got := narrowAllowedMimeTypes(base, uploadSettingMimePatterns(settings))
	if !got["image/png"] || !got["video/mp4"] || got["image/webp"] || got["application/pdf"] {
		t.Fatalf("unexpected narrowing: %+v", got)
	}

	all := narrowAllowedMimeTypes(base, uploadSettingMimePatterns([]domain.UploadSetting{{Format: "*"}}))
	if len(all) != len(base) {
		t.Fatalf("wildcard format should allow all library types: %+v", all)
	}
}

func TestUploadSettingsNarrowLimits(t *testing.T) {
	base := domain.UploadLimits{
		ImageBytes: 100,
		AudioBytes: 100,
		VideoBytes: 100,
		PDFBytes:   100,
	}
	got := narrowUploadLimits(base, []domain.UploadSetting{
		{Format: "image", MaxBytes: 50},
		{Format: "video", MaxBytes: 200},
	})
	if got.ImageBytes != 50 {
		t.Fatalf("image limit = %d, want 50", got.ImageBytes)
	}
	if got.VideoBytes != 100 {
		t.Fatalf("token should not raise the video limit: %d", got.VideoBytes)
	}
	if got.AudioBytes != 100 || got.PDFBytes != 100 {
		t.Fatalf("unrelated limits changed: %+v", got)
	}

	all := narrowUploadLimits(base, []domain.UploadSetting{{Format: "*", MaxBytes: 10}})
	if all.ImageBytes != 10 || all.AudioBytes != 10 || all.VideoBytes != 10 || all.PDFBytes != 10 {
		t.Fatalf("wildcard limit not applied: %+v", all)
	}

	typed := narrowUploadLimits(base, []domain.UploadSetting{{
		Format:    "*",
		MimeTypes: []string{"image/png"},
		MaxBytes:  10,
	}})
	if typed.ImageBytes != 10 || typed.AudioBytes != 100 || typed.VideoBytes != 100 || typed.PDFBytes != 100 {
		t.Fatalf("typed wildcard limit applied to the wrong categories: %+v", typed)
	}
}

func TestUploadCategory(t *testing.T) {
	cases := map[string]string{
		"image/webp":       "image",
		"audio/webm":       "audio",
		"video/quicktime":  "video",
		"application/pdf":  "pdf",
		"text/plain":       "",
		"application/json": "",
	}
	for mime, want := range cases {
		if got := uploadCategory(mime); got != want {
			t.Errorf("uploadCategory(%q) = %q, want %q", mime, got, want)
		}
	}
}
