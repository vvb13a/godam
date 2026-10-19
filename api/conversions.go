package api

import (
	"io"
	"net/http"

	"github.com/vvb13a/godam/service"
)

// handleConversion renders or serves a cached conversion. The route is
// intentionally unauthenticated for now; presets and authorization will be
// layered on before it is exposed as a public API.
func (s *Server) handleConversion(w http.ResponseWriter, r *http.Request) {
	spec, err := service.ParseConversionSpec(r.PathValue("spec"))
	if err != nil {
		writeError(w, err)
		return
	}

	result, err := s.deps.Conversions.Convert(r.Context(), r.PathValue("id"), spec)
	if err != nil {
		writeError(w, err)
		return
	}
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", service.ConversionCacheControl)
	if result.CacheHit {
		w.Header().Set("X-Godam-Conversion", "HIT")
	} else {
		w.Header().Set("X-Godam-Conversion", "MISS")
	}
	_, _ = io.Copy(w, result.Reader)
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
