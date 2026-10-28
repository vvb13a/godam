package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/vvb13a/godam/api"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/store"
)

func TestAttributeMetadataAPI(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "attributes.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	assets := service.NewAssetService(nil, db, nil, nil, nil, nil, nil)
	instances := service.NewInstanceService(db, nil)
	if _, err := instances.Ensure(t.Context(), "Test"); err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	server := httptest.NewServer(api.NewServer(api.Deps{
		Assets:   assets,
		Instance: instances,
	}))
	defer server.Close()
	client := server.Client()

	now := time.Now().UTC()
	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db,
		"a1",
		[]domain.AssetAttribute{{Key: "model", Value: "news"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}

	code, body := request(t, client, http.MethodPut, server.URL+"/api/attributes/metadata/label", map[string]any{
		"key": "model", "value": "news", "label": "Newsroom",
	})
	if code != http.StatusNoContent {
		t.Fatalf("set label: %d %s", code, body)
	}
	code, body = request(t, client, http.MethodPut, server.URL+"/api/attributes/metadata/sort", map[string]any{
		"key": "model", "sort": domain.AttributeSortABCAsc,
	})
	if code != http.StatusNoContent {
		t.Fatalf("set sort: %d %s", code, body)
	}
	code, body = request(t, client, http.MethodPut, server.URL+"/api/attributes/metadata/render", map[string]any{
		"key": "model", "render": domain.AttributeRenderTree,
	})
	if code != http.StatusNoContent {
		t.Fatalf("set render: %d %s", code, body)
	}

	code, body = request(t, client, http.MethodGet, server.URL+"/api/attributes", nil)
	if code != http.StatusOK {
		t.Fatalf("list attributes: %d %s", code, body)
	}
	var response struct {
		Attributes []struct {
			Key    string `json:"key"`
			Label  string `json:"label"`
			Sort   string `json:"sort"`
			Render string `json:"render"`
			Values []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			} `json:"values"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(response.Attributes) != 1 {
		t.Fatalf("attributes = %+v", response.Attributes)
	}
	facet := response.Attributes[0]
	if facet.Key != "model" || facet.Label != "model" {
		t.Fatalf("key metadata = %+v", facet)
	}
	if facet.Sort != domain.AttributeSortABCAsc {
		t.Fatalf("sort = %q", facet.Sort)
	}
	if facet.Render != domain.AttributeRenderTree {
		t.Fatalf("render = %q", facet.Render)
	}
	if len(facet.Values) != 1 || facet.Values[0].Label != "Newsroom" {
		t.Fatalf("values = %+v", facet.Values)
	}
}

func TestAttributeCatalogAndAssetSelectionAPI(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "attribute-selection.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	assets := service.NewAssetService(nil, db, nil, nil, nil, nil, nil)
	instances := service.NewInstanceService(db, nil)
	if _, err := instances.Ensure(t.Context(), "Test"); err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	server := httptest.NewServer(api.NewServer(api.Deps{
		Assets:   assets,
		Instance: instances,
	}))
	defer server.Close()
	client := server.Client()

	now := time.Now().UTC()
	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db,
		"a1",
		[]domain.AssetAttribute{{Key: "model", Value: "news"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}

	code, body := request(t, client, http.MethodPost, server.URL+"/api/attributes/metadata/values", map[string]any{
		"key": "model", "values": []string{"sports", "weather"},
	})
	if code != http.StatusNoContent {
		t.Fatalf("create values: %d %s", code, body)
	}

	code, body = request(t, client, http.MethodGet, server.URL+"/api/attributes/catalog", nil)
	if code != http.StatusOK {
		t.Fatalf("catalog: %d %s", code, body)
	}
	var catalog struct {
		Attributes []struct {
			Key    string `json:"key"`
			Values []struct {
				Value string `json:"value"`
				Count int64  `json:"count"`
			} `json:"values"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalog.Attributes) != 1 {
		t.Fatalf("catalog = %+v", catalog)
	}
	values := make(map[string]int64)
	for _, value := range catalog.Attributes[0].Values {
		values[value.Value] = value.Count
	}
	if values["news"] != 1 || values["sports"] != 0 || values["weather"] != 0 {
		t.Fatalf("catalog values = %+v", values)
	}

	code, body = request(t, client, http.MethodPut, server.URL+"/api/assets/a1/attributes", map[string]any{
		"key": "model", "values": []string{"sports"},
	})
	if code != http.StatusOK {
		t.Fatalf("set asset attributes: %d %s", code, body)
	}
	var asset struct {
		Attributes []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(body, &asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	if len(asset.Attributes) != 1 || asset.Attributes[0].Key != "model" || asset.Attributes[0].Value != "sports" {
		t.Fatalf("asset attributes = %+v", asset.Attributes)
	}
}
