package api

import "net/http"

func (s *Server) handleListTagGroups(w http.ResponseWriter, r *http.Request) {
	libraryID, ok := requireLibraryID(w, r)
	if !ok {
		return
	}
	if !s.requireOwnedLibrary(w, r, libraryID) {
		return
	}
	groups, err := s.deps.TagGroups.List(r.Context(), libraryID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

type tagGroupRequest struct {
	LibraryID   string `json:"library_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func (s *Server) handleCreateTagGroup(w http.ResponseWriter, r *http.Request) {
	var req tagGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if req.LibraryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "library_id is required"})
		return
	}
	if !s.requireOwnedLibrary(w, r, req.LibraryID) {
		return
	}
	group, err := s.deps.TagGroups.Create(r.Context(), req.LibraryID, req.Name, req.Description, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) handleUpdateTagGroup(w http.ResponseWriter, r *http.Request) {
	var req tagGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if _, ok := s.ownedTagGroup(w, r, r.PathValue("id")); !ok {
		return
	}
	group, err := s.deps.TagGroups.Update(r.Context(), r.PathValue("id"), req.Name, req.Description, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (s *Server) handleDeleteTagGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedTagGroup(w, r, r.PathValue("id")); !ok {
		return
	}
	if err := s.deps.TagGroups.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
