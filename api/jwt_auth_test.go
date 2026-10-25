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

func TestJWTAuthMeReturnsPublicScope(t *testing.T) {
	server := httptest.NewServer(NewServer(Deps{JWTSecret: "test-secret"}))
	defer server.Close()

	token := signAPITestJWT(t, "test-secret", map[string]any{
		"sub":      "cms-1",
		"username": "Grace Hopper",
		"exp":      time.Now().Add(time.Hour).Unix(),
		"scope": map[string]any{
			"filters": map[string]any{
				"type":   map[string]any{"values": "image", "mode": "locked"},
				"search": map[string]any{"values": "secret", "mode": "hidden"},
			},
			"upload_settings": []map[string]any{
				{"format": "image", "max_bytes": 1048576},
			},
			"capabilities": []string{"assets:read", "assets:upload"},
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
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Capabilities []string `json:"capabilities"`
		Scope        struct {
			Filters map[string]struct {
				Values []string `json:"values"`
				Mode   string   `json:"mode"`
			} `json:"filters"`
			UploadSettings []struct {
				Format   string `json:"format"`
				MaxBytes int64  `json:"max_bytes"`
			} `json:"upload_settings"`
		} `json:"scope"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "cms-1" || body.Name != "Grace Hopper" {
		t.Fatalf("unexpected user: %+v", body)
	}
	if len(body.Scope.Filters) != 2 || body.Scope.Filters["type"].Mode != "locked" {
		t.Fatalf("visible filters = %+v", body.Scope.Filters)
	}
	if search, ok := body.Scope.Filters["search"]; !ok || search.Mode != "hidden" || len(search.Values) != 0 {
		t.Fatalf("hidden filter leaked values: %+v", search)
	}
	if len(body.Scope.UploadSettings) != 1 || body.Scope.UploadSettings[0].MaxBytes != 1048576 {
		t.Fatalf("upload settings = %+v", body.Scope.UploadSettings)
	}
	if len(body.Capabilities) != 2 {
		t.Fatalf("capabilities = %v", body.Capabilities)
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
