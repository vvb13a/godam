package api

import "net/http"

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	libraryID, ok := requireLibraryID(w, r)
	if !ok {
		return
	}
	stats, err := s.deps.Stats.Get(r.Context(), libraryID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
