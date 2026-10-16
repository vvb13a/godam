// Package api exposes godam over a small JSON HTTP API so the Vue frontend
// can read and mutate the library. It wraps the services and holds no
// business logic of its own.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/storage"
)

// Deps carries the services the HTTP handlers operate on.
type Deps struct {
	Assets           *service.AssetService
	Collections      *service.CollectionService
	SmartCollections *service.SmartCollectionService
	Tags             *service.TagService
	TagGroups        *service.TagGroupService
	Libraries        *service.LibraryService
	Users            *service.UserService
	Stats            *service.StatsService
	MaxUploadBytes   int64
}

// Server is the HTTP handler for the JSON API.
type Server struct {
	deps    Deps
	mux     *http.ServeMux
	handler http.Handler
}

// NewServer builds the API handler with every route registered. The mux is
// wrapped in authentication middleware, so route handlers can assume an
// authenticated user except on the public auth routes.
func NewServer(deps Deps) *Server {
	s := &Server{
		deps: deps,
		mux:  http.NewServeMux(),
	}
	s.routes()
	s.handler = s.authMiddleware(s.mux)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/dev-login", s.handleDevLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/auth/me", s.handleMe)
	s.mux.HandleFunc("PUT /api/auth/profile", s.handleUpdateProfile)
	s.mux.HandleFunc("PUT /api/auth/password", s.handleChangePassword)

	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	s.mux.HandleFunc("GET /api/libraries", s.handleListLibraries)
	s.mux.HandleFunc("POST /api/libraries", s.handleCreateLibrary)
	s.mux.HandleFunc("PUT /api/libraries/sort", s.handleReorderLibraries)
	s.mux.HandleFunc("PUT /api/libraries/{id}", s.handleUpdateLibrary)
	s.mux.HandleFunc("PUT /api/libraries/{id}/cover", s.handleSetLibraryCover)
	s.mux.HandleFunc("PUT /api/libraries/{id}/watermark", s.handleSetLibraryWatermark)
	s.mux.HandleFunc("PUT /api/libraries/{id}/upload-settings", s.handleSetLibraryUploadSettings)
	s.mux.HandleFunc("DELETE /api/libraries/{id}", s.handleDeleteLibrary)

	s.mux.HandleFunc("GET /api/assets", s.handleListAssets)
	s.mux.HandleFunc("POST /api/assets/upload", s.handleUploadAsset)
	s.mux.HandleFunc("POST /api/assets/import", s.handleImportAsset)
	s.mux.HandleFunc("POST /api/assets/collections", s.handleAddAssetsToCollection)
	s.mux.HandleFunc("POST /api/assets/tags", s.handleApplyAssetsTags)
	s.mux.HandleFunc("GET /api/assets/zip", s.handleDownloadAssetsZip)
	s.mux.HandleFunc("GET /api/assets/{id}", s.handleGetAsset)
	s.mux.HandleFunc("PUT /api/assets/{id}", s.handleUpdateAsset)
	s.mux.HandleFunc("DELETE /api/assets/{id}", s.handleDeleteAsset)
	s.mux.HandleFunc("GET /api/assets/{id}/content", s.handleAssetContent)
	s.mux.HandleFunc("PUT /api/assets/{id}/content", s.handleReplaceAssetContent)
	s.mux.HandleFunc("GET /api/assets/{id}/preview", s.handleAssetPreview)
	s.mux.HandleFunc("POST /api/assets/{id}/preview", s.handleRegeneratePreview)
	s.mux.HandleFunc("POST /api/assets/{id}/tags", s.handleAddTag)
	s.mux.HandleFunc("DELETE /api/assets/{id}/tags/{tagId}", s.handleRemoveTag)
	s.mux.HandleFunc("POST /api/assets/{id}/collections", s.handleAddToCollection)
	s.mux.HandleFunc("DELETE /api/assets/{id}/collections/{collectionId}", s.handleRemoveFromCollection)

	s.mux.HandleFunc("GET /api/collections", s.handleListCollections)
	s.mux.HandleFunc("POST /api/collections", s.handleCreateCollection)
	s.mux.HandleFunc("PUT /api/collections/{id}", s.handleUpdateCollection)
	s.mux.HandleFunc("PUT /api/collections/{id}/move", s.handleMoveCollection)
	s.mux.HandleFunc("DELETE /api/collections/{id}", s.handleDeleteCollection)

	s.mux.HandleFunc("GET /api/smart-collections", s.handleListSmartCollections)
	s.mux.HandleFunc("POST /api/smart-collections", s.handleCreateSmartCollection)
	s.mux.HandleFunc("PUT /api/smart-collections/sort", s.handleReorderSmartCollections)
	s.mux.HandleFunc("PUT /api/smart-collections/{id}", s.handleUpdateSmartCollection)
	s.mux.HandleFunc("DELETE /api/smart-collections/{id}", s.handleDeleteSmartCollection)

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
	case errors.Is(err, service.ErrInvalidCredentials):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrEmailTaken):
		status = http.StatusConflict
	case errors.Is(err, service.ErrInvalidInput), errors.Is(err, service.ErrWeakPassword):
		status = http.StatusBadRequest
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

// requireLibraryID reads the mandatory library_id query parameter that scopes
// every library-owned listing, writing a 400 and returning false when absent.
func requireLibraryID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.URL.Query().Get("library_id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "library_id is required"})
		return "", false
	}
	return id, true
}

// ---- Authentication context & ownership ----

type contextKey int

const userContextKey contextKey = iota

func userFromContext(ctx context.Context) *domain.User {
	user, _ := ctx.Value(userContextKey).(*domain.User)
	return user
}

// currentUser returns the authenticated user. The auth middleware guarantees a
// non-nil user on every protected route.
func currentUser(r *http.Request) *domain.User {
	return userFromContext(r.Context())
}

// requireOwnedLibrary verifies the library belongs to the authenticated user,
// writing an error and returning false when it does not.
func (s *Server) requireOwnedLibrary(w http.ResponseWriter, r *http.Request, libraryID string) bool {
	user := currentUser(r)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return false
	}
	if err := s.deps.Libraries.VerifyOwned(r.Context(), libraryID, user.ID); err != nil {
		writeError(w, err)
		return false
	}
	return true
}

// The owned* helpers resolve a resource by id and confirm its library belongs
// to the authenticated user before handing it back.

func (s *Server) ownedAsset(w http.ResponseWriter, r *http.Request, id string) (*domain.Asset, bool) {
	asset, err := s.deps.Assets.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	if !s.requireOwnedLibrary(w, r, asset.LibraryID) {
		return nil, false
	}
	return asset, true
}

func (s *Server) ownedTag(w http.ResponseWriter, r *http.Request, id string) (*domain.Tag, bool) {
	tag, err := s.deps.Tags.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	if !s.requireOwnedLibrary(w, r, tag.LibraryID) {
		return nil, false
	}
	return tag, true
}

func (s *Server) ownedCollection(w http.ResponseWriter, r *http.Request, id string) (*domain.Collection, bool) {
	collection, err := s.deps.Collections.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	if !s.requireOwnedLibrary(w, r, collection.LibraryID) {
		return nil, false
	}
	return collection, true
}

func (s *Server) ownedTagGroup(w http.ResponseWriter, r *http.Request, id string) (*domain.TagGroup, bool) {
	group, err := s.deps.TagGroups.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	if !s.requireOwnedLibrary(w, r, group.LibraryID) {
		return nil, false
	}
	return group, true
}

func (s *Server) ownedSmartCollection(w http.ResponseWriter, r *http.Request, id string) (*domain.SmartCollection, bool) {
	collection, err := s.deps.SmartCollections.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	if !s.requireOwnedLibrary(w, r, collection.LibraryID) {
		return nil, false
	}
	return collection, true
}
