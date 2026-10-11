package api

import "net/http"

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.deps.Tags.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}
