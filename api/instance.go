package api

import (
	"net/http"

	"github.com/vvb13a/godam/domain"
)

func (s *Server) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	instance, err := s.deps.Instance.Get(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

type instanceRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (s *Server) handleUpdateInstance(w http.ResponseWriter, r *http.Request) {
	var req instanceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	instance, err := s.deps.Instance.Update(r.Context(), req.Name, req.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

type attributeStampingRequest struct {
	Keys   bool `json:"keys"`
	Values bool `json:"values"`
}

func (s *Server) handleSetInstanceAttributeStamping(w http.ResponseWriter, r *http.Request) {
	var req attributeStampingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	instance, err := s.deps.Instance.SetAttributeStamping(
		r.Context(), req.Keys, req.Values,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

type instanceAssetRefRequest struct {
	AssetID string `json:"asset_id"`
}

func (s *Server) handleSetInstanceCover(w http.ResponseWriter, r *http.Request) {
	var req instanceAssetRefRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	instance, err := s.deps.Instance.SetCover(r.Context(), req.AssetID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

func (s *Server) handleSetInstanceWatermark(w http.ResponseWriter, r *http.Request) {
	var req instanceAssetRefRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	instance, err := s.deps.Instance.SetWatermark(r.Context(), req.AssetID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}

type instanceUploadSettingsRequest struct {
	AllowedMimeTypes []string            `json:"allowed_mime_types"`
	UploadLimits     domain.UploadLimits `json:"upload_limits"`
}

func (s *Server) handleSetInstanceUploadSettings(w http.ResponseWriter, r *http.Request) {
	var req instanceUploadSettingsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	instance, err := s.deps.Instance.SetUploadSettings(r.Context(), req.AllowedMimeTypes, req.UploadLimits)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, instance)
}
