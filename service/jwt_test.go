package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func signTestJWT(t *testing.T, secret string, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedHeader + "." + encodedPayload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedHeader + "." + encodedPayload + "." + signature
}

func TestJWTVerifierMapsScope(t *testing.T) {
	secret := "test-secret"
	now := time.Now().UTC()
	token := signTestJWT(t, secret, map[string]any{
		"sub":      "cms-user-42",
		"username": "Ada Lovelace",
		"exp":      now.Add(time.Hour).Unix(),
		"scope": map[string]any{
			"default_filters": map[string]any{
				"collection": "collection-1",
				"tag":        []string{"tag-1", "untagged"},
				"type":       "image",
				"mime":       []string{"image/png", "image/webp"},
				"search":     "brand",
				"sort":       "name-asc",
			},
			"locked_filters":     []string{"collection_ids", "tag", "types", "mime", "search", "sort"},
			"allowed_mime_types": []string{"IMAGE/*"},
			"can_upload":         false,
		},
	})

	verifier := NewJWTVerifier(secret, "", "")
	actor, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if actor.ID != "cms-user-42" || actor.Name != "Ada Lovelace" || !actor.Token {
		t.Fatalf("unexpected actor: %+v", actor)
	}
	scope := actor.Scope
	if len(scope.DefaultFilters.CollectionIDs) != 1 || scope.DefaultFilters.CollectionIDs[0] != "collection-1" {
		t.Fatalf("collections = %v", scope.DefaultFilters.CollectionIDs)
	}
	if len(scope.DefaultFilters.TagIDs) != 1 || scope.DefaultFilters.TagIDs[0] != "tag-1" || !scope.DefaultFilters.Untagged {
		t.Fatalf("tags = %v untagged=%v", scope.DefaultFilters.TagIDs, scope.DefaultFilters.Untagged)
	}
	if len(scope.DefaultFilters.Types) != 1 || scope.DefaultFilters.Types[0] != "image" {
		t.Fatalf("types = %v", scope.DefaultFilters.Types)
	}
	if scope.DefaultFilters.Search != "brand" || scope.DefaultFilters.Sort != "name-asc" {
		t.Fatalf("defaults = %+v", scope.DefaultFilters)
	}
	if len(scope.AllowedMimeTypes) != 1 || scope.AllowedMimeTypes[0] != "image/*" {
		t.Fatalf("allowed mime = %v", scope.AllowedMimeTypes)
	}
	if scope.CanUpload {
		t.Fatal("can_upload=false should disable uploads")
	}
	for _, name := range []string{"collection", "tag", "type", "mime", "search", "sort"} {
		if !scope.Locked(name) {
			t.Fatalf("filter %q should be locked", name)
		}
	}
}

func TestJWTVerifierDefaultsCanUpload(t *testing.T) {
	secret := "test-secret"
	token := signTestJWT(t, secret, map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	actor, err := NewJWTVerifier(secret, "", "").Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if actor.Name != "user-1" {
		t.Fatalf("username should fall back to subject, got %q", actor.Name)
	}
	if !actor.Scope.CanUpload {
		t.Fatal("can_upload should default to true")
	}
}

func TestJWTVerifierRejectsExpiredAndForgedTokens(t *testing.T) {
	secret := "test-secret"
	now := time.Now().UTC()
	expired := signTestJWT(t, secret, map[string]any{
		"sub": "user-1",
		"exp": now.Add(-time.Minute).Unix(),
	})
	valid := signTestJWT(t, secret, map[string]any{
		"sub": "user-1",
		"exp": now.Add(time.Hour).Unix(),
	})
	for name, token := range map[string]string{
		"expired":      expired,
		"wrong secret": signTestJWT(t, "other-secret", map[string]any{"sub": "user-1", "exp": now.Add(time.Hour).Unix()}),
		"missing exp":  signTestJWT(t, secret, map[string]any{"sub": "user-1"}),
		"garbage":      "not-a-token",
	} {
		if _, err := NewJWTVerifier(secret, "", "").Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%s should be rejected, got %v", name, err)
		}
	}
	if _, err := NewJWTVerifier(secret, "", "").Verify(valid); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func TestJWTVerifierEnforcesIssuerAndAudience(t *testing.T) {
	secret := "test-secret"
	now := time.Now().UTC()
	token := signTestJWT(t, secret, map[string]any{
		"sub": "user-1",
		"exp": now.Add(time.Hour).Unix(),
		"iss": "https://cms.example.test",
		"aud": "godam",
	})

	verifier := NewJWTVerifier(secret, "https://cms.example.test", "godam")
	if _, err := verifier.Verify(token); err != nil {
		t.Fatalf("matching issuer/audience rejected: %v", err)
	}
	if _, err := NewJWTVerifier(secret, "https://other.test", "godam").Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong issuer should be rejected, got %v", err)
	}
	if _, err := NewJWTVerifier(secret, "https://cms.example.test", "other").Verify(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong audience should be rejected, got %v", err)
	}
}

func TestNarrowAllowedMimeTypes(t *testing.T) {
	base := map[string]bool{
		"image/png":       true,
		"image/jpeg":      true,
		"application/pdf": true,
	}
	got := narrowAllowedMimeTypes(base, []string{"image/*"})
	if !got["image/png"] || !got["image/jpeg"] || got["application/pdf"] {
		t.Fatalf("unexpected narrowing: %+v", got)
	}
	got = narrowAllowedMimeTypes(map[string]bool{"image/*": true}, []string{"image/webp"})
	if !got["image/*"] {
		t.Fatalf("wildcard library rule should survive exact token rule: %+v", got)
	}
}
