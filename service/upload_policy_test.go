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
