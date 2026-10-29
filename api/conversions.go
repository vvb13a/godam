package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/vvb13a/godam/service"
)

// serveConversion renders segment for sourceAssetID. scopeAssetID is the asset
// the caller references: it verifies signed segments and scopes presets, while
// the actual pixels may come from a derived visual asset.
func (s *Server) serveConversion(w http.ResponseWriter, r *http.Request, scopeAssetID, sourceAssetID, segment string) {
	var (
		result *service.ConversionResult
		err    error
	)
	switch {
	case strings.HasPrefix(segment, "~"):
		result, err = s.deps.Conversions.ConvertSignedSource(r.Context(), scopeAssetID, sourceAssetID, segment)
	default:
		result, err = s.deps.Conversions.ConvertPresetSource(r.Context(), scopeAssetID, sourceAssetID, segment)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	s.writeConversionResult(w, result)
}

func (s *Server) writeConversionResult(w http.ResponseWriter, result *service.ConversionResult) {
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", s.cacheControl("public"))
	if result.CacheHit {
		w.Header().Set("X-Godam-Conversion", "HIT")
	} else {
		w.Header().Set("X-Godam-Conversion", "MISS")
	}
	_, _ = io.Copy(w, result.Reader)
}

// handlePurgeConversions removes every generated conversion. Originals and
// preset definitions are untouched.
func (s *Server) handlePurgeConversions(w http.ResponseWriter, r *http.Request) {
	removed, err := s.deps.Conversions.Purge(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if s.deps.Stats != nil {
		s.deps.Stats.Invalidate()
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}

// handleListConversions returns the stored conversions for an asset, sorted by
// size by the service. This is an authenticated library route.
func (s *Server) handleListConversions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	conversions, err := s.deps.Conversions.List(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversions)
}

// handleConversionSource streams an original asset to imgproxy. It is
// registered outside the user-auth boundary on purpose: imgproxy fetches it
// over the internal network, and Caddy blocks /internal/* on the public site.
func (s *Server) handleConversionSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	asset, err := s.deps.Assets.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	rc, err := s.deps.Assets.Open(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, rc)
}
