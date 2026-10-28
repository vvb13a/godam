package domain

import "context"

// ScopeMode controls whether a token-scoped filter or attribute is a default
// value, an enforced value, or an enforced value hidden from the embed UI.
type ScopeMode string

const (
	// ScopeModeAllowed makes the filter available with no default.
	ScopeModeAllowed ScopeMode = "allowed"
	// ScopeModeActive applies the values as the initial filter state. The client
	// may change or clear them.
	ScopeModeActive ScopeMode = "active"
	// ScopeModeLocked applies the values and the client cannot change them.
	ScopeModeLocked ScopeMode = "locked"
	// ScopeModeHidden applies the values without exposing them to the client.
	ScopeModeHidden ScopeMode = "hidden"
	// ScopeModeDenied hides the filter and rejects it when supplied.
	ScopeModeDenied ScopeMode = "denied"
)

// FilterNames is the canonical set of listing filters that a token can grant or
// deny. Attribute filters are scoped separately.
var FilterNames = []string{
	"type",
	"mime",
	"search",
	"sort",
	"color",
	"size",
	"duration",
	"pages",
	"dimensions",
	"added",
	"modified",
}

// FilterGrant is the token's scope for one listing filter. Values are defaults
// or enforced values depending on Mode; they never restrict which options the
// picker offers.
type FilterGrant struct {
	Values []string  `json:"values,omitempty"`
	Mode   ScopeMode `json:"mode"`
}

// UploadSetting narrows the formats and size limits available to a token. The
// settings are applied on top of the library's own upload policy and can only
// make it stricter.
type UploadSetting struct {
	Format    string   `json:"format,omitempty"`
	MimeTypes []string `json:"mime_types,omitempty"`
	MaxBytes  int64    `json:"max_bytes,omitempty"`
}

// Scope is the authorization envelope of a JWT-backed embed session. It is
// always enforced server-side; the frontend also reads it to render the
// matching controls.
type Scope struct {
	Username         string                 `json:"username,omitempty"`
	Filters          map[string]FilterGrant `json:"filters,omitempty"`
	AllFilters       bool                   `json:"all_filters,omitempty"`
	Attributes       []AttributeGrant       `json:"attributes,omitempty"`
	AllAttributes    bool                   `json:"all_attributes,omitempty"`
	UploadAttributes []AssetAttribute       `json:"-"`
	UploadSettings   []UploadSetting        `json:"upload_settings,omitempty"`
}

// Filter returns the grant for a filter name. Canonical names are type, mime,
// search, and sort.
func (s Scope) Filter(name string) (FilterGrant, bool) {
	grant, ok := s.Filters[name]
	return grant, ok
}

// Locked reports whether a filter is fixed by the token for the client.
func (s Scope) Locked(name string) bool {
	grant, ok := s.Filters[name]
	return ok && (grant.Mode == ScopeModeLocked || grant.Mode == ScopeModeHidden)
}

// Hidden reports whether a filter is enforced but omitted from the embed UI.
func (s Scope) Hidden(name string) bool {
	grant, ok := s.Filters[name]
	return ok && grant.Mode == ScopeModeHidden
}

// FilterDenied reports whether the token forbids a filter. Filters without a
// grant are denied unless the token has the wildcard grant.
func (s Scope) FilterDenied(name string) bool {
	if grant, ok := s.Filters[name]; ok {
		return grant.Mode == ScopeModeDenied
	}
	return !s.AllFilters
}

// FilterVisible reports whether the embed UI should show an editable filter
// control. Locked, hidden, and denied filters are not editable.
func (s Scope) FilterVisible(name string) bool {
	if grant, ok := s.Filters[name]; ok {
		return grant.Mode == ScopeModeAllowed || grant.Mode == ScopeModeActive
	}
	return s.AllFilters
}

// Actor is the authenticated caller for a request. Direct actors have an id
// and no scope; JWT actors carry a token-backed scope and a synthetic identity
// whose id comes from the token subject.
type Actor struct {
	ID           string
	Name         string
	Token        bool
	Scope        Scope
	Capabilities []string
}

// Can reports whether the actor is allowed to perform a capability.
func (a *Actor) Can(capability string) bool {
	return a != nil && Can(a.Capabilities, capability)
}

type actorContextKey struct{}

// WithActor attaches the authenticated actor to a request context.
func WithActor(ctx context.Context, actor *Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// ActorFromContext returns the authenticated actor, or nil for requests that
// bypass authentication (public delivery routes).
func ActorFromContext(ctx context.Context) *Actor {
	actor, _ := ctx.Value(actorContextKey{}).(*Actor)
	return actor
}
