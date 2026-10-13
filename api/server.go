// Package api exposes godam over a small JSON HTTP API so the Vue frontend
// can read and mutate the library. It wraps the services and holds no
// business logic of its own.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/storage"
)

// Deps carries the services the HTTP handlers operate on.
type Deps struct {
	Assets         *service.AssetService
	Collections    *service.CollectionService
	Tags           *service.TagService
	TagGroups      *service.TagGroupService
	Stats          *service.StatsService
	MaxUploadBytes int64
}

// Server is the HTTP handler for the JSON API.
type Server struct {
	deps Deps
	mux  *http.ServeMux
}

// NewServer builds the API handler with every route registered.
func NewServer(deps Deps) *Server {
	s := &Server{
		deps: deps,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	s.mux.HandleFunc("GET /api/assets", s.handleListAssets)
	s.mux.HandleFunc("POST /api/assets/upload", s.handleUploadAsset)
	s.mux.HandleFunc("POST /api/assets/import", s.handleImportAsset)
	s.mux.HandleFunc("GET /api/assets/{id}", s.handleGetAsset)
	s.mux.HandleFunc("DELETE /api/assets/{id}", s.handleDeleteAsset)
	s.mux.HandleFunc("GET /api/assets/{id}/content", s.handleAssetContent)
	s.mux.HandleFunc("GET /api/assets/{id}/preview", s.handleAssetPreview)
	s.mux.HandleFunc("POST /api/assets/{id}/tags", s.handleAddTag)
	s.mux.HandleFunc("DELETE /api/assets/{id}/tags/{tagId}", s.handleRemoveTag)
	s.mux.HandleFunc("POST /api/assets/{id}/collections", s.handleAddToCollection)
	s.mux.HandleFunc("DELETE /api/assets/{id}/collections/{collectionId}", s.handleRemoveFromCollection)

	s.mux.HandleFunc("GET /api/collections", s.handleListCollections)
	s.mux.HandleFunc("POST /api/collections", s.handleCreateCollection)
	s.mux.HandleFunc("PUT /api/collections/{id}", s.handleUpdateCollection)
	s.mux.HandleFunc("POST /api/collections/{id}/duplicate", s.handleDuplicateCollection)
	s.mux.HandleFunc("DELETE /api/collections/{id}", s.handleDeleteCollection)

	s.mux.HandleFunc("GET /api/tags", s.handleListTags)
	s.mux.HandleFunc("POST /api/tags", s.handleCreateTag)
	s.mux.HandleFunc("PUT /api/tags/{id}", s.handleUpdateTag)
	s.mux.HandleFunc("DELETE /api/tags/{id}", s.handleDeleteTag)

	s.mux.HandleFunc("GET /api/tag-groups", s.handleListTagGroups)
	s.mux.HandleFunc("POST /api/tag-groups", s.handleCreateTagGroup)
	s.mux.HandleFunc("PUT /api/tag-groups/{id}", s.handleUpdateTagGroup)
	s.mux.HandleFunc("DELETE /api/tag-groups/{id}", s.handleDeleteTagGroup)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- Helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrUnsupportedFormat), errors.Is(err, service.ErrFileTooLarge):
		status = http.StatusBadRequest
	case errors.Is(err, storage.ErrDriverNotFound):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decodeJSON reads a JSON request body into v, writing a 400 and reporting
// false when the body is invalid.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}
