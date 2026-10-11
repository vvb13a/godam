package api

import (
	"io"
	"net/http"
)

// handleListAssets returns every asset, optionally filtered to a collection
// via the collection_id query parameter.
func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	var collectionID *string
	if v := r.URL.Query().Get("collection_id"); v != "" {
		collectionID = &v
	}

	assets, err := s.deps.Assets.List(r.Context(), collectionID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	asset, err := s.deps.Assets.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

// handleUploadAsset accepts a multipart form with a single "file" field and an
// optional "driver" field, storing the file through the asset service.
func (s *Server) handleUploadAsset(w http.ResponseWriter, r *http.Request) {
	if s.deps.MaxUploadBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, s.deps.MaxUploadBytes+(1<<20))
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid upload"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file"})
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		mimeType = http.DetectContentType(buf[:n])
		_, _ = file.Seek(0, 0)
	}

	asset, err := s.deps.Assets.Upload(r.Context(), r.FormValue("driver"), header.Filename, mimeType, header.Size, file)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

type importRequest struct {
	Source string `json:"source"`
}

// handleImportAsset pulls an asset from a local path or an http(s) URL.
func (s *Server) handleImportAsset(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Source == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source is required"})
		return
	}
	asset, err := s.deps.Assets.UploadFromSource(r.Context(), req.Source, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Assets.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAssetContent streams the original stored file.
func (s *Server) handleAssetContent(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, rc)
}

// handleAssetPreview streams the generated JPEG preview.
func (s *Server) handleAssetPreview(w http.ResponseWriter, r *http.Request) {
	rc, err := s.deps.Assets.OpenPreview(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, rc)
}

type tagRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleAddTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req tagRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := s.deps.Assets.AddTag(r.Context(), id, req.Name); err != nil {
		writeError(w, err)
		return
	}
	s.respondAsset(w, r, id)
}

func (s *Server) handleRemoveTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.deps.Assets.RemoveTag(r.Context(), id, r.PathValue("tagId")); err != nil {
		writeError(w, err)
		return
	}
	s.respondAsset(w, r, id)
}

type collectionRelationRequest struct {
	CollectionID string `json:"collection_id"`
}

func (s *Server) handleAddToCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req collectionRelationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.CollectionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		return
	}
	if err := s.deps.Assets.AddToCollection(r.Context(), id, req.CollectionID); err != nil {
		writeError(w, err)
		return
	}
	s.respondAsset(w, r, id)
}

func (s *Server) handleRemoveFromCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.deps.Assets.RemoveFromCollection(r.Context(), id, r.PathValue("collectionId")); err != nil {
		writeError(w, err)
		return
	}
	s.respondAsset(w, r, id)
}

// respondAsset writes the current state of an asset, so relation mutations
// return the updated membership without a follow-up request.
func (s *Server) respondAsset(w http.ResponseWriter, r *http.Request, id string) {
	asset, err := s.deps.Assets.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}
