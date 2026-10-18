package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// requireManagementToken protects fleet-facing endpoints with a token that is
// separate from user sessions and library tokens. When no token is configured
// the endpoints do not exist, so a misconfigured deployment does not expose
// metrics by accident.
func (s *Server) requireManagementToken(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(s.deps.ManagementToken)
	if expected == "" {
		http.NotFound(w, r)
		return false
	}
	provided := strings.TrimSpace(r.Header.Get("X-Godam-Management-Token"))
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		provided = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "management token required"})
		return false
	}
	return true
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireManagementToken(w, r) {
		return
	}
	info, err := s.deps.System.Info(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.requireManagementToken(w, r) {
		return
	}
	metrics, err := s.deps.System.Metrics(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}
