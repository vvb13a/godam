package domain

import "context"

// ScopeFilters is the filter set a JWT may apply to a library listing. The JSON
// names match the embed URL query parameters so a token can be mapped onto the
// picker without a translation layer. CollectionIDs and TagIDs accept both the
// short embed names ("collection"/"tag") and the API names via the verifier.
type ScopeFilters struct {
	CollectionIDs []string `json:"collection,omitempty"`
	TagIDs        []string `json:"tag,omitempty"`
	Types         []string `json:"type,omitempty"`
	MimeTypes     []string `json:"mime,omitempty"`
	Uncategorized bool     `json:"uncategorized,omitempty"`
	Untagged      bool     `json:"untagged,omitempty"`
	Search        string   `json:"search,omitempty"`
	Sort          string   `json:"sort,omitempty"`
}

// Scope is the authorization envelope of a JWT-backed embed session. It is
// always enforced server-side; the frontend also reads it to render the
// matching locked controls.
type Scope struct {
	Username         string       `json:"username,omitempty"`
	DefaultFilters   ScopeFilters `json:"default_filters"`
	LockedFilters    []string     `json:"locked_filters,omitempty"`
	AllowedMimeTypes []string     `json:"allowed_mime_types,omitempty"`
	CanUpload        bool         `json:"can_upload"`
}

// Locked reports whether a filter name is fixed by the token. Names use the
// embed query vocabulary ("collection", "tag", "type", "mime", "search",
// "sort").
func (s Scope) Locked(name string) bool {
	for _, locked := range s.LockedFilters {
		if locked == name {
			return true
		}
	}
	return false
}

// Actor is the authenticated caller for a request. Direct actors have an id
// and no scope; JWT actors carry a token-backed scope and a synthetic identity
// whose id comes from the token subject.
type Actor struct {
	ID    string
	Name  string
	Token bool
	Scope Scope
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
