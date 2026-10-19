package service

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestImgProxySignature checks the signing algorithm against the worked
// example in imgproxy's documentation.
func TestImgProxySignature(t *testing.T) {
	p := &ImgProxy{key: []byte("secret"), salt: []byte("hello")}
	path := "/rs:fill:300:400:0/g:sm/aHR0cDovL2V4YW1w/bGUuY29tL2ltYWdl/cy9jdXJpb3NpdHku/anBn.png"
	const want = "oKfUtW34Dvo2BGQehJFR4Nr0_rIjOtdtzJ3QFsUcXH8"
	if got := p.sign(path); got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}

func TestImgProxyURLUsesInternalSource(t *testing.T) {
	p, err := NewImgProxy("http://imgproxy:8080/", "http://godam:8080/", "736563726574", "68656c6c6f")
	if err != nil {
		t.Fatalf("new imgproxy: %v", err)
	}

	raw := p.URL("asset-1", "rs:fit:100:200/format:webp", "webp")
	if !strings.HasPrefix(raw, "http://imgproxy:8080/") {
		t.Fatalf("unexpected base: %s", raw)
	}
	parts := strings.Split(strings.TrimPrefix(raw, "http://imgproxy:8080/"), "/")
	if len(parts) != 4 {
		t.Fatalf("unexpected path parts: %#v", parts)
	}
	if parts[1]+"/"+parts[2] != "rs:fit:100:200/format:webp" {
		t.Fatalf("processing = %q", parts[1]+"/"+parts[2])
	}
	encoded := strings.TrimSuffix(parts[3], ".webp")
	source, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode source: %v", err)
	}
	if string(source) != "http://godam:8080/internal/source/asset-1" {
		t.Fatalf("source = %q", source)
	}
}

func TestImgProxyUnsigned(t *testing.T) {
	p, err := NewImgProxy("http://imgproxy:8080", "http://godam:8080", "", "")
	if err != nil {
		t.Fatalf("new imgproxy: %v", err)
	}
	if p.Signed() {
		t.Fatal("expected unsigned client")
	}
	if !strings.HasPrefix(p.URL("asset-1", "format:png", "png"), "http://imgproxy:8080/unsafe/") {
		t.Fatal("unsigned URL should use the unsafe signature")
	}
}

func TestParseConversionSpec(t *testing.T) {
	tests := []struct {
		raw     string
		width   int
		height  int
		format  string
		wantErr bool
	}{
		{raw: "800x600.webp", width: 800, height: 600, format: "webp"},
		{raw: "0x0.png", format: "png"},
		{raw: "400x0.jpeg", width: 400, format: "jpg"},
		{raw: "800x600", wantErr: true},
		{raw: "800.webp", wantErr: true},
		{raw: "800x600.tiff", wantErr: true},
		{raw: "-1x600.png", wantErr: true},
		{raw: "5000x600.png", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			spec, err := ParseConversionSpec(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.Width != tc.width || spec.Height != tc.height || spec.Format != tc.format {
				t.Fatalf("got %+v", spec)
			}
		})
	}
}
