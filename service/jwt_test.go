package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
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
			"filters": map[string]any{
				"collection": map[string]any{
					"values": []string{"collection-1"},
					"mode":   "locked",
				},
				"type": map[string]any{
					"values": []string{"image"},
					"mode":   "hidden",
				},
				"mime": map[string]any{
					"values": []string{"image/png", "image/webp"},
				},
				"search": map[string]any{"values": "brand"},
				"sort":   map[string]any{"values": "name-asc"},
				"uncategorized": map[string]any{
					"mode": "locked",
				},
			},
			"upload_settings": []map[string]any{
				{
					"format":     "image",
					"mime_types": []string{"image/png", "image/webp"},
					"max_bytes":  10485760,
				},
				{"format": "video"},
			},
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
	if grant, _ := scope.Filter("collection"); len(grant.Values) != 1 || grant.Values[0] != "collection-1" || grant.Mode != domain.ScopeModeLocked {
		t.Fatalf("collection = %+v", grant)
	}
	if grant, _ := scope.Filter("type"); !scope.Hidden("type") || grant.Mode != domain.ScopeModeHidden {
		t.Fatalf("type = %+v", grant)
	}
	if grant, _ := scope.Filter("mime"); len(grant.Values) != 2 || grant.Mode != domain.ScopeModeActive {
		t.Fatalf("mime = %+v", grant)
	}
	if grant, _ := scope.Filter("search"); len(grant.Values) != 1 || grant.Values[0] != "brand" {
		t.Fatalf("search = %+v", grant)
	}
	if grant, _ := scope.Filter("sort"); len(grant.Values) != 1 || grant.Values[0] != "name-asc" {
		t.Fatalf("sort = %+v", grant)
	}
	if grant, _ := scope.Filter("uncategorized"); len(grant.Values) != 1 || grant.Values[0] != "true" {
		t.Fatalf("uncategorized = %+v", grant)
	}
	if !scope.Locked("collection") {
		t.Fatalf("locked filters = %+v", scope.Filters)
	}
	if len(scope.UploadSettings) != 2 {
		t.Fatalf("upload settings = %+v", scope.UploadSettings)
	}
	if scope.UploadSettings[0].Format != "image" ||
		scope.UploadSettings[0].MaxBytes != 10485760 ||
		len(scope.UploadSettings[0].MimeTypes) != 2 {
		t.Fatalf("image upload setting = %+v", scope.UploadSettings[0])
	}
	if scope.UploadSettings[1].Format != "video" || scope.UploadSettings[1].MaxBytes != 0 {
		t.Fatalf("video upload setting = %+v", scope.UploadSettings[1])
	}
}

func TestJWTVerifierDefaultsAreReadOnly(t *testing.T) {
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
	if !actor.Can(domain.CapabilityAssetsRead) {
		t.Fatalf("default capabilities = %v", actor.Capabilities)
	}
	if actor.Can(domain.CapabilityAssetsUpload) {
		t.Fatal("tokens should not upload without assets:upload")
	}
	if len(actor.Scope.Filters) != 0 || len(actor.Scope.Attributes) != 0 {
		t.Fatalf("default scope should be empty: %+v", actor.Scope)
	}
}

func TestJWTVerifierMapsAttributesAndUploadValues(t *testing.T) {
	secret := "test-secret"
	token := signTestJWT(t, secret, map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"scope": map[string]any{
			"attributes": map[string]any{
				"tenant": map[string]any{
					"values":        []string{"acme", "beta"},
					"mode":          "hidden",
					"upload_values": "acme",
				},
				"entity": map[string]any{
					"values": []string{"news", "press"},
					"mode":   "locked",
				},
				"kind": map[string]any{
					"values":        []string{"logo", "icon"},
					"mode":          "active",
					"upload_values": "*",
				},
				"wildcard": map[string]any{
					"values": "*",
					"mode":   "active",
				},
			},
		},
	})
	actor, err := NewJWTVerifier(secret, "", "").Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	grants := actor.Scope.Attributes
	if len(grants) != 3 {
		t.Fatalf("grants = %+v", grants)
	}
	byKey := make(map[string]domain.AttributeGrant, len(grants))
	for _, grant := range grants {
		byKey[grant.Key] = grant
	}
	if byKey["tenant"].Mode != domain.AttributeModeHidden {
		t.Fatalf("tenant mode = %q", byKey["tenant"].Mode)
	}
	if byKey["entity"].Mode != domain.AttributeModeLocked {
		t.Fatalf("entity mode = %q", byKey["entity"].Mode)
	}
	if byKey["kind"].Mode != domain.AttributeModeActive {
		t.Fatalf("kind mode = %q", byKey["kind"].Mode)
	}
	if _, ok := byKey["wildcard"]; ok {
		t.Fatal("wildcard attribute should not add a filter grant")
	}

	upload := make(map[string][]string)
	for _, attr := range actor.Scope.UploadAttributes {
		upload[attr.Key] = append(upload[attr.Key], attr.Value)
	}
	if len(upload["tenant"]) != 1 || upload["tenant"][0] != "acme" {
		t.Fatalf("tenant upload = %v", upload["tenant"])
	}
	if _, ok := upload["entity"]; ok {
		t.Fatal("attributes without upload_values must not be stamped")
	}
	if len(upload["kind"]) != 2 {
		t.Fatalf("wildcard upload values should expand declared values: %v", upload["kind"])
	}
}

func TestJWTVerifierRejectsInvalidScope(t *testing.T) {
	tests := map[string]map[string]any{
		"invalid filter mode": {
			"filters": map[string]any{
				"collection": map[string]any{"values": []string{"x"}, "mode": "nope"},
			},
		},
		"invalid filter name": {
			"filters": map[string]any{
				"colour": map[string]any{"values": []string{"red"}},
			},
		},
		"invalid attribute mode": {
			"attributes": map[string]any{
				"tenant": map[string]any{"values": []string{"acme"}, "mode": "nope"},
			},
		},
		"invalid upload format": {
			"upload_settings": []map[string]any{{"format": "document"}},
		},
		"mismatched upload type": {
			"upload_settings": []map[string]any{{
				"format":     "image",
				"mime_types": []string{"application/pdf"},
			}},
		},
	}
	for name, scope := range tests {
		token := signTestJWT(t, "secret", map[string]any{
			"sub":   "user-1",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"scope": scope,
		})
		if _, err := NewJWTVerifier("secret", "", "").Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}

func TestJWTVerifierMapsCapabilities(t *testing.T) {
	token := signTestJWT(t, "secret", map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"scope": map[string]any{
			"capabilities": []string{"assets:*", "settings:upload"},
		},
	})
	actor, err := NewJWTVerifier("secret", "", "").Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !actor.Can(domain.CapabilityAssetsDelete) {
		t.Fatal("assets:* should allow assets:delete")
	}
	if !actor.Can(domain.CapabilitySettingsUpload) {
		t.Fatal("settings:upload should be present")
	}
	if actor.Can(domain.CapabilitySettingsStorage) {
		t.Fatal("settings:storage should not be present")
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

func TestJWTVerifierWildcardAttributes(t *testing.T) {
	for name, attributes := range map[string]any{
		"string wildcard": "*",
		"object wildcard": map[string]any{
			"*": map[string]any{"mode": "active"},
		},
	} {
		token := signTestJWT(t, "secret", map[string]any{
			"sub": "user-1",
			"exp": time.Now().Add(time.Hour).Unix(),
			"scope": map[string]any{
				"attributes": attributes,
			},
		})
		actor, err := NewJWTVerifier("secret", "", "").Verify(token)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !actor.Scope.AllAttributes {
			t.Fatalf("%s should enable all attributes", name)
		}
		if len(actor.Scope.Attributes) != 0 {
			t.Fatalf("%s added grants: %+v", name, actor.Scope.Attributes)
		}
	}
}

func TestJWTVerifierRejectsInvalidWildcardAttributes(t *testing.T) {
	tests := map[string]any{
		"locked": map[string]any{
			"*": map[string]any{"mode": "locked"},
		},
		"with values": map[string]any{
			"*": map[string]any{"mode": "active", "values": []string{"x"}},
		},
		"invalid string": "all",
	}
	for name, attributes := range tests {
		token := signTestJWT(t, "secret", map[string]any{
			"sub": "user-1",
			"exp": time.Now().Add(time.Hour).Unix(),
			"scope": map[string]any{
				"attributes": attributes,
			},
		})
		if _, err := NewJWTVerifier("secret", "", "").Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}

func TestJWTVerifierWildcardFiltersAndOverrides(t *testing.T) {
	token := signTestJWT(t, "secret", map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"scope": map[string]any{
			"filters": map[string]any{
				"*":    map[string]any{"mode": "allowed"},
				"type": map[string]any{"mode": "denied"},
				"sort": map[string]any{"mode": "active", "values": "name-asc"},
			},
		},
	})
	actor, err := NewJWTVerifier("secret", "", "").Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	scope := actor.Scope
	if !scope.AllFilters {
		t.Fatal("wildcard filters should be enabled")
	}
	if grant := scope.Filters["type"]; grant.Mode != domain.ScopeModeDenied {
		t.Fatalf("type override = %+v", grant)
	}
	if scope.FilterDenied("type") == false || scope.FilterVisible("type") {
		t.Fatal("denied type should be hidden and forbidden")
	}
	if !scope.FilterVisible("mime") || scope.FilterDenied("mime") {
		t.Fatal("wildcard should allow unlisted filters")
	}
	if grant := scope.Filters["sort"]; grant.Mode != domain.ScopeModeActive ||
		len(grant.Values) != 1 || grant.Values[0] != "name-asc" {
		t.Fatalf("sort grant = %+v", grant)
	}
}

func TestJWTVerifierStringWildcardFilters(t *testing.T) {
	token := signTestJWT(t, "secret", map[string]any{
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"scope": map[string]any{
			"filters": "*",
		},
	})
	actor, err := NewJWTVerifier("secret", "", "").Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !actor.Scope.AllFilters || !actor.Scope.FilterVisible("search") {
		t.Fatalf("wildcard filters = %+v", actor.Scope)
	}
}

func TestJWTVerifierRejectsInvalidFilterWildcards(t *testing.T) {
	tests := map[string]any{
		"wildcard locked": map[string]any{
			"*": map[string]any{"mode": "locked"},
		},
		"wildcard values": map[string]any{
			"*": map[string]any{"mode": "allowed", "values": []string{"x"}},
		},
		"allowed with values": map[string]any{
			"type": map[string]any{"mode": "allowed", "values": []string{"image"}},
		},
		"denied with values": map[string]any{
			"type": map[string]any{"mode": "denied", "values": []string{"image"}},
		},
		"invalid string": "deny-all",
	}
	for name, filters := range tests {
		token := signTestJWT(t, "secret", map[string]any{
			"sub": "user-1",
			"exp": time.Now().Add(time.Hour).Unix(),
			"scope": map[string]any{
				"filters": filters,
			},
		})
		if _, err := NewJWTVerifier("secret", "", "").Verify(token); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}
