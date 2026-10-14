package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
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
	deps := api.Deps{
		Collections: service.NewCollectionService(db),
		Libraries:   service.NewLibraryService(db, nil),
		Users:       service.NewUserService(db),
	}
	server := httptest.NewServer(api.NewServer(deps))
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return server, &http.Client{Jar: jar}
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

	if code, _ := request(t, client, http.MethodGet, server.URL+"/api/libraries", nil); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 before login, got %d", code)
	}
	if code, _ := request(t, client, http.MethodPost, server.URL+"/api/auth/dev-login", nil); code != http.StatusOK {
		t.Fatalf("dev login status %d", code)
	}

	code, body := request(t, client, http.MethodPost, server.URL+"/api/libraries", map[string]any{
		"name": "L", "color": "", "sort": 0,
	})
	if code != http.StatusCreated {
		t.Fatalf("create library: %d %s", code, body)
	}
	var lib struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &lib)

	createCollection := func(name string, parent *string) string {
		payload := map[string]any{"library_id": lib.ID, "name": name, "color": "8b5cf6", "parent_id": parent}
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

	_, body = request(t, client, http.MethodGet, server.URL+"/api/collections?library_id="+lib.ID, nil)
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
		"library_id": lib.ID, "name": "X", "color": "", "parent_id": missing,
	})
	if code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing parent, got %d", code)
	}
}
