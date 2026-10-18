package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vvb13a/godam/domain"
)

// handleListAssets returns one cursor-paginated slice of assets. Search matches
// title, note and filename; type filters by media category; sort chooses the
// keyset order.
func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.AssetFilter{
		Search: q.Get("search"),
		Type:   q.Get("type"),
		Color:  q.Get("color"),
		Sort:   q.Get("sort"),
		Cursor: q.Get("cursor"),
	}
	if value := q.Get("limit"); value != "" {
		if limit, err := strconv.Atoi(value); err == nil {
			filter.Limit = limit
		}
	}
	if v := q.Get("collection_id"); v != "" {
		filter.CollectionID = &v
	}
	for _, raw := range []string{q.Get("tag_ids"), q.Get("tag_id")} {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				filter.TagIDs = append(filter.TagIDs, id)
			}
		}
	}
	filter.Uncategorized = q.Get("uncategorized") == "true"
	filter.Untagged = q.Get("untagged") == "true"

	page, err := s.deps.Assets.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := s.ownedAsset(w, r, r.PathValue("id"))
	if !ok {
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

	asset, err := s.deps.Assets.Upload(r.Context(), currentUser(r).ID, r.FormValue("driver"), header.Filename, mimeType, header.Size, file)
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
	asset, err := s.deps.Assets.UploadFromSource(r.Context(), currentUser(r).ID, req.Source, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ownedAsset(w, r, r.PathValue("id")); !ok {
		return
	}
	if err := s.deps.Assets.Delete(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateAssetRequest struct {
	Title string `json:"title"`
	Note  string `json:"note"`
}

// handleUpdateAsset updates the editable title and note of an asset and
// responds with the refreshed asset.
func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	var req updateAssetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	asset, err := s.deps.Assets.Update(r.Context(), id, req.Title, req.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

// handleReplaceAssetContent overwrites an asset's stored file (for example
// after an in-browser edit), regenerating its preview and metadata. The body is
// a multipart form with a single "file" field.
func (s *Server) handleReplaceAssetContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
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

	asset, err := s.deps.Assets.ReplaceContent(r.Context(), id, mimeType, file)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

// handleAssetContent streams the original stored file.
func (s *Server) handleAssetContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	asset, ok := s.ownedAsset(w, r, id)
	if !ok {
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
	// Media players need range requests (and a known length) to show duration
	// and seek. ServeContent handles Range/Content-Length for seekable readers
	// (local storage); other drivers fall back to a plain stream.
	if seeker, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(w, r, asset.Filename, time.Time{}, seeker)
		return
	}
	_, _ = io.Copy(w, rc)
}

// handleDownloadAssetsZip streams the selected assets as a zip archive. All
// ids are ownership-checked before the response starts so errors are reported
// as JSON rather than a truncated archive.
func (s *Server) handleDownloadAssetsZip(w http.ResponseWriter, r *http.Request) {
	var ids []string
	for _, part := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if id := strings.TrimSpace(part); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids is required"})
		return
	}

	names := make([]string, 0, len(ids))
	for _, id := range ids {
		asset, ok := s.ownedAsset(w, r, id)
		if !ok {
			return
		}
		names = append(names, asset.Filename)
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="godam-assets.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	used := make(map[string]int, len(names))
	for i, id := range ids {
		rc, err := s.deps.Assets.Open(r.Context(), id)
		if err != nil {
			continue
		}
		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name:   uniqueArchiveName(names[i], used),
			Method: zip.Store,
		})
		if err != nil {
			_ = rc.Close()
			continue
		}
		_, _ = io.Copy(entry, rc)
		_ = rc.Close()
	}
}

// uniqueArchiveName appends " (2)", " (3)"… to repeated filenames so every zip
// entry is unique.
func uniqueArchiveName(name string, used map[string]int) string {
	base, ext := name, ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}
	count := used[name]
	used[name] = count + 1
	if count == 0 {
		return name
	}
	return fmt.Sprintf("%s (%d)%s", base, count+1, ext)
}

// handleAssetPreview streams the generated JPEG preview.
func (s *Server) handleAssetPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	rc, contentType, err := s.deps.Assets.OpenPreview(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, rc)
}

// handleRegeneratePreview re-renders and stores the preview for an asset.
func (s *Server) handleRegeneratePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	asset, err := s.deps.Assets.RegeneratePreview(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

type tagRequest struct {
	Name  string `json:"name"`
	TagID string `json:"tag_id"`
}

func (s *Server) handleAddTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	var req tagRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TagID != "" {
		if err := s.deps.Assets.AddTagByID(r.Context(), id, req.TagID); err != nil {
			writeError(w, err)
			return
		}
		s.respondAsset(w, r, id)
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
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
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
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
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
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	if err := s.deps.Assets.RemoveFromCollection(r.Context(), id, r.PathValue("collectionId")); err != nil {
		writeError(w, err)
		return
	}
	s.respondAsset(w, r, id)
}

type bulkCollectionRequest struct {
	CollectionID string   `json:"collection_id"`
	AssetIDs     []string `json:"asset_ids"`
}

// handleAddAssetsToCollection links many assets to one collection in a single
// request, replacing the per-asset POST /api/assets/{id}/collections calls.
func (s *Server) handleAddAssetsToCollection(w http.ResponseWriter, r *http.Request) {
	var req bulkCollectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.CollectionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		return
	}
	if _, ok := s.ownedCollection(w, r, req.CollectionID); !ok {
		return
	}
	if err := s.deps.Assets.AddToCollectionBulk(r.Context(), req.CollectionID, req.AssetIDs); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type bulkTagRequest struct {
	AssetIDs []string `json:"asset_ids"`
	TagIDs   []string `json:"tag_ids"`
	Mode     string   `json:"mode"`
}

// handleApplyAssetsTags applies a tag set to many assets in one request,
// replacing the per-asset/tag POST/DELETE tag calls. "add" merges the tags into
// each asset; "replace" makes tag_ids the exact tag set of every asset.
func (s *Server) handleApplyAssetsTags(w http.ResponseWriter, r *http.Request) {
	var req bulkTagRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Mode != "add" && req.Mode != "replace" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be add or replace"})
		return
	}
	var err error
	if req.Mode == "replace" {
		err = s.deps.Assets.ReplaceTagsBulk(r.Context(), req.AssetIDs, req.TagIDs)
	} else {
		err = s.deps.Assets.AddTagsBulk(r.Context(), req.AssetIDs, req.TagIDs)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
