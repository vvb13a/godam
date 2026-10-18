package api

import (
	"net/http"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/service"
)

func (s *Server) handleListSmartCollections(w http.ResponseWriter, r *http.Request) {
	collections, err := s.deps.SmartCollections.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collections)
}

type smartCollectionRequest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Color       string                       `json:"color"`
	Filter      domain.SmartCollectionFilter `json:"filter"`
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
	collection, err := s.deps.SmartCollections.Create(r.Context(), req.Name, req.Description, req.Color, req.Filter)
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
	collection, err := s.deps.SmartCollections.Update(r.Context(), r.PathValue("id"), req.Name, req.Description, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collection)
}

type smartCollectionSortRequest struct {
	Items []service.SmartCollectionSort `json:"items"`
}

// handleReorderSmartCollections applies sort positions in bulk for a drag
// reorder of the flat smart collections list.
func (s *Server) handleReorderSmartCollections(w http.ResponseWriter, r *http.Request) {
	var req smartCollectionSortRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.SmartCollections.Reorder(r.Context(), req.Items); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
