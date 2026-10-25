package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vvb13a/godam/domain"
)

func TestWithCapability(t *testing.T) {
	server := &Server{}
	next := server.withCapability(domain.CapabilityAssetsDelete, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/assets/a1", nil)
	req = req.WithContext(domain.WithActor(req.Context(), &domain.Actor{
		ID:           "token-user",
		Token:        true,
		Capabilities: []string{"assets:*"},
	}))
	res := httptest.NewRecorder()
	next(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("assets:* status = %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/assets/a1", nil)
	req = req.WithContext(domain.WithActor(context.Background(), &domain.Actor{
		ID:           "token-user",
		Token:        true,
		Capabilities: []string{domain.CapabilityAssetsRead},
	}))
	res = httptest.NewRecorder()
	next(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("missing capability status = %d", res.Code)
	}
}
