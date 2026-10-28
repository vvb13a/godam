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
	Assets          *service.AssetService
	Conversions     *service.ConversionService
	Presets         *service.PresetService
	AssetsBaseURL   string
	Instance        *service.InstanceService
	Stats           *service.StatsService
	Backups         *service.BackupService
	System          *service.SystemService
	ManagementToken string

	// JWTSecret enables scoped embed tokens. Issuer and Audience are optional
	// and enforced when set.
	JWTSecret   string
	JWTIssuer   string
	JWTAudience string
}

// Server is the HTTP handler for the JSON API.
type Server struct {
	deps    Deps
	mux     *http.ServeMux
	handler http.Handler
	tokens  *service.JWTVerifier
}

// NewServer builds the API handler with every route registered. The mux is
// wrapped in authentication middleware, so route handlers can assume an
// authenticated user except on the public auth routes.
func NewServer(deps Deps) *Server {
	s := &Server{
		deps:   deps,
		mux:    http.NewServeMux(),
		tokens: service.NewJWTVerifier(deps.JWTSecret, deps.JWTIssuer, deps.JWTAudience),
	}
	s.routes()
	s.handler = s.authMiddleware(s.mux)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Delivery routes live outside /api so they can be used directly by
	// consuming sites. /img requires an explicit preset or signed spec and
	// resolves to the visual source asset (image or derived preview); /src
	// streams originals for audio/video/pdf only. The library's own 400px
	// preview stays on the authenticated /api/assets/{id}/preview alias.
	s.mux.HandleFunc("GET /img/{id}/{spec}", s.handleImageDelivery)
	s.mux.HandleFunc("GET /src/{id}", s.handleSourceDelivery)

	// Conversion routes live outside /api so Caddy can fall back to them from
	// the assets subdomain cache. The internal source route is fetched by
	// imgproxy and is never proxied to the public site.
	s.mux.HandleFunc("GET /convert/{id}/{spec}", s.handleConversion)
	s.mux.HandleFunc("GET /internal/source/{id}", s.handleConversionSource)
	s.mux.HandleFunc("POST /api/conversions/purge", s.withCapability(domain.CapabilityConversionsPurge, s.handlePurgeConversions))

	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/system/info", s.handleSystemInfo)
	s.mux.HandleFunc("GET /api/system/metrics", s.handleSystemMetrics)

	s.mux.HandleFunc("GET /api/auth/me", s.handleMe)

	s.mux.HandleFunc("GET /api/stats", s.withCapability(domain.CapabilityAssetsRead, s.handleStats))

	s.mux.HandleFunc("GET /api/presets", s.withCapability(domain.CapabilityAssetsRead, s.handleListPresets))
	s.mux.HandleFunc("GET /api/presets/catalog", s.withCapability(domain.CapabilitySettingsPresets, s.handlePresetCatalog))
	s.mux.HandleFunc("POST /api/presets", s.withCapability(domain.CapabilitySettingsPresets, s.handleCreatePreset))
	s.mux.HandleFunc("PUT /api/presets/{id}", s.withCapability(domain.CapabilitySettingsPresets, s.handleUpdatePreset))
	s.mux.HandleFunc("DELETE /api/presets/{id}", s.withCapability(domain.CapabilitySettingsPresets, s.handleDeletePreset))
	s.mux.HandleFunc("POST /api/imgproxy/sign", s.withCapability(domain.CapabilitySettingsPresets, s.handleSignImgproxy))
	s.mux.HandleFunc("POST /api/assets/{id}/preset-preview", s.withCapability(domain.CapabilitySettingsPresets, s.handlePresetPreview))
	s.mux.HandleFunc("POST /api/assets/presets/warm", s.withCapability(domain.CapabilityConversionsWarm, s.handleWarmPresets))

	s.mux.HandleFunc("GET /api/instance", s.withCapability(domain.CapabilityAssetsRead, s.handleGetInstance))
	s.mux.HandleFunc("PUT /api/instance", s.withCapability(domain.CapabilityLibraryUpdate, s.handleUpdateInstance))
	s.mux.HandleFunc("PUT /api/instance/cover", s.withCapability(domain.CapabilityLibraryUpdate, s.handleSetInstanceCover))
	s.mux.HandleFunc("PUT /api/instance/watermark", s.withCapability(domain.CapabilityLibraryUpdate, s.handleSetInstanceWatermark))
	s.mux.HandleFunc("PUT /api/instance/upload-settings", s.withCapability(domain.CapabilitySettingsUpload, s.handleSetInstanceUploadSettings))
	s.mux.HandleFunc("PUT /api/instance/attribute-stamping", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetInstanceAttributeStamping))
	s.mux.HandleFunc("GET /api/instance/backup", s.withCapability(domain.CapabilitySettingsStorage, s.handleGetBackup))
	s.mux.HandleFunc("PUT /api/instance/backup", s.withCapability(domain.CapabilitySettingsStorage, s.handleSetBackup))
	s.mux.HandleFunc("POST /api/instance/backup/test", s.withCapability(domain.CapabilitySettingsStorage, s.handleTestBackup))
	s.mux.HandleFunc("POST /api/instance/backup/run", s.withCapability(domain.CapabilitySettingsStorage, s.handleRunBackup))
	s.mux.HandleFunc("GET /api/instance/backup/status", s.withCapability(domain.CapabilitySettingsStorage, s.handleBackupStatus))

	s.mux.HandleFunc("GET /api/assets", s.withCapability(domain.CapabilityAssetsRead, s.handleListAssets))
	s.mux.HandleFunc("GET /api/attributes", s.withCapability(domain.CapabilityAssetsRead, s.handleListAttributes))
	s.mux.HandleFunc("GET /api/attributes/catalog", s.withCapability(domain.CapabilityAssetsRead, s.handleListAttributeCatalog))
	s.mux.HandleFunc("PUT /api/attributes/metadata/label", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeLabel))
	s.mux.HandleFunc("PUT /api/attributes/metadata/sort", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeSort))
	s.mux.HandleFunc("PUT /api/attributes/metadata/render", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeRender))
	s.mux.HandleFunc("POST /api/attributes/metadata/values", s.withAnyCapability(s.handleAddAttributeValues, domain.CapabilitySettingsAttributes, domain.CapabilityAttributesCreateValues))
	s.mux.HandleFunc("PUT /api/attributes/metadata/values-lock", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeValuesLocked))
	s.mux.HandleFunc("POST /api/attributes", s.withAnyCapability(s.handleCreateAttribute, domain.CapabilitySettingsAttributes, domain.CapabilityAttributesCreateKeys))
	s.mux.HandleFunc("DELETE /api/attributes/metadata/value", s.withCapability(domain.CapabilitySettingsAttributes, s.handleDeleteAttributeValue))
	s.mux.HandleFunc("DELETE /api/attributes/metadata/key", s.withCapability(domain.CapabilitySettingsAttributes, s.handleDeleteAttributeKey))
	s.mux.HandleFunc("PUT /api/attributes/metadata/search", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeSearchable))
	s.mux.HandleFunc("PUT /api/attributes/metadata/multi", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeMulti))
	s.mux.HandleFunc("PUT /api/attributes/metadata/unassigned", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeUnassigned))
	s.mux.HandleFunc("PUT /api/attributes/metadata/unassigned-label", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeUnassignedLabel))
	s.mux.HandleFunc("PUT /api/attributes/metadata/parent", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeParent))
	s.mux.HandleFunc("PUT /api/attributes/metadata/value-search", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeValueSearch))
	s.mux.HandleFunc("PUT /api/attributes/metadata/description", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeDescription))
	s.mux.HandleFunc("PUT /api/attributes/metadata/color", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeColor))
	s.mux.HandleFunc("PUT /api/attributes/metadata/value-order", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeValueOrder))
	s.mux.HandleFunc("PUT /api/attributes/metadata/key-order", s.withCapability(domain.CapabilitySettingsAttributes, s.handleSetAttributeKeyOrder))
	s.mux.HandleFunc("GET /api/attributes/tree", s.withCapability(domain.CapabilityAssetsRead, s.handleAttributeTree))
	s.mux.HandleFunc("POST /api/assets/upload", s.withCapability(domain.CapabilityAssetsUpload, s.handleUploadAsset))
	s.mux.HandleFunc("POST /api/assets/uploads", s.withCapability(domain.CapabilityAssetsUpload, s.handleStartUpload))
	s.mux.HandleFunc("PUT /api/assets/uploads/{id}/chunks", s.withCapability(domain.CapabilityAssetsUpload, s.handleUploadChunk))
	s.mux.HandleFunc("POST /api/assets/uploads/{id}/complete", s.withCapability(domain.CapabilityAssetsUpload, s.handleCompleteUpload))
	s.mux.HandleFunc("DELETE /api/assets/uploads/{id}", s.withCapability(domain.CapabilityAssetsUpload, s.handleCancelUpload))
	s.mux.HandleFunc("POST /api/assets/attributes", s.withCapability(domain.CapabilityAttributesSelect, s.handleApplyAssetsAttributes))
	s.mux.HandleFunc("GET /api/assets/zip", s.withCapability(domain.CapabilityAssetsRead, s.handleDownloadAssetsZip))
	s.mux.HandleFunc("GET /api/assets/{id}", s.withCapability(domain.CapabilityAssetsRead, s.handleGetAsset))
	s.mux.HandleFunc("PUT /api/assets/{id}", s.withCapability(domain.CapabilityAssetsUpdate, s.handleUpdateAsset))
	s.mux.HandleFunc("PUT /api/assets/{id}/attributes", s.withCapability(domain.CapabilityAttributesSelect, s.handleSetAssetAttributes))
	s.mux.HandleFunc("DELETE /api/assets/{id}", s.withCapability(domain.CapabilityAssetsDelete, s.handleDeleteAsset))
	s.mux.HandleFunc("GET /api/assets/{id}/content", s.withCapability(domain.CapabilityAssetsRead, s.handleAssetContent))
	s.mux.HandleFunc("GET /api/assets/{id}/conversions", s.withCapability(domain.CapabilityAssetsRead, s.handleListConversions))
	s.mux.HandleFunc("PUT /api/assets/{id}/content", s.withCapability(domain.CapabilityAssetsUpdate, s.handleReplaceAssetContent))
	s.mux.HandleFunc("GET /api/assets/{id}/preview", s.withCapability(domain.CapabilityAssetsRead, s.handleAssetPreview))
	s.mux.HandleFunc("POST /api/assets/{id}/preview", s.withCapability(domain.CapabilityAssetsUpdate, s.handleRegeneratePreview))
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
	case errors.Is(err, service.ErrPresetNameTaken):
		status = http.StatusConflict
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrUnsupportedFormat), errors.Is(err, service.ErrFileTooLarge):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrNotImage):
		status = http.StatusUnsupportedMediaType
	case errors.Is(err, service.ErrConversionFailed):
		status = http.StatusBadGateway
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

// ---- Authentication context & ownership ----

// adminActorID identifies direct (non-embed) callers for upload-session
// ownership. Direct access is expected to be restricted at the network layer.
const adminActorID = "admin"

// currentActor returns the authenticated actor. The auth middleware guarantees
// a non-nil actor on every protected route.
func currentActor(r *http.Request) *domain.Actor {
	actor := domain.ActorFromContext(r.Context())
	if actor == nil {
		return &domain.Actor{ID: adminActorID}
	}
	return actor
}

// requireCapability enforces a token capability. Direct access carries all
// capabilities; JWT actors must include the capability or a matching wildcard.
func (s *Server) requireCapability(w http.ResponseWriter, r *http.Request, capability string) bool {
	actor := domain.ActorFromContext(r.Context())
	if actor.Can(capability) {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error": "missing capability: " + capability,
	})
	return false
}

func (s *Server) withCapability(capability string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.requireCapability(w, r, capability) {
			return
		}
		next(w, r)
	}
}

func (s *Server) withAnyCapability(next http.HandlerFunc, capabilities ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := domain.ActorFromContext(r.Context())
		for _, capability := range capabilities {
			if actor.Can(capability) {
				next(w, r)
				return
			}
		}
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "missing required capability",
		})
	}
}

// The owned* helpers resolve a resource by id.

func (s *Server) ownedAsset(w http.ResponseWriter, r *http.Request, id string) (*domain.Asset, bool) {
	asset, err := s.deps.Assets.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return nil, false
	}
	if actor := domain.ActorFromContext(r.Context()); !assetVisibleToActor(asset, actor) {
		writeError(w, domain.ErrNotFound)
		return nil, false
	}
	return asset, true
}

// assetVisibleToActor enforces locked and hidden token attributes. Active
// attributes are filters, not access restrictions.
func assetVisibleToActor(asset *domain.Asset, actor *domain.Actor) bool {
	if actor == nil || !actor.Token {
		return true
	}
	for _, grant := range actor.Scope.Attributes {
		if grant.Mode == domain.AttributeModeActive {
			continue
		}
		found := false
		for _, attr := range asset.Attributes {
			if attr.Key != grant.Key {
				continue
			}
			for _, value := range grant.Values {
				if attr.Value == value {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
