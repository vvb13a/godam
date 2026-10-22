package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/vvb13a/godam/domain"
)

const sessionCookieName = "godam_session"

// publicPaths are reachable without a session. Everything else under /api/
// requires authentication. The embedded frontend is served outside this mux.
var publicPaths = map[string]bool{
	"/api/health":         true,
	"/api/auth/login":     true,
	"/api/auth/dev-login": true,
	"/api/auth/logout":    true,
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

// authMiddleware resolves a session cookie or a scoped JWT into a user and an
// actor and injects both into the request context. Unauthenticated requests to
// protected routes get 401.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// A supplied JWT always wins over an existing session cookie so an embed
		// cannot accidentally inherit a broader godam login from the same browser.
		if raw := requestToken(r); raw != "" && s.tokens.Enabled() {
			actor, err := s.tokens.Verify(raw)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}
			user := &domain.User{ID: actor.ID, Name: actor.Name}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = domain.WithActor(ctx, actor)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if user, err := s.deps.Users.UserForSession(r.Context(), sessionToken(r)); err == nil {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = domain.WithActor(ctx, &domain.Actor{ID: user.ID, Name: user.Name})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
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

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// startSession creates a session and sets the cookie, reporting false (with an
// error already written) on failure.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string) bool {
	token, err := s.deps.Users.StartSession(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return false
	}
	setSessionCookie(w, token)
	return true
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	scope, scoped := requestScope(r)
	if !scoped {
		writeJSON(w, http.StatusOK, currentUser(r))
		return
	}
	writeJSON(w, http.StatusOK, struct {
		*domain.User
		Scope *domain.Scope `json:"scope"`
	}{User: currentUser(r), Scope: &scope})
}

type userOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.deps.Users.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	options := make([]userOption, 0, len(users))
	for _, user := range users {
		options = append(options, userOption{ID: user.ID, Name: user.Name})
	}
	writeJSON(w, http.StatusOK, options)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.deps.Users.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.startSession(w, r, user.ID) {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// handleDevLogin auto-creates (and logs in) the developer account so local
// development does not require a pre-existing user.
func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	user, err := s.deps.Users.DevUser(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.startSession(w, r, user.ID) {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	_ = s.deps.Users.EndSession(r.Context(), sessionToken(r))
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type profileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.deps.Users.UpdateProfile(r.Context(), currentUser(r).ID, req.Name, req.Email)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

type passwordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req passwordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Users.ChangePassword(r.Context(), currentUser(r).ID, req.CurrentPassword, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
