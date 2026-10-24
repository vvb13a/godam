package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signAPITestJWT(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedHeader + "." + encodedPayload))
	return encodedHeader + "." + encodedPayload + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestJWTAuthMeReturnsScope(t *testing.T) {
	server := httptest.NewServer(NewServer(Deps{JWTSecret: "test-secret"}))
	defer server.Close()

	token := signAPITestJWT(t, "test-secret", map[string]any{
		"sub":      "cms-1",
		"username": "Grace Hopper",
		"exp":      time.Now().Add(time.Hour).Unix(),
		"scope": map[string]any{
			"default_filters":    map[string]any{"type": "image"},
			"locked_filters":     []string{"type"},
			"allowed_mime_types": []string{"image/*"},
		},
	})

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var body struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Scope struct {
			DefaultFilters struct {
				Types []string `json:"type"`
			} `json:"default_filters"`
			LockedFilters    []string `json:"locked_filters"`
			AllowedMimeTypes []string `json:"allowed_mime_types"`
			CanUpload        bool     `json:"can_upload"`
		} `json:"scope"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "cms-1" || body.Name != "Grace Hopper" {
		t.Fatalf("unexpected user: %+v", body)
	}
	if len(body.Scope.DefaultFilters.Types) != 1 || body.Scope.DefaultFilters.Types[0] != "image" {
		t.Fatalf("scope defaults missing: %+v", body.Scope)
	}
	if !body.Scope.CanUpload {
		t.Fatal("can_upload should default to true")
	}

	req, _ = http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	unauthorized, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid token status = %d", unauthorized.StatusCode)
	}
}
