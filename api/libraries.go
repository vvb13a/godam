package api

import (
	"net/http"

	"github.com/vvb13a/godam/service"
)

func (s *Server) handleListLibraries(w http.ResponseWriter, r *http.Request) {
	libraries, err := s.deps.Libraries.List(r.Context(), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, libraries)
}

type libraryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Sort  int    `json:"sort"`
}

// handleReorderLibraries applies sort positions in bulk for a drag reorder.
type librarySortRequest struct {
	Items []service.LibrarySort `json:"items"`
}

func (s *Server) handleReorderLibraries(w http.ResponseWriter, r *http.Request) {
	var req librarySortRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Libraries.Reorder(r.Context(), currentUser(r).ID, req.Items); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req libraryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	lib, err := s.deps.Libraries.Create(r.Context(), currentUser(r).ID, req.Name, req.Color, req.Sort)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, lib)
}

func (s *Server) handleUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	var req libraryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	lib, err := s.deps.Libraries.Update(r.Context(), r.PathValue("id"), currentUser(r).ID, req.Name, req.Color, req.Sort)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lib)
}

func (s *Server) handleDeleteLibrary(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Libraries.Delete(r.Context(), r.PathValue("id"), currentUser(r).ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
