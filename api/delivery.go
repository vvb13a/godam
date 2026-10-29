package api

import (
	"fmt"
	"io"
	"mime"
	"net/http"
)

// handleImageDelivery is the visual delivery route. It resolves a parent asset
// to its image source (the asset itself for images, the derived visual asset
// for audio/video/pdf) and then delegates to the normal imgproxy conversion
// paths.
func (s *Server) handleImageDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	source, err := s.deps.Assets.ResolveDeliveryImage(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	segment := r.PathValue("spec")
	if segment == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a preset or signed spec is required"})
		return
	}
	s.serveConversion(w, r, id, source.ID, segment)
}

// handleSourceDelivery streams originals for audio/video/pdf. Images and
// derived visual assets are intentionally excluded; they are delivered via
// /img.
func (s *Server) handleSourceDelivery(w http.ResponseWriter, r *http.Request) {
	asset, err := s.deps.Assets.ResolveSourceAsset(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	rc, err := s.deps.Assets.Open(r.Context(), asset.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Cache-Control", s.cacheControl("public"))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	if value := mime.FormatMediaType(disposition, map[string]string{"filename": asset.Filename}); value != "" {
		w.Header().Set("Content-Disposition", value)
	}

	if seeker, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(w, r, asset.Filename, asset.UpdatedAt, seeker)
		return
	}
	if asset.ByteSize > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", asset.ByteSize))
	}
	_, _ = io.Copy(w, rc)
}
