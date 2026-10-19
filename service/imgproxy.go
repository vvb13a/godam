package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ImgProxy builds signed imgproxy URLs and fetches rendered images from it.
// Source images are handed to imgproxy through godam's internal source route,
// so imgproxy never needs direct access to the asset storage driver.
type ImgProxy struct {
	baseURL       string
	sourceBaseURL string
	key           []byte
	salt          []byte
	client        *http.Client
}

func NewImgProxy(baseURL, sourceBaseURL, keyHex, saltHex string) (*ImgProxy, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("imgproxy url is required")
	}
	sourceBaseURL = strings.TrimRight(strings.TrimSpace(sourceBaseURL), "/")
	if sourceBaseURL == "" {
		return nil, fmt.Errorf("imgproxy source base url is required")
	}

	p := &ImgProxy{
		baseURL:       baseURL,
		sourceBaseURL: sourceBaseURL,
		client:        &http.Client{Timeout: 30 * time.Second},
	}

	keyHex = strings.TrimSpace(keyHex)
	saltHex = strings.TrimSpace(saltHex)
	if keyHex != "" || saltHex != "" {
		if keyHex == "" || saltHex == "" {
			return nil, fmt.Errorf("imgproxy key and salt must be set together")
		}
		key, err := hex.DecodeString(keyHex)
		if err != nil {
			return nil, fmt.Errorf("decode imgproxy key: %w", err)
		}
		salt, err := hex.DecodeString(saltHex)
		if err != nil {
			return nil, fmt.Errorf("decode imgproxy salt: %w", err)
		}
		p.key = key
		p.salt = salt
	}

	return p, nil
}

// Signed reports whether the client was configured with a key/salt pair.
func (p *ImgProxy) Signed() bool {
	return len(p.key) > 0 && len(p.salt) > 0
}

// URL returns a signed imgproxy URL for one asset and processing pipeline.
// The returned URL includes the output extension, so imgproxy sets the right
// Content-Type even when no explicit format option is present.
func (p *ImgProxy) URL(assetID, processing, format string) string {
	source := p.sourceBaseURL + "/internal/source/" + url.PathEscape(assetID)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(source))
	path := "/" + processing + "/" + encoded + "." + format
	return p.baseURL + "/" + p.sign(path) + path
}

// sign implements imgproxy's URL signature: HMAC-SHA256 over salt+path using
// the key as the HMAC key, then URL-safe Base64 without padding.
func (p *ImgProxy) sign(path string) string {
	if !p.Signed() {
		return "unsafe"
	}
	mac := hmac.New(sha256.New, p.key)
	mac.Write(p.salt)
	mac.Write([]byte(path))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Get requests a URL from imgproxy. The caller owns the response body.
func (p *ImgProxy) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "godam")
	return p.client.Do(req)
}

// Drain closes a response body after discarding a small amount of it, so
// connection reuse is not blocked on error paths.
func Drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	_ = body.Close()
}
