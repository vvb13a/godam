package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/service"
)

// handleListAssets returns one cursor-paginated slice of assets. Search matches
// title, note and filename; type filters by media category; sort chooses the
// keyset order.
func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	presetID := q.Get("preset_id")
	if presetID == "" {
		// Allow ?preset=<name> for shareable URLs; the service filter works on ids.
		if name := q.Get("preset"); name != "" && s.deps.Presets != nil {
			if preset, err := s.deps.Presets.GetByName(r.Context(), name); err == nil {
				presetID = preset.ID
			}
		}
	}
	filter := domain.AssetFilter{
		PresetID:       presetID,
		PresetExclude:  q.Get("preset_exclude") == "true",
		Search:         q.Get("search"),
		Sort:           q.Get("sort"),
		Cursor:         q.Get("cursor"),
		CollectionIDs:  append(parseCSV(q.Get("collection_ids")), parseCSV(q.Get("collection_id"))...),
		Types:          append(parseCSV(q.Get("types")), parseCSV(q.Get("type"))...),
		Colors:         append(parseCSV(q.Get("colors")), parseCSV(q.Get("color"))...),
		MimeTypes:      append(parseCSV(q.Get("mime_types")), parseCSV(q.Get("mime"))...),
		UploadedBy:     parseCSV(q.Get("uploaded_by")),
		MinDuration:    parseFloatPtr(q.Get("min_duration")),
		MaxDuration:    parseFloatPtr(q.Get("max_duration")),
		MinPages:       parseIntPtr(q.Get("min_pages")),
		MaxPages:       parseIntPtr(q.Get("max_pages")),
		MinWidth:       parseIntPtr(q.Get("min_width")),
		MaxWidth:       parseIntPtr(q.Get("max_width")),
		MinHeight:      parseIntPtr(q.Get("min_height")),
		MaxHeight:      parseIntPtr(q.Get("max_height")),
		MinSize:        parseInt64Ptr(q.Get("min_size")),
		MaxSize:        parseInt64Ptr(q.Get("max_size")),
		AddedAfter:     parseDatePtr(q.Get("added_after"), false),
		AddedBefore:    parseDatePtr(q.Get("added_before"), true),
		ModifiedAfter:  parseDatePtr(q.Get("modified_after"), false),
		ModifiedBefore: parseDatePtr(q.Get("modified_before"), true),
	}
	if value := q.Get("limit"); value != "" {
		if limit, err := strconv.Atoi(value); err == nil {
			filter.Limit = limit
		}
	}
	for _, raw := range []string{q.Get("tag_ids"), q.Get("tag_id")} {
		filter.TagIDs = append(filter.TagIDs, parseCSV(raw)...)
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

func parseCSV(raw string) []string {
	values := make([]string, 0, 4)
	seen := make(map[string]bool, 4)
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values
}

func parseFloatPtr(raw string) *float64 {
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

func parseIntPtr(raw string) *int {
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func parseInt64Ptr(raw string) *int64 {
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

func parseDatePtr(raw string, endOfDay bool) *time.Time {
	if raw == "" {
		return nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil
	}
	if endOfDay {
		value = value.Add(24 * time.Hour)
	}
	return &value
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
	if limit := s.deps.Assets.UploadStreamLimit(r.Context()); limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid upload"})
		return
	}
	defer r.MultipartForm.RemoveAll()
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

type startUploadRequest struct {
	Driver   string `json:"driver"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// handleStartUpload opens a resumable upload session for a large file. The
// client sends sequential chunks to the returned session id and then completes
// the upload.
func (s *Server) handleStartUpload(w http.ResponseWriter, r *http.Request) {
	var req startUploadRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	session, err := s.deps.Assets.StartUpload(
		r.Context(),
		currentUser(r).ID,
		req.Driver,
		req.Filename,
		req.MimeType,
		req.Size,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

// handleUploadChunk appends one raw chunk at the given byte offset.
func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || offset < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chunk offset"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, service.UploadChunkSize+1)

	written, err := s.deps.Assets.WriteUploadChunk(
		r.Context(),
		r.PathValue("id"),
		currentUser(r).ID,
		offset,
		r.Body,
	)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "upload chunk is too large"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{
		"received": written,
		"offset":   offset + written,
	})
}

// handleCompleteUpload validates the assembled file and creates the asset.
func (s *Server) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	asset, err := s.deps.Assets.CompleteUpload(r.Context(), r.PathValue("id"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

// handleCancelUpload discards an in-progress upload.
func (s *Server) handleCancelUpload(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Assets.CancelUpload(r.PathValue("id"), currentUser(r).ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	if limit := s.deps.Assets.UploadStreamLimit(r.Context()); limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
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
		if asset.Role != domain.AssetRoleOriginal {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "visual assets cannot be downloaded"})
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

// handleAssetPreview is the authenticated preview alias. It resolves the same
// source as /img and renders the default 400px preview through imgproxy.
func (s *Server) handleAssetPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	source, err := s.deps.Assets.ResolveDeliveryImage(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.deps.Conversions.PreviewDefault(r.Context(), source.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, result.Reader)
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
	CollectionID  string   `json:"collection_id"`
	CollectionIDs []string `json:"collection_ids"`
	AssetIDs      []string `json:"asset_ids"`
	Mode          string   `json:"mode"`
}

// handleAddAssetsToCollection applies a collection set to many assets in one
// request. The legacy single collection_id form adds that collection; the
// collection_ids + mode form supports bulk add and replace.
func (s *Server) handleAddAssetsToCollection(w http.ResponseWriter, r *http.Request) {
	var req bulkCollectionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ids := req.CollectionIDs
	if req.CollectionID != "" {
		ids = append(ids, req.CollectionID)
	}
	mode := req.Mode
	if mode == "" {
		mode = "add"
	}
	if mode != "add" && mode != "replace" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be add or replace"})
		return
	}
	if mode == "add" && len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "collection_id or collection_ids is required"})
		return
	}
	for _, id := range ids {
		if _, ok := s.ownedCollection(w, r, id); !ok {
			return
		}
	}
	if err := s.deps.Assets.ApplyCollectionsBulk(r.Context(), req.AssetIDs, ids, mode); err != nil {
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
