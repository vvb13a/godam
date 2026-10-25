package service

import (
	"context"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
)

func TestAssetListFiltersByAttributes(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	rows := []store.Asset{
		{ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1", MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "a2", Filename: "a2.png", StorageDriver: "local", StorageKey: "a2", MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "a3", Filename: "a3.png", StorageDriver: "local", StorageKey: "a3", MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}
	attributeSets := map[string][]domain.AssetAttribute{
		"a1": {{Key: "tenant", Value: "acme"}, {Key: "entity", Value: "news"}, {Key: "kind", Value: "logo"}},
		"a2": {{Key: "tenant", Value: "acme"}, {Key: "entity", Value: "press"}, {Key: "kind", Value: "logo"}},
		"a3": {{Key: "tenant", Value: "beta"}, {Key: "entity", Value: "news"}, {Key: "kind", Value: "icon"}},
	}
	for id, attrs := range attributeSets {
		if err := store.ReplaceAssetAttributes(db.WithContext(ctx), id, attrs); err != nil {
			t.Fatalf("replace attributes: %v", err)
		}
	}

	ids := func(filter domain.AssetFilter) map[string]bool {
		page, err := assets.List(ctx, filter)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		out := make(map[string]bool)
		for _, asset := range page.Items {
			out[asset.ID] = true
		}
		return out
	}

	got := ids(domain.AssetFilter{Attributes: map[string][]string{"tenant": {"acme"}}})
	if !got["a1"] || !got["a2"] || got["a3"] {
		t.Fatalf("tenant filter = %v", got)
	}

	got = ids(domain.AssetFilter{
		Attributes: map[string][]string{
			"tenant": {"acme"},
			"entity": {"news", "press"},
		},
	})
	if !got["a1"] || !got["a2"] || got["a3"] {
		t.Fatalf("multi-value filter = %v", got)
	}

	got = ids(domain.AssetFilter{
		Attributes: map[string][]string{
			"tenant": {"acme"},
			"entity": {"news"},
		},
	})
	if !got["a1"] || got["a2"] || got["a3"] {
		t.Fatalf("AND across keys = %v", got)
	}
}

func TestAttributeFacetsRespectScope(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	rows := []store.Asset{
		{ID: "a1", Filename: "a1.png", MimeType: "image/png", StorageDriver: "local", StorageKey: "a1", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "a2", Filename: "a2.png", MimeType: "image/png", StorageDriver: "local", StorageKey: "a2", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "a3", Filename: "a3.png", MimeType: "image/png", StorageDriver: "local", StorageKey: "a3", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}
	attributes := map[string][]domain.AssetAttribute{
		"a1": {{Key: "tenant", Value: "acme"}, {Key: "model", Value: "News"}},
		"a2": {{Key: "tenant", Value: "acme"}, {Key: "model", Value: "Sports"}, {Key: "kind", Value: "logo"}},
		"a3": {{Key: "tenant", Value: "beta"}, {Key: "model", Value: "News"}},
	}
	for id, attrs := range attributes {
		if err := store.ReplaceAssetAttributes(db.WithContext(ctx), id, attrs); err != nil {
			t.Fatalf("replace attributes: %v", err)
		}
	}

	facets := func(actor *domain.Actor) map[string][]string {
		t.Helper()
		requestCtx := ctx
		if actor != nil {
			requestCtx = domain.WithActor(ctx, actor)
		}
		got, err := assets.AttributeFacets(requestCtx)
		if err != nil {
			t.Fatalf("facets: %v", err)
		}
		out := make(map[string][]string, len(got))
		for _, facet := range got {
			for _, value := range facet.Values {
				out[facet.Key] = append(out[facet.Key], value.Value)
			}
		}
		return out
	}

	all := facets(nil)
	for _, key := range []string{"tenant", "model", "kind"} {
		if len(all[key]) == 0 {
			t.Fatalf("direct access missing %q: %+v", key, all)
		}
	}

	none := facets(&domain.Actor{Token: true})
	if len(none) != 0 {
		t.Fatalf("token without attributes should see no facets: %+v", none)
	}

	active := facets(&domain.Actor{
		Token: true,
		Scope: domain.Scope{Attributes: []domain.AttributeGrant{
			{Key: "model", Mode: domain.AttributeModeActive},
		}},
	})
	if len(active) != 1 || len(active["model"]) != 2 {
		t.Fatalf("active model facets = %+v", active)
	}

	wildcard := facets(&domain.Actor{
		Token: true,
		Scope: domain.Scope{AllAttributes: true},
	})
	for _, key := range []string{"tenant", "model", "kind"} {
		if len(wildcard[key]) == 0 {
			t.Fatalf("wildcard missing %q: %+v", key, wildcard)
		}
	}

	locked := facets(&domain.Actor{
		Token: true,
		Scope: domain.Scope{
			AllAttributes: true,
			Attributes: []domain.AttributeGrant{
				{Key: "tenant", Values: []string{"acme"}, Mode: domain.AttributeModeLocked},
			},
		},
	})
	if _, ok := locked["tenant"]; ok {
		t.Fatalf("locked key should not be a filter facet: %+v", locked)
	}
	if len(locked["model"]) != 2 {
		t.Fatalf("locked tenant should restrict facet values: %+v", locked)
	}
}
