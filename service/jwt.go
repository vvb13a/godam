package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

	scope, err := claims.Scope.toDomain(username)
	if err != nil {
		return nil, ErrInvalidToken
	}

	capabilities := domain.NormalizeCapabilities(claims.Scope.Capabilities)
	if len(capabilities) == 0 {
		capabilities = []string{domain.CapabilityAssetsRead}
	}

	return &domain.Actor{
		ID:           subject,
		Name:         username,
		Token:        true,
		Scope:        scope,
		Capabilities: capabilities,
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
	Username       string            `json:"username"`
	Filters        jwtFilters        `json:"filters"`
	Attributes     jwtAttributes     `json:"attributes"`
	UploadSettings jwtUploadSettings `json:"upload_settings"`
	Capabilities   stringList        `json:"capabilities"`
}

type jwtFilters struct {
	All   bool
	Items map[string]jwtFilter
}

// UnmarshalJSON accepts a map of filter grants, the wildcard string
// "filters": "*", or the object form with a "*" key.
func (f *jwtFilters) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var marker string
	if err := json.Unmarshal(data, &marker); err == nil {
		if strings.TrimSpace(marker) != "*" {
			return fmt.Errorf("invalid filters")
		}
		f.All = true
		return nil
	}
	var items map[string]jwtFilter
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}
	if star, ok := items["*"]; ok {
		mode := domain.ScopeModeAllowed
		if strings.TrimSpace(star.Mode) != "" {
			var err error
			mode, err = normalizeScopeMode(star.Mode)
			if err != nil {
				return err
			}
		}
		if mode != domain.ScopeModeAllowed || len(star.Values) > 0 {
			return fmt.Errorf("wildcard filters only support allowed mode without values")
		}
		f.All = true
		delete(items, "*")
	}
	f.Items = items
	return nil
}

type jwtFilter struct {
	Values stringList `json:"values"`
	Mode   string     `json:"mode"`
}

type jwtAttributes struct {
	All   bool
	Items map[string]jwtAttribute
}

// UnmarshalJSON accepts either a map of attribute grants or the wildcard forms
// "attributes": "*" and "attributes": {"*": {"mode":"active"}}.
func (a *jwtAttributes) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var marker string
	if err := json.Unmarshal(data, &marker); err == nil {
		if strings.TrimSpace(marker) != "*" {
			return fmt.Errorf("invalid attributes")
		}
		a.All = true
		return nil
	}
	var items map[string]jwtAttribute
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("invalid attributes: %w", err)
	}
	if star, ok := items["*"]; ok {
		mode, err := normalizeAttributeMode(star.Mode)
		if err != nil {
			return err
		}
		if mode != domain.AttributeModeActive || len(star.Values) > 0 || len(star.UploadValues) > 0 {
			return fmt.Errorf("wildcard attributes only support active mode without values")
		}
		a.All = true
		delete(items, "*")
	}
	a.Items = items
	return nil
}

type jwtAttribute struct {
	Values       stringList `json:"values"`
	Mode         string     `json:"mode"`
	UploadValues stringList `json:"upload_values"`
}

// UnmarshalJSON accepts either {values:[...],mode:"..."} or a bare string /
// array of strings (which defaults to active mode).
func (a *jwtAttribute) UnmarshalJSON(data []byte) error {
	var object struct {
		Values       stringList `json:"values"`
		Mode         string     `json:"mode"`
		UploadValues stringList `json:"upload_values"`
	}
	if err := json.Unmarshal(data, &object); err == nil {
		a.Values = object.Values
		a.Mode = object.Mode
		a.UploadValues = object.UploadValues
		return nil
	}
	var list stringList
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	a.Values = list
	return nil
}

type jwtUploadSetting struct {
	Format    string     `json:"format"`
	MimeTypes stringList `json:"mime_types"`
	Types     stringList `json:"types"`
	MaxBytes  int64      `json:"max_bytes"`
}

// jwtUploadSettings accepts either a single object or an array of objects.
type jwtUploadSettings []jwtUploadSetting

func (s *jwtUploadSettings) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	var many []jwtUploadSetting
	if err := json.Unmarshal(data, &many); err == nil {
		*s = many
		return nil
	}
	var one jwtUploadSetting
	if err := json.Unmarshal(data, &one); err != nil {
		return fmt.Errorf("expected upload setting object or array: %w", err)
	}
	*s = []jwtUploadSetting{one}
	return nil
}

// toDomain converts the token scope into the server-side authorization
// envelope. It rejects unknown filter names, modes and upload formats.
func (s jwtScope) toDomain(username string) (domain.Scope, error) {
	scope := domain.Scope{
		Username:       username,
		Filters:        make(map[string]domain.FilterGrant, len(s.Filters.Items)),
		AllFilters:     s.Filters.All,
		AllAttributes:  s.Attributes.All,
		UploadSettings: make([]domain.UploadSetting, 0, len(s.UploadSettings)),
	}

	for rawName, filter := range s.Filters.Items {
		name := normalizeFilterName(rawName)
		if name == "" {
			return domain.Scope{}, fmt.Errorf("invalid filter %q", rawName)
		}
		if _, exists := scope.Filters[name]; exists {
			return domain.Scope{}, fmt.Errorf("duplicate filter %q", name)
		}
		mode, err := normalizeScopeMode(filter.Mode)
		if err != nil {
			return domain.Scope{}, err
		}
		values := normalizeAttributeValues(filter.Values)
		if name == "type" || name == "mime" {
			values = normalizeMimeTypes(values)
		}
		if hasWildcard(values) {
			values = nil
		}
		if (mode == domain.ScopeModeAllowed || mode == domain.ScopeModeDenied) && len(values) > 0 {
			return domain.Scope{}, fmt.Errorf("mode %q does not accept values", mode)
		}
		scope.Filters[name] = domain.FilterGrant{Values: values, Mode: mode}
	}

	uploadKeys := make(map[string]bool)
	for rawKey, attr := range s.Attributes.Items {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return domain.Scope{}, fmt.Errorf("invalid attribute key")
		}
		mode, err := normalizeAttributeMode(attr.Mode)
		if err != nil {
			return domain.Scope{}, err
		}
		values := normalizeAttributeValues(attr.Values)
		wildcard := hasWildcard(values)
		if wildcard {
			values = nil
		}
		if len(values) > 0 {
			scope.Attributes = append(scope.Attributes, domain.AttributeGrant{
				Key:    key,
				Values: values,
				Mode:   mode,
			})
		}
		uploadValues := normalizeAttributeValues(attr.UploadValues)
		if hasWildcard(uploadValues) {
			uploadValues = values
		}
		for _, value := range uploadValues {
			token := key + "\x00" + value
			if uploadKeys[token] {
				continue
			}
			uploadKeys[token] = true
			scope.UploadAttributes = append(scope.UploadAttributes, domain.AssetAttribute{
				Key:   key,
				Value: value,
			})
		}
	}
	sort.Slice(scope.Attributes, func(i, j int) bool {
		return scope.Attributes[i].Key < scope.Attributes[j].Key
	})

	for _, setting := range s.UploadSettings {
		normalized, err := normalizeUploadSetting(setting)
		if err != nil {
			return domain.Scope{}, err
		}
		scope.UploadSettings = append(scope.UploadSettings, normalized)
	}

	return scope, nil
}

func normalizeScopeMode(raw string) (domain.ScopeMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "active":
		return domain.ScopeModeActive, nil
	case "allowed":
		return domain.ScopeModeAllowed, nil
	case "locked":
		return domain.ScopeModeLocked, nil
	case "hidden":
		return domain.ScopeModeHidden, nil
	case "denied":
		return domain.ScopeModeDenied, nil
	}
	return "", fmt.Errorf("invalid scope mode %q", raw)
}

func normalizeAttributeMode(raw string) (domain.AttributeMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "active":
		return domain.AttributeModeActive, nil
	case "locked":
		return domain.AttributeModeLocked, nil
	case "hidden":
		return domain.AttributeModeHidden, nil
	}
	return "", fmt.Errorf("invalid attribute mode %q", raw)
}

func normalizeUploadSetting(setting jwtUploadSetting) (domain.UploadSetting, error) {
	format := strings.ToLower(strings.TrimSpace(setting.Format))
	if format == "" {
		format = "*"
	}
	switch format {
	case "*", "image", "audio", "video", "pdf":
	default:
		return domain.UploadSetting{}, fmt.Errorf("invalid upload format %q", setting.Format)
	}
	if setting.MaxBytes < 0 {
		return domain.UploadSetting{}, fmt.Errorf("invalid upload size")
	}
	mimeTypes := append(append(stringList{}, setting.MimeTypes...), setting.Types...)
	normalizedTypes := normalizeMimeTypes(mimeTypes)
	for _, mimeType := range normalizedTypes {
		if format == "*" || mimeType == "*" || mimeType == "*/*" {
			continue
		}
		if uploadFormatForMimeType(mimeType) != format {
			return domain.UploadSetting{}, fmt.Errorf(
				"upload type %q does not match format %q", mimeType, format,
			)
		}
	}
	return domain.UploadSetting{
		Format:    format,
		MimeTypes: normalizedTypes,
		MaxBytes:  setting.MaxBytes,
	}, nil
}

func uploadFormatForMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case mimeType == "application/pdf":
		return "pdf"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	}
	return ""
}

func hasWildcard(values []string) bool {
	for _, value := range values {
		if value == "*" || value == "*/*" {
			return true
		}
	}
	return false
}

func normalizeFilterName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
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

func normalizeAttributeValues(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
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
