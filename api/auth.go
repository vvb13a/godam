package api

import (
	"context"
	"net/http"
	"time"
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

// authMiddleware resolves the session cookie into a user and injects it into
// the request context. Unauthenticated requests to protected routes get 401.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		user, err := s.deps.Users.UserForSession(r.Context(), sessionToken(r))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
	writeJSON(w, http.StatusOK, currentUser(r))
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
