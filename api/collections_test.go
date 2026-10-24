package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vvb13a/godam/api"
	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/store"
)

func newTestClient(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	instances := service.NewInstanceService(db, nil)
	if _, err := instances.Ensure(t.Context(), "Test"); err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	deps := api.Deps{
		Collections: service.NewCollectionService(db),
		Instance:    instances,
	}
	server := httptest.NewServer(api.NewServer(deps))
	return server, &http.Client{}
}

func request(t *testing.T, client *http.Client, method, url string, body any) (int, []byte) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	return res.StatusCode, buf.Bytes()
}

func TestCollectionTreeAPI(t *testing.T) {
	server, client := newTestClient(t)
	defer server.Close()

	code, body := request(t, client, http.MethodGet, server.URL+"/api/instance", nil)
	if code != http.StatusOK {
		t.Fatalf("get instance: %d %s", code, body)
	}

	createCollection := func(name string, parent *string) string {
		payload := map[string]any{"name": name, "color": "8b5cf6", "parent_id": parent}
		code, body := request(t, client, http.MethodPost, server.URL+"/api/collections", payload)
		if code != http.StatusCreated {
			t.Fatalf("create collection %s: %d %s", name, code, body)
		}
		var col struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &col)
		return col.ID
	}

	root := createCollection("Root", nil)
	child := createCollection("Child", &root)

	// Cycle: moving the root under its own descendant must be rejected.
	code, _ = request(t, client, http.MethodPut, server.URL+"/api/collections/"+root+"/move", map[string]any{
		"parent_id": child, "position": 0,
	})
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cycle, got %d", code)
	}

	// Reparent the child to the root level.
	code, _ = request(t, client, http.MethodPut, server.URL+"/api/collections/"+child+"/move", map[string]any{
		"parent_id": nil, "position": 0,
	})
	if code != http.StatusNoContent {
		t.Fatalf("move status %d", code)
	}

	_, body = request(t, client, http.MethodGet, server.URL+"/api/collections", nil)
	var collections []struct {
		ID       string  `json:"id"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.Unmarshal(body, &collections); err != nil {
		t.Fatalf("decode collections: %v", err)
	}
	if len(collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(collections))
	}
	for _, collection := range collections {
		if collection.ParentID != nil {
			t.Fatalf("expected all roots after move, got parent %v", *collection.ParentID)
		}
	}

	// Unknown parent is a 404.
	missing := "does-not-exist"
	code, _ = request(t, client, http.MethodPost, server.URL+"/api/collections", map[string]any{
		"name": "X", "color": "", "parent_id": missing,
	})
	if code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing parent, got %d", code)
	}
}
