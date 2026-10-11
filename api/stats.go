package api

import "net/http"

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.deps.Stats.Get(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
