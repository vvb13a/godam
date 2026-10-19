package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/service"
)

type presetRequest struct {
	Label         string                  `json:"label"`
	Name          string                  `json:"name"`
	Parameters    domain.PresetParameters `json:"parameters"`
	LibraryWide   *bool                   `json:"library_wide"`
	CollectionIDs []string                `json:"collection_ids"`
	TagIDs        []string                `json:"tag_ids"`
}

func (r presetRequest) libraryWide() bool {
	return r.LibraryWide == nil || *r.LibraryWide
}

func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	presets, err := s.deps.Presets.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, presets)
}

// handlePresetCatalog returns the imgproxy manipulation catalog used by the
// Vue configurator. It is the single source of truth for option names and
// argument validation.
func (s *Server) handlePresetCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, service.ImgProxyCatalog())
}

func (s *Server) handleCreatePreset(w http.ResponseWriter, r *http.Request) {
	var req presetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	preset, err := s.deps.Presets.Create(
		r.Context(), currentUser(r).ID, req.Label, req.Name, req.Parameters,
		req.libraryWide(), req.CollectionIDs, req.TagIDs,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, preset)
}

func (s *Server) handleUpdatePreset(w http.ResponseWriter, r *http.Request) {
	var req presetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	preset, err := s.deps.Presets.Update(
		r.Context(), r.PathValue("id"), req.Label, req.Name, req.Parameters,
		req.libraryWide(), req.CollectionIDs, req.TagIDs,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preset)
}

func (s *Server) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Presets.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type signImgproxyRequest struct {
	AssetID    string `json:"asset_id"`
	Format     string `json:"format"`
	Processing string `json:"processing"`
}

// handlePresetPreview renders unsaved preset parameters for an asset without
// touching the conversion cache.
func (s *Server) handlePresetPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	var params domain.PresetParameters
	if !decodeJSON(w, r, &params) {
		return
	}
	body, contentType, err := s.deps.Conversions.Preview(r.Context(), id, params)
	if err != nil {
		writeError(w, err)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, body)
}

// handleSignImgproxy is the second access pattern: callers can ask godam to
// sign an ad-hoc imgproxy processing pipeline. The returned URL is served by
// godam (and cached by Caddy) just like preset conversions.
func (s *Server) handleSignImgproxy(w http.ResponseWriter, r *http.Request) {
	var req signImgproxyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.AssetID) == "" || strings.TrimSpace(req.Processing) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset_id and processing are required"})
		return
	}
	segment, err := s.deps.Conversions.Sign(r.Context(), req.AssetID, req.Format, req.Processing)
	if err != nil {
		writeError(w, err)
		return
	}
	assetID := url.PathEscape(req.AssetID)
	if base := strings.TrimRight(s.deps.AssetsBaseURL, "/"); base != "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"segment": segment,
			"url":     base + "/" + assetID + "/" + segment,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"segment": segment,
		"url":     "/convert/" + assetID + "/" + segment,
	})
}
