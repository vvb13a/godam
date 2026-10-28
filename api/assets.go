package api

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	filter := domain.AssetFilter{
		Search:         q.Get("search"),
		Sort:           q.Get("sort"),
		Cursor:         q.Get("cursor"),
		Types:          append(parseCSV(q.Get("types")), parseCSV(q.Get("type"))...),
		Colors:         append(parseCSV(q.Get("colors")), parseCSV(q.Get("color"))...),
		MimeTypes:      append(parseCSV(q.Get("mime_types")), parseCSV(q.Get("mime"))...),
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

	if raw := q.Get("attributes"); raw != "" {
		attributes, err := parseAttributeFilters(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid attributes filter"})
			return
		}
		filter.Attributes = attributes
	}

	if scope, ok := requestScope(r); ok {
		if denied := deniedFilterName(scope, q); denied != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "filter not allowed: " + denied,
			})
			return
		}
		applyScopeToAssetFilter(&filter, scope)
		if err := applyAttributeScope(&filter, scope); err != nil {
			writeError(w, err)
			return
		}
	}

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
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		value, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		if endOfDay && layout == "2006-01-02" {
			value = value.Add(24 * time.Hour)
		}
		return &value
	}
	return nil
}

// filterRequested reports whether a request supplies a canonical filter.
func filterRequested(name string, q url.Values) bool {
	switch name {
	case "type":
		return q.Get("types") != "" || q.Get("type") != ""
	case "mime":
		return q.Get("mime_types") != "" || q.Get("mime") != ""
	case "search":
		return strings.TrimSpace(q.Get("search")) != ""
	case "sort":
		return strings.TrimSpace(q.Get("sort")) != ""
	case "color":
		return q.Get("colors") != "" || q.Get("color") != ""
	case "size":
		return q.Get("min_size") != "" || q.Get("max_size") != ""
	case "duration":
		return q.Get("min_duration") != "" || q.Get("max_duration") != ""
	case "pages":
		return q.Get("min_pages") != "" || q.Get("max_pages") != ""
	case "dimensions":
		return q.Get("min_width") != "" || q.Get("max_width") != "" ||
			q.Get("min_height") != "" || q.Get("max_height") != ""
	case "added":
		return q.Get("added_after") != "" || q.Get("added_before") != ""
	case "modified":
		return q.Get("modified_after") != "" || q.Get("modified_before") != ""
	}
	return false
}

func deniedFilterName(scope domain.Scope, q url.Values) string {
	for _, name := range domain.FilterNames {
		if scope.FilterDenied(name) && filterRequested(name, q) {
			return name
		}
	}
	return ""
}

// applyScopeToAssetFilter applies the JWT scope to a listing. Only locked and
// hidden filters are enforced server-side. Active values are defaults for the
// embed UI, so the client can clear them without the server restoring them.
func applyScopeToAssetFilter(filter *domain.AssetFilter, scope domain.Scope) {
	if typeFilter, _ := scope.Filter("type"); scope.Locked("type") {
		filter.Types = append([]string{}, typeFilter.Values...)
	}

	if mimeFilter, _ := scope.Filter("mime"); scope.Locked("mime") {
		filter.MimeTypes = append([]string{}, mimeFilter.Values...)
	}

	if search, ok := scope.Filter("search"); scope.Locked("search") && ok && len(search.Values) > 0 {
		filter.Search = strings.TrimSpace(search.Values[0])
	}
	if sortFilter, ok := scope.Filter("sort"); scope.Locked("sort") && ok && len(sortFilter.Values) > 0 {
		filter.Sort = strings.TrimSpace(sortFilter.Values[0])
	}
}

func parseAttributeFilters(raw string) (map[string][]string, error) {
	var values map[string][]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(values))
	for key, list := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		seen := make(map[string]bool, len(list))
		cleaned := make([]string, 0, len(list))
		for _, value := range list {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			cleaned = append(cleaned, value)
		}
		if len(cleaned) > 0 {
			out[key] = cleaned
		}
	}
	return out, nil
}

// applyAttributeScope applies token attributes. Only locked and hidden
// attributes are enforced server-side. Active values are defaults for the embed
// UI, so the client can clear them without the server restoring them. Attribute
// keys not present in the token are dropped.
func applyAttributeScope(filter *domain.AssetFilter, scope domain.Scope) error {
	if filter.Attributes == nil {
		filter.Attributes = make(map[string][]string)
	}
	grants := make(map[string]domain.AttributeGrant, len(scope.Attributes))
	for _, grant := range scope.Attributes {
		grants[grant.Key] = grant
	}
	if !scope.AllAttributes {
		for key := range filter.Attributes {
			if _, ok := grants[key]; !ok {
				delete(filter.Attributes, key)
			}
		}
	}
	for _, grant := range scope.Attributes {
		if grant.Mode == domain.AttributeModeLocked || grant.Mode == domain.AttributeModeHidden {
			filter.Attributes[grant.Key] = append([]string{}, grant.Values...)
		}
	}
	return nil
}

// handleListAttributes returns the attribute facets available to the current
// actor for filtering.
func (s *Server) handleListAttributes(w http.ResponseWriter, r *http.Request) {
	facets, err := s.deps.Assets.AttributeFacets(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attributes": facets})
}

type assetAttributesBulkRequest struct {
	AssetIDs []string `json:"asset_ids"`
	Key      string   `json:"key"`
	Values   []string `json:"values"`
	Mode     string   `json:"mode"`
}

// handleApplyAssetsAttributes applies one attribute key to many assets.
func (s *Server) handleApplyAssetsAttributes(w http.ResponseWriter, r *http.Request) {
	var req assetAttributesBulkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	for _, id := range req.AssetIDs {
		if _, ok := s.ownedAsset(w, r, id); !ok {
			return
		}
	}
	if err := s.deps.Assets.ApplyAssetAttributesBulk(
		r.Context(), req.AssetIDs, req.Key, req.Values, req.Mode,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListAttributeCatalog returns usage facets plus metadata-only values for
// keys already in use. Used by the attributes dialog and inspector.
func (s *Server) handleListAttributeCatalog(w http.ResponseWriter, r *http.Request) {
	facets, err := s.deps.Assets.AttributeCatalog(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attributes": facets})
}

type attributeValuesRequest struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// handleAddAttributeValues creates metadata-only values for an existing key.
func (s *Server) handleAddAttributeValues(w http.ResponseWriter, r *http.Request) {
	var req attributeValuesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.AddAttributeValues(
		r.Context(), req.Key, req.Values,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSetAssetAttributes replaces the values of one attribute key on an
// asset. The actor also needs a write-capable attribute grant for the key.
func (s *Server) handleSetAssetAttributes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.ownedAsset(w, r, id); !ok {
		return
	}
	var req attributeValuesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	asset, err := s.deps.Assets.SetAssetAttributeValues(
		r.Context(), id, req.Key, req.Values,
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

type createAttributeRequest struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func (s *Server) handleCreateAttribute(w http.ResponseWriter, r *http.Request) {
	var req createAttributeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.CreateAttribute(r.Context(), req.Key, req.Label); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeValuesLockRequest struct {
	Key    string `json:"key"`
	Locked bool   `json:"locked"`
}

func (s *Server) handleSetAttributeValuesLocked(w http.ResponseWriter, r *http.Request) {
	var req attributeValuesLockRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeValuesLocked(r.Context(), req.Key, req.Locked); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeLabelRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Label string `json:"label"`
}

// handleSetAttributeLabel overrides the display label for a key or value. An
// empty value targets the key-level label; the stored attribute is unchanged.
func (s *Server) handleSetAttributeLabel(w http.ResponseWriter, r *http.Request) {
	var req attributeLabelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeLabel(
		r.Context(), req.Key, req.Value, req.Label,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeSortRequest struct {
	Key  string `json:"key"`
	Sort string `json:"sort"`
}

// handleSetAttributeSort sets the key-level ordering setting.
func (s *Server) handleSetAttributeSort(w http.ResponseWriter, r *http.Request) {
	var req attributeSortRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeSort(
		r.Context(), req.Key, req.Sort,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeRenderRequest struct {
	Key    string `json:"key"`
	Render string `json:"render"`
}

// handleSetAttributeRender sets the key-level rendering setting.
func (s *Server) handleSetAttributeRender(w http.ResponseWriter, r *http.Request) {
	var req attributeRenderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeRender(
		r.Context(), req.Key, req.Render,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeSearchableRequest struct {
	Key        string `json:"key"`
	Searchable bool   `json:"searchable"`
}

func (s *Server) handleSetAttributeSearchable(w http.ResponseWriter, r *http.Request) {
	var req attributeSearchableRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeSearchable(
		r.Context(), req.Key, req.Searchable,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeMultiRequest struct {
	Key   string `json:"key"`
	Multi bool   `json:"multi"`
}

func (s *Server) handleSetAttributeMulti(w http.ResponseWriter, r *http.Request) {
	var req attributeMultiRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeMulti(
		r.Context(), req.Key, req.Multi,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeUnassignedRequest struct {
	Key               string `json:"key"`
	IncludeUnassigned bool   `json:"include_unassigned"`
}

func (s *Server) handleSetAttributeUnassigned(w http.ResponseWriter, r *http.Request) {
	var req attributeUnassignedRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeUnassigned(
		r.Context(), req.Key, req.IncludeUnassigned,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeUnassignedLabelRequest struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

func (s *Server) handleSetAttributeUnassignedLabel(w http.ResponseWriter, r *http.Request) {
	var req attributeUnassignedLabelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeUnassignedLabel(
		r.Context(), req.Key, req.Label,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeDescriptionRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

func (s *Server) handleSetAttributeDescription(w http.ResponseWriter, r *http.Request) {
	var req attributeDescriptionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeDescription(
		r.Context(), req.Key, req.Value, req.Description,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeColorRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Color string `json:"color"`
}

func (s *Server) handleSetAttributeColor(w http.ResponseWriter, r *http.Request) {
	var req attributeColorRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeColor(
		r.Context(), req.Key, req.Value, req.Color,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeValueOrderRequest struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func (s *Server) handleSetAttributeValueOrder(w http.ResponseWriter, r *http.Request) {
	var req attributeValueOrderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeValueOrder(
		r.Context(), req.Key, req.Values,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeKeyOrderRequest struct {
	Keys []string `json:"keys"`
}

func (s *Server) handleSetAttributeKeyOrder(w http.ResponseWriter, r *http.Request) {
	var req attributeKeyOrderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeKeyOrder(r.Context(), req.Keys); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeValueSearchRequest struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

func (s *Server) handleSetAttributeValueSearch(w http.ResponseWriter, r *http.Request) {
	var req attributeValueSearchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeValueSearch(
		r.Context(), req.Key, req.Enabled,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeParentRequest struct {
	Key    string `json:"key"`
	Parent string `json:"parent"`
}

func (s *Server) handleSetAttributeParent(w http.ResponseWriter, r *http.Request) {
	var req attributeParentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.SetAttributeParent(
		r.Context(), req.Key, req.Parent,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAttributeTree returns nested attribute trees for the sidebar.
func (s *Server) handleAttributeTree(w http.ResponseWriter, r *http.Request) {
	trees, err := s.deps.Assets.AttributeTree(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trees": trees})
}

type attributeDeleteValueRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s *Server) handleDeleteAttributeValue(w http.ResponseWriter, r *http.Request) {
	var req attributeDeleteValueRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.DeleteAttributeValue(
		r.Context(), req.Key, req.Value,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type attributeDeleteKeyRequest struct {
	Key string `json:"key"`
}

func (s *Server) handleDeleteAttributeKey(w http.ResponseWriter, r *http.Request) {
	var req attributeDeleteKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Assets.DeleteAttributeKey(
		r.Context(), req.Key,
	); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

	asset, err := s.deps.Assets.Upload(r.Context(), r.FormValue("driver"), header.Filename, mimeType, header.Size, file)
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
		currentActor(r).ID,
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
		currentActor(r).ID,
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
	asset, err := s.deps.Assets.CompleteUpload(r.Context(), r.PathValue("id"), currentActor(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, asset)
}

// handleCancelUpload discards an in-progress upload.
func (s *Server) handleCancelUpload(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Assets.CancelUpload(r.PathValue("id"), currentActor(r).ID); err != nil {
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
