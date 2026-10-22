package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vvb13a/godam/domain"
)

// ErrInvalidToken is returned for malformed, unsigned, expired or otherwise
// unacceptable JWTs. Callers should translate it to a 401 without leaking why.
var ErrInvalidToken = errors.New("invalid token")

// JWTVerifier validates HS256 JWT embed tokens. A verifier with an empty secret
// is disabled (Enabled returns false) so deployments that only use sessions are
// unaffected.
type JWTVerifier struct {
	secret   []byte
	issuer   string
	audience string
	now      func() time.Time
}

// NewJWTVerifier builds a verifier from the deployment settings. issuer and
// audience are optional; when set they are enforced.
func NewJWTVerifier(secret, issuer, audience string) *JWTVerifier {
	return &JWTVerifier{
		secret:   []byte(strings.TrimSpace(secret)),
		issuer:   strings.TrimSpace(issuer),
		audience: strings.TrimSpace(audience),
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Enabled reports whether JWT authentication is configured.
func (v *JWTVerifier) Enabled() bool {
	return v != nil && len(v.secret) > 0
}

// Verify checks the signature and standard time claims, then maps the token's
// scope into the request actor. Only HS256 is accepted.
func (v *JWTVerifier) Verify(token string) (*domain.Actor, error) {
	if !v.Enabled() {
		return nil, ErrInvalidToken
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "HS256" {
		return nil, ErrInvalidToken
	}

	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(signature, expected) != 1 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	now := v.now()
	if claims.ExpiresAt == 0 || now.Unix() >= claims.ExpiresAt {
		return nil, ErrInvalidToken
	}
	if claims.NotBefore != 0 && now.Unix() < claims.NotBefore {
		return nil, ErrInvalidToken
	}
	if v.issuer != "" && claims.Issuer != v.issuer {
		return nil, ErrInvalidToken
	}
	if v.audience != "" && !containsString(claims.Audience, v.audience) {
		return nil, ErrInvalidToken
	}

	username := firstNonEmpty(claims.Username, claims.Scope.Username)
	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		subject = username
	}
	if username == "" {
		username = subject
	}
	if subject == "" || username == "" {
		return nil, ErrInvalidToken
	}

	return &domain.Actor{
		ID:    subject,
		Name:  username,
		Token: true,
		Scope: claims.Scope.toDomain(username),
	}, nil
}

type jwtClaims struct {
	Subject   string     `json:"sub"`
	Username  string     `json:"username"`
	ExpiresAt int64      `json:"exp"`
	NotBefore int64      `json:"nbf"`
	IssuedAt  int64      `json:"iat"`
	Issuer    string     `json:"iss"`
	Audience  stringList `json:"aud"`
	Scope     jwtScope   `json:"scope"`
}

type jwtScope struct {
	Username         string          `json:"username"`
	DefaultFilters   jwtScopeFilters `json:"default_filters"`
	LockedFilters    stringList      `json:"locked_filters"`
	AllowedMimeTypes stringList      `json:"allowed_mime_types"`
	CanUpload        *bool           `json:"can_upload"`
}

type jwtScopeFilters struct {
	Collection    stringList `json:"collection"`
	CollectionIDs stringList `json:"collection_ids"`
	Tag           stringList `json:"tag"`
	TagIDs        stringList `json:"tag_ids"`
	Type          stringList `json:"type"`
	Types         stringList `json:"types"`
	Mime          stringList `json:"mime"`
	MimeTypes     stringList `json:"mime_types"`
	Uncategorized bool       `json:"uncategorized"`
	Untagged      bool       `json:"untagged"`
	Search        string     `json:"search"`
	Sort          string     `json:"sort"`
}

func (s jwtScope) toDomain(username string) domain.Scope {
	collections := firstList(s.DefaultFilters.Collection, s.DefaultFilters.CollectionIDs)
	tags := firstList(s.DefaultFilters.Tag, s.DefaultFilters.TagIDs)
	types := firstList(s.DefaultFilters.Type, s.DefaultFilters.Types)
	mimes := firstList(s.DefaultFilters.Mime, s.DefaultFilters.MimeTypes)

	collections, uncategorized := splitSpecial(collections, "uncategorized")
	tags, untagged := splitSpecial(tags, "untagged")

	canUpload := true
	if s.CanUpload != nil {
		canUpload = *s.CanUpload
	}

	return domain.Scope{
		Username: username,
		DefaultFilters: domain.ScopeFilters{
			CollectionIDs: collections,
			TagIDs:        tags,
			Types:         types,
			MimeTypes:     mimes,
			Uncategorized: s.DefaultFilters.Uncategorized || uncategorized,
			Untagged:      s.DefaultFilters.Untagged || untagged,
			Search:        strings.TrimSpace(s.DefaultFilters.Search),
			Sort:          strings.TrimSpace(s.DefaultFilters.Sort),
		},
		LockedFilters:    normalizeLockedFilters(s.LockedFilters),
		AllowedMimeTypes: normalizeMimeTypes(s.AllowedMimeTypes),
		CanUpload:        canUpload,
	}
}

func normalizeLockedFilters(values []string) []string {
	seen := make(map[string]bool, len(values))
	locked := make([]string, 0, len(values))
	for _, value := range values {
		switch normalizeFilterName(value) {
		case "collection", "tag", "type", "mime", "search", "sort":
			name := normalizeFilterName(value)
			if !seen[name] {
				seen[name] = true
				locked = append(locked, name)
			}
		}
	}
	return locked
}

func normalizeFilterName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "collection", "collections", "collection_id", "collection_ids":
		return "collection"
	case "tag", "tags", "tag_id", "tag_ids":
		return "tag"
	case "type", "types":
		return "type"
	case "mime", "mimes", "mime_type", "mime_types":
		return "mime"
	case "search", "query", "q":
		return "search"
	case "sort", "order", "sorting":
		return "sort"
	}
	return ""
}

func normalizeMimeTypes(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func splitSpecial(values []string, special string) ([]string, bool) {
	kept := make([]string, 0, len(values))
	found := false
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), special) {
			found = true
			continue
		}
		kept = append(kept, strings.TrimSpace(value))
	}
	return kept, found
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstList(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			out := make([]string, 0, len(value))
			for _, item := range value {
				if item = strings.TrimSpace(item); item != "" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// stringList accepts a JSON string or array of strings, which makes tokens
// pleasant to write in PHP/Laravel while remaining strict about their type.
type stringList []string

func (l *stringList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*l = nil
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			*l = nil
		} else {
			*l = []string{single}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return fmt.Errorf("expected string or array of strings: %w", err)
	}
	*l = many
	return nil
}
