package api

import "testing"

func TestIsPublicPath(t *testing.T) {
	tests := map[string]bool{
		"/api/health":                   true,
		"/convert/asset-1/800x600.webp": true,
		"/internal/source/asset-1":      true,
		"/api/assets":                   false,
		"/api/assets/asset-1/content":   false,
	}
	for path, want := range tests {
		if got := isPublicPath(path); got != want {
			t.Errorf("isPublicPath(%q) = %v, want %v", path, got, want)
		}
	}
}
