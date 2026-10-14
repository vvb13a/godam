package api

import "net/http"

func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request) {
	libraryID, ok := requireLibraryID(w, r)
	if !ok {
		return
	}
	if !s.requireOwnedLibrary(w, r, libraryID) {
		return
	}
	collections, err := s.deps.Collections.List(r.Context(), libraryID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collections)
}

type collectionRequest struct {
	LibraryID   string `json:"library_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var req collectionRequest
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
	collection, err := s.deps.Collections.Create(r.Context(), req.LibraryID, req.Name, req.Description, req.Icon)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, collection)
}

func (s *Server) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	var req collectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if _, ok := s.ownedCollection(w, r, r.PathValue("id")); !ok {
		return
	}
	collection, err := s.deps.Collections.Update(r.Context(), r.PathValue("id"), req.Name, req.Description, req.Icon)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

func (s *Server) handleDuplicateCollection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedCollection(w, r, r.PathValue("id")); !ok {
		return
	}
	collection, err := s.deps.Collections.Duplicate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, collection)
}

func (s *Server) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedCollection(w, r, r.PathValue("id")); !ok {
		return
	}
	if err := s.deps.Collections.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
