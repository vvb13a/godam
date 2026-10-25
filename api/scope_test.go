package api

import (
	"net/url"
	"testing"

	"github.com/vvb13a/godam/domain"
)

func TestApplyScopeToAssetFilterEnforcesLockedOnly(t *testing.T) {
	scope := domain.Scope{Filters: map[string]domain.FilterGrant{
		"collection": {Values: []string{"locked-collection"}, Mode: domain.ScopeModeLocked},
		"tag":        {Values: []string{"default-tag"}, Mode: domain.ScopeModeActive},
		"type":       {Values: []string{"image"}, Mode: domain.ScopeModeActive},
		"mime":       {Values: []string{"image/png"}, Mode: domain.ScopeModeActive},
		"search":     {Values: []string{"brand"}, Mode: domain.ScopeModeLocked},
		"sort":       {Values: []string{"name-asc"}, Mode: domain.ScopeModeActive},
	}}

	filter := domain.AssetFilter{
		CollectionIDs: []string{"attacker-collection"},
		TagIDs:        []string{"chosen-tag"},
		Types:         []string{"video"},
		MimeTypes:     []string{"video/mp4"},
		Search:        "other",
		Sort:          "date-asc",
	}
	applyScopeToAssetFilter(&filter, scope)

	if len(filter.CollectionIDs) != 1 || filter.CollectionIDs[0] != "locked-collection" {
		t.Fatalf("locked collection was not enforced: %v", filter.CollectionIDs)
	}
	if filter.Search != "brand" {
		t.Fatalf("locked search was not enforced: %q", filter.Search)
	}
	if len(filter.TagIDs) != 1 || filter.TagIDs[0] != "chosen-tag" {
		t.Fatalf("active tag default should not override the request: %v", filter.TagIDs)
	}
	if len(filter.Types) != 1 || filter.Types[0] != "video" {
		t.Fatalf("active type default should not override the request: %v", filter.Types)
	}
	if len(filter.MimeTypes) != 1 || filter.MimeTypes[0] != "video/mp4" {
		t.Fatalf("active mime default should not override the request: %v", filter.MimeTypes)
	}
	if filter.Sort != "date-asc" {
		t.Fatalf("active sort default should not override the request: %q", filter.Sort)
	}
}

func TestApplyScopeActiveFiltersAreNotRestored(t *testing.T) {
	scope := domain.Scope{Filters: map[string]domain.FilterGrant{
		"collection": {Values: []string{"default-collection"}, Mode: domain.ScopeModeActive},
		"type":       {Values: []string{"image"}, Mode: domain.ScopeModeActive},
	}}
	filter := domain.AssetFilter{}
	applyScopeToAssetFilter(&filter, scope)
	if len(filter.CollectionIDs) != 0 || len(filter.Types) != 0 {
		t.Fatalf("active defaults were restored after the client cleared them: %+v", filter)
	}
}

func TestApplyScopeLockedCollectionRejectsUncategorized(t *testing.T) {
	scope := domain.Scope{Filters: map[string]domain.FilterGrant{
		"collection": {Values: []string{"collection-1"}, Mode: domain.ScopeModeLocked},
	}}
	filter := domain.AssetFilter{Uncategorized: true}
	applyScopeToAssetFilter(&filter, scope)
	if filter.Uncategorized {
		t.Fatal("uncategorized must not broaden a locked collection")
	}
	if len(filter.CollectionIDs) != 1 || filter.CollectionIDs[0] != "collection-1" {
		t.Fatalf("locked collection was not enforced: %v", filter.CollectionIDs)
	}
}

func TestApplyScopeHiddenUncategorizedFilter(t *testing.T) {
	scope := domain.Scope{Filters: map[string]domain.FilterGrant{
		"uncategorized": {Values: []string{"true"}, Mode: domain.ScopeModeHidden},
	}}
	filter := domain.AssetFilter{}
	applyScopeToAssetFilter(&filter, scope)
	if !filter.Uncategorized {
		t.Fatal("hidden uncategorized filter was not applied")
	}
}

func TestApplyAttributeScope(t *testing.T) {
	scope := domain.Scope{Attributes: []domain.AttributeGrant{
		{Key: "tenant", Values: []string{"acme"}, Mode: domain.AttributeModeHidden},
		{Key: "model", Values: []string{"News"}, Mode: domain.AttributeModeActive},
		{Key: "kind", Values: []string{"logo"}, Mode: domain.AttributeModeLocked},
	}}
	filter := domain.AssetFilter{Attributes: map[string][]string{
		"tenant": {"beta"},
		"other":  {"x"},
	}}
	if err := applyAttributeScope(&filter, scope); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(filter.Attributes["tenant"]) != 1 || filter.Attributes["tenant"][0] != "acme" {
		t.Fatalf("hidden attribute should be forced: %v", filter.Attributes["tenant"])
	}
	if len(filter.Attributes["kind"]) != 1 || filter.Attributes["kind"][0] != "logo" {
		t.Fatalf("locked attribute should be forced: %v", filter.Attributes["kind"])
	}
	if _, ok := filter.Attributes["model"]; ok {
		t.Fatalf("active attribute default was restored: %v", filter.Attributes["model"])
	}
	if _, ok := filter.Attributes["other"]; ok {
		t.Fatal("unknown attribute key should be dropped")
	}

	// Active attributes are defaults, not allow-lists, and can be changed.
	custom := domain.AssetFilter{Attributes: map[string][]string{"model": {"Sports"}}}
	if err := applyAttributeScope(&custom, scope); err != nil {
		t.Fatalf("active attribute override: %v", err)
	}
	if len(custom.Attributes["model"]) != 1 || custom.Attributes["model"][0] != "Sports" {
		t.Fatalf("active attribute override was restricted: %v", custom.Attributes["model"])
	}
}

func TestDeniedFilterName(t *testing.T) {
	denied := domain.Scope{Filters: map[string]domain.FilterGrant{
		"search": {Mode: domain.ScopeModeDenied},
	}}
	if got := deniedFilterName(denied, url.Values{"search": {"brand"}}); got != "search" {
		t.Fatalf("denied search = %q", got)
	}
	if got := deniedFilterName(denied, url.Values{"search": {""}}); got != "" {
		t.Fatalf("empty search should be allowed: %q", got)
	}

	empty := domain.Scope{}
	if got := deniedFilterName(empty, url.Values{"types": {"image"}}); got != "type" {
		t.Fatalf("absent filter should be denied, got %q", got)
	}

	wildcard := domain.Scope{AllFilters: true}
	if got := deniedFilterName(wildcard, url.Values{"types": {"image"}}); got != "" {
		t.Fatalf("wildcard should allow type, got %q", got)
	}
}
