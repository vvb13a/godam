package api

import (
	"net/http"

	"github.com/vvb13a/godam/domain"
)

func (s *Server) handleListSmartCollections(w http.ResponseWriter, r *http.Request) {
	libraryID, ok := requireLibraryID(w, r)
	if !ok {
		return
	}
	if !s.requireOwnedLibrary(w, r, libraryID) {
		return
	}
	collections, err := s.deps.SmartCollections.List(r.Context(), libraryID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collections)
}

type smartCollectionRequest struct {
	LibraryID string                       `json:"library_id"`
	Name      string                       `json:"name"`
	Color     string                       `json:"color"`
	Filter    domain.SmartCollectionFilter `json:"filter"`
}

func (s *Server) handleCreateSmartCollection(w http.ResponseWriter, r *http.Request) {
	var req smartCollectionRequest
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
	collection, err := s.deps.SmartCollections.Create(r.Context(), req.LibraryID, req.Name, req.Color, req.Filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, collection)
}

func (s *Server) handleUpdateSmartCollection(w http.ResponseWriter, r *http.Request) {
	var req smartCollectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if _, ok := s.ownedSmartCollection(w, r, r.PathValue("id")); !ok {
		return
	}
	collection, err := s.deps.SmartCollections.Update(r.Context(), r.PathValue("id"), req.Name, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

func (s *Server) handleDeleteSmartCollection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedSmartCollection(w, r, r.PathValue("id")); !ok {
		return
	}
	if err := s.deps.SmartCollections.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
