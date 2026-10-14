package api

import (
	"net/http"
	"strings"
)

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	libraryID, ok := requireLibraryID(w, r)
	if !ok {
		return
	}
	tags, err := s.deps.Tags.List(r.Context(), libraryID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// handleCreateTag creates a standalone tag (or returns the existing one) so
// tags can be managed without attaching them to an asset first.
func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var req tagRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(strings.TrimPrefix(req.Name, "#")) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.LibraryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "library_id is required"})
		return
	}
	tag, err := s.deps.Tags.GetOrCreate(r.Context(), req.LibraryID, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

type updateTagRequest struct {
	Name    string  `json:"name"`
	GroupID *string `json:"group_id"`
	Starred *bool   `json:"starred"`
}

// handleUpdateTag sets a tag's name, group (null for uncategorized) and
// starred flag.
func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	var req updateTagRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	starred := false
	if req.Starred != nil {
		starred = *req.Starred
	}
	tag, err := s.deps.Tags.Update(r.Context(), r.PathValue("id"), req.Name, req.GroupID, starred)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tag)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Tags.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
