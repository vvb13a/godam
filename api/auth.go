package api

import (
	"net/http"
	"strings"

	"github.com/vvb13a/godam/domain"
)

// publicPaths are reachable without authentication. Everything else under
// /api/ requires either a scoped JWT or trusted direct access. The embedded
// frontend is served outside this mux.
var publicPaths = map[string]bool{
	"/api/health":         true,
	"/api/system/info":    true,
	"/api/system/metrics": true,
}

// publicPrefixes are unauthenticated route families. /internal is only reachable
// from the Docker network (Caddy blocks it publicly); /convert is the early,
// not-yet-authorized conversion handoff.
var publicPrefixes = []string{
	"/convert/",
	"/img/",
	"/src/",
	"/internal/source/",
}

func isPublicPath(path string) bool {
	if publicPaths[path] {
		return true
	}
	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// authMiddleware resolves a scoped JWT into an actor and injects it into the
// request context. Requests without a JWT are treated as trusted direct access
// and receive an unscoped admin actor; deployments must restrict that path at
// the network/proxy layer.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// A supplied token must always verify. Never let an invalid or
		// unverifiable token fall through to the unscoped admin actor.
		if raw := requestToken(r); raw != "" {
			if !s.tokens.Enabled() {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}
			actor, err := s.tokens.Verify(raw)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}
			ctx := domain.WithActor(r.Context(), actor)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		ctx := domain.WithActor(r.Context(), &domain.Actor{ID: adminActorID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requestToken extracts a JWT from the standard Authorization header, a query
// parameter (needed for iframe and <img>/<video> URLs), or the explicit
// X-Godam-Token header used by embed clients.
func requestToken(r *http.Request) string {
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(auth, "Bearer ") {
		if token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")); token != "" {
			return token
		}
	}
	if token := strings.TrimSpace(r.Header.Get("X-Godam-Token")); token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func requestScope(r *http.Request) (domain.Scope, bool) {
	actor := domain.ActorFromContext(r.Context())
	if actor == nil || !actor.Token {
		return domain.Scope{}, false
	}
	return actor.Scope, true
}

type meResponse struct {
	ID    string        `json:"id"`
	Name  string        `json:"name,omitempty"`
	Scope *domain.Scope `json:"scope,omitempty"`
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	actor := currentActor(r)
	response := meResponse{ID: actor.ID, Name: actor.Name}
	if actor.Token {
		scope := actor.Scope
		response.Scope = &scope
	}
	writeJSON(w, http.StatusOK, response)
}
