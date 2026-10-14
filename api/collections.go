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
	LibraryID string  `json:"library_id"`
	ParentID  *string `json:"parent_id"`
	Name      string  `json:"name"`
	Color     string  `json:"color"`
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
	collection, err := s.deps.Collections.Create(r.Context(), req.LibraryID, req.ParentID, req.Name, req.Color)
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
	collection, err := s.deps.Collections.Update(r.Context(), r.PathValue("id"), req.Name, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

type collectionMoveRequest struct {
	ParentID *string `json:"parent_id"`
	Position int     `json:"position"`
}

// handleMoveCollection reparents/reorders a folder for drag-and-drop.
func (s *Server) handleMoveCollection(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedCollection(w, r, r.PathValue("id")); !ok {
		return
	}
	var req collectionMoveRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Collections.Move(r.Context(), r.PathValue("id"), req.ParentID, req.Position); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
