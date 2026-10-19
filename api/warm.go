package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vvb13a/godam/domain"
)

type warmRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

type warmItem struct {
	assetID string
	preset  domain.Preset
}

type warmEvent struct {
	Type             string `json:"type"`
	TotalAssets      int    `json:"total_assets,omitempty"`
	TotalConversions int    `json:"total_conversions,omitempty"`
	Skipped          int    `json:"skipped,omitempty"`
	AssetID          string `json:"asset_id,omitempty"`
	Preset           string `json:"preset,omitempty"`
	Done             int    `json:"done,omitempty"`
	Failed           int    `json:"failed,omitempty"`
	Message          string `json:"message,omitempty"`
}

// handleWarmPresets renders every preset allowed for each requested asset and
// streams progress as Server-Sent Events. The plan is built before the stream
// starts so the client immediately knows both totals.
func (s *Server) handleWarmPresets(w http.ResponseWriter, r *http.Request) {
	var req warmRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.AssetIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset_ids is required"})
		return
	}
	if len(req.AssetIDs) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many assets (max 500)"})
		return
	}

	seen := make(map[string]bool, len(req.AssetIDs))
	items := make([]warmItem, 0, len(req.AssetIDs))
	totalAssets := 0
	skipped := 0
	for _, raw := range req.AssetIDs {
		assetID := strings.TrimSpace(raw)
		if assetID == "" || seen[assetID] {
			continue
		}
		seen[assetID] = true

		asset, err := s.deps.Assets.GetByID(r.Context(), assetID)
		if err != nil || !strings.HasPrefix(asset.MimeType, "image/") {
			skipped++
			continue
		}
		allowed, err := s.deps.Presets.AllowedForAsset(r.Context(), assetID)
		if err != nil || len(allowed) == 0 {
			skipped++
			continue
		}
		totalAssets++
		for _, preset := range allowed {
			items = append(items, warmItem{assetID: assetID, preset: preset})
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	send := func(event warmEvent) bool {
		data, err := json.Marshal(event)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !send(warmEvent{
		Type:             "start",
		TotalAssets:      totalAssets,
		TotalConversions: len(items),
		Skipped:          skipped,
	}) {
		return
	}

	done, failed := 0, 0
	for _, item := range items {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		if err := s.deps.Conversions.WarmPreset(r.Context(), item.assetID, item.preset); err != nil {
			failed++
			if !send(warmEvent{
				Type:             "error",
				AssetID:          item.assetID,
				Preset:           item.preset.Name,
				Done:             done,
				Failed:           failed,
				TotalConversions: len(items),
				Message:          err.Error(),
			}) {
				return
			}
			continue
		}

		done++
		if !send(warmEvent{
			Type:             "progress",
			AssetID:          item.assetID,
			Preset:           item.preset.Name,
			Done:             done,
			Failed:           failed,
			TotalConversions: len(items),
		}) {
			return
		}
	}

	if s.deps.Stats != nil {
		s.deps.Stats.Invalidate()
	}
	_ = send(warmEvent{
		Type:             "done",
		TotalAssets:      totalAssets,
		TotalConversions: len(items),
		Skipped:          skipped,
		Done:             done,
		Failed:           failed,
	})
}
