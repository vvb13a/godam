package api

import (
	"net/url"
	"testing"

	"github.com/vvb13a/godam/domain"
)

func TestApplyScopeToAssetFilterDefaultsAndLocks(t *testing.T) {
	scope := domain.Scope{
		DefaultFilters: domain.ScopeFilters{
			CollectionIDs: []string{"default-collection"},
			TagIDs:        []string{"default-tag"},
			Types:         []string{"image"},
			MimeTypes:     []string{"image/png"},
			Search:        "brand",
			Sort:          "name-asc",
		},
		LockedFilters:    []string{"collection", "search"},
		AllowedMimeTypes: []string{"image/*"},
		CanUpload:        true,
	}

	// The caller tries to override a locked collection and search, and does not
	// supply tag/type/mime/sort, so those defaults fill in.
	query := url.Values{
		"collection_ids": {"attacker-collection"},
		"search":         {"other"},
	}
	filter := domain.AssetFilter{
		CollectionIDs: []string{"attacker-collection"},
		Search:        "other",
	}
	applyScopeToAssetFilter(&filter, scope, query)

	if len(filter.CollectionIDs) != 1 || filter.CollectionIDs[0] != "default-collection" {
		t.Fatalf("locked collection was not enforced: %v", filter.CollectionIDs)
	}
	if filter.Search != "brand" {
		t.Fatalf("locked search was not enforced: %q", filter.Search)
	}
	if len(filter.TagIDs) != 1 || filter.TagIDs[0] != "default-tag" {
		t.Fatalf("tag default was not applied: %v", filter.TagIDs)
	}
	if len(filter.Types) != 1 || filter.Types[0] != "image" {
		t.Fatalf("type default was not applied: %v", filter.Types)
	}
	if len(filter.MimeTypes) != 1 || filter.MimeTypes[0] != "image/png" {
		t.Fatalf("mime default was not applied: %v", filter.MimeTypes)
	}
	if filter.Sort != "name-asc" {
		t.Fatalf("sort default was not applied: %q", filter.Sort)
	}
	if len(filter.AllowedMimeTypes) != 1 || filter.AllowedMimeTypes[0] != "image/*" {
		t.Fatalf("allowed mime was not applied: %v", filter.AllowedMimeTypes)
	}
}

func TestApplyScopeToAssetFilterAllowsUnlockedOverrides(t *testing.T) {
	scope := domain.Scope{
		DefaultFilters: domain.ScopeFilters{
			CollectionIDs: []string{"default-collection"},
			Types:         []string{"image"},
		},
	}
	query := url.Values{
		"collection_ids": {"chosen-collection"},
		"types":          {"video"},
	}
	filter := domain.AssetFilter{
		CollectionIDs: []string{"chosen-collection"},
		Types:         []string{"video"},
	}
	applyScopeToAssetFilter(&filter, scope, query)

	if filter.CollectionIDs[0] != "chosen-collection" {
		t.Fatalf("unlocked collection override was lost: %v", filter.CollectionIDs)
	}
	if filter.Types[0] != "video" {
		t.Fatalf("unlocked type override was lost: %v", filter.Types)
	}
}
