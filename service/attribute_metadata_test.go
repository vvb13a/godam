package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
)

func TestAttributeMetadataLabelsAndSort(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	asset := store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		asset.ID,
		[]domain.AssetAttribute{{Key: "model", Value: "news"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}

	if err := assets.SetAttributeLabel(ctx, "model", "", "Content model"); err != nil {
		t.Fatalf("set key label: %v", err)
	}
	if err := assets.SetAttributeLabel(ctx, "model", "news", "Newsroom"); err != nil {
		t.Fatalf("set value label: %v", err)
	}
	if err := assets.SetAttributeSort(ctx, "model", domain.AttributeSortABCAsc); err != nil {
		t.Fatalf("set sort: %v", err)
	}
	if err := assets.SetAttributeRender(ctx, "model", domain.AttributeRenderTags); err != nil {
		t.Fatalf("set render: %v", err)
	}

	facets, err := assets.AttributeFacets(ctx)
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	if len(facets) != 1 {
		t.Fatalf("facets = %+v", facets)
	}
	facet := facets[0]
	if facet.Key != "model" || facet.Label != "Content model" {
		t.Fatalf("key metadata = %+v", facet)
	}
	if facet.Sort != domain.AttributeSortABCAsc {
		t.Fatalf("sort = %q", facet.Sort)
	}
	if facet.Render != domain.AttributeRenderTags {
		t.Fatalf("render = %q", facet.Render)
	}
	if len(facet.Values) != 1 || facet.Values[0].Label != "Newsroom" {
		t.Fatalf("value metadata = %+v", facet.Values)
	}

	if err := assets.SetAttributeLabel(ctx, "model", "news", ""); err != nil {
		t.Fatalf("clear value label: %v", err)
	}
	facets, err = assets.AttributeFacets(ctx)
	if err != nil {
		t.Fatalf("facets after clear: %v", err)
	}
	if facets[0].Values[0].Label != "news" {
		t.Fatalf("value label fallback = %q", facets[0].Values[0].Label)
	}
}

func TestAttributeSortValidation(t *testing.T) {
	assets := &AssetService{db: newTestDB(t)}
	ctx := context.Background()

	if err := assets.SetAttributeSort(ctx, "model", "sideways"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid sort error = %v", err)
	}
	if err := assets.SetAttributeSort(ctx, "", domain.AttributeSortOpen); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty key error = %v", err)
	}
	if err := assets.SetAttributeRender(ctx, "model", "carousel"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid render error = %v", err)
	}
}

func TestAttributeCatalogIncludesMetadataValues(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a1",
		[]domain.AssetAttribute{{Key: "model", Value: "news"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}
	if err := assets.AddAttributeValues(ctx, "model", []string{"sports", "weather"}); err != nil {
		t.Fatalf("add values: %v", err)
	}

	catalog, err := assets.AttributeCatalog(ctx)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(catalog) != 1 {
		t.Fatalf("catalog = %+v", catalog)
	}
	values := make(map[string]int64)
	for _, value := range catalog[0].Values {
		values[value.Value] = value.Count
	}
	if values["news"] != 1 || values["sports"] != 0 || values["weather"] != 0 {
		t.Fatalf("catalog values = %+v", values)
	}
}

func TestSetAssetAttributeValues(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a1",
		[]domain.AssetAttribute{{Key: "model", Value: "news"}, {Key: "kind", Value: "logo"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}

	asset, err := assets.SetAssetAttributeValues(ctx, "a1", "model", []string{"sports", "sports", ""})
	if err != nil {
		t.Fatalf("set values: %v", err)
	}
	values := make(map[string]string)
	for _, attr := range asset.Attributes {
		values[attr.Key] = attr.Value
	}
	if values["model"] != "sports" || values["kind"] != "logo" {
		t.Fatalf("attributes = %+v", asset.Attributes)
	}

	if _, err := assets.SetAssetAttributeValues(ctx, "a1", "model", nil); err != nil {
		t.Fatalf("clear values: %v", err)
	}
	asset, err = assets.GetByID(ctx, "a1")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	for _, attr := range asset.Attributes {
		if attr.Key == "model" {
			t.Fatalf("model should be cleared: %+v", asset.Attributes)
		}
	}
}

func TestSetAssetAttributeValuesHonorsLockedScope(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := domain.WithActor(context.Background(), &domain.Actor{
		Token: true,
		Scope: domain.Scope{Attributes: []domain.AttributeGrant{
			{Key: "model", Mode: domain.AttributeModeLocked, Values: []string{"news"}},
		}},
	})
	now := time.Now().UTC()
	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := assets.SetAssetAttributeValues(ctx, "a1", "model", []string{"sports"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("locked key error = %v", err)
	}
}

func TestAttributeSearchRespectsSearchableMetadata(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a1",
		[]domain.AssetAttribute{{Key: "brand", Value: "zzbrandnamezz"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}

	search := func() int {
		page, err := assets.List(ctx, domain.AssetFilter{Search: "zzbrandnamezz"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		return len(page.Items)
	}
	if count := search(); count != 0 {
		t.Fatalf("unsearchable attribute matched %d assets", count)
	}
	if err := assets.SetAttributeSearchable(ctx, "brand", true); err != nil {
		t.Fatalf("enable search: %v", err)
	}
	if count := search(); count != 1 {
		t.Fatalf("searchable attribute matched %d assets", count)
	}
	if err := assets.SetAttributeSearchable(ctx, "brand", false); err != nil {
		t.Fatalf("disable search: %v", err)
	}
	if count := search(); count != 0 {
		t.Fatalf("disabled attribute matched %d assets", count)
	}
}

func TestAttributeUnassignedFacetAndFilter(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	for _, id := range []string{"a1", "a2"} {
		if err := db.Create(&store.Asset{
			ID: id, Filename: id + ".png", StorageDriver: "local", StorageKey: id,
			MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create asset %s: %v", id, err)
		}
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a1",
		[]domain.AssetAttribute{{Key: "model", Value: "news"}},
	); err != nil {
		t.Fatalf("replace a1: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a2",
		[]domain.AssetAttribute{{Key: "kind", Value: "logo"}},
	); err != nil {
		t.Fatalf("replace a2: %v", err)
	}
	if err := assets.SetAttributeUnassigned(ctx, "model", true); err != nil {
		t.Fatalf("include unassigned: %v", err)
	}

	facets, err := assets.AttributeFacets(ctx)
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	var unassigned int64 = -1
	for _, facet := range facets {
		if facet.Key != "model" {
			continue
		}
		if !facet.IncludeUnassigned {
			t.Fatal("model should include unassigned")
		}
		for _, value := range facet.Values {
			if value.Value == domain.AttributeUnassigned {
				unassigned = value.Count
			}
		}
	}
	if unassigned != 1 {
		t.Fatalf("unassigned count = %d", unassigned)
	}

	page, err := assets.List(ctx, domain.AssetFilter{
		Attributes: map[string][]string{"model": {domain.AttributeUnassigned}},
	})
	if err != nil {
		t.Fatalf("list unassigned: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "a2" {
		t.Fatalf("unassigned assets = %+v", page.Items)
	}

	page, err = assets.List(ctx, domain.AssetFilter{
		Attributes: map[string][]string{"model": {"news", domain.AttributeUnassigned}},
	})
	if err != nil {
		t.Fatalf("list mixed: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("mixed assets = %+v", page.Items)
	}
}

func TestDeleteAttributeValueAndKey(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a1",
		[]domain.AssetAttribute{
			{Key: "model", Value: "news"},
			{Key: "kind", Value: "logo"},
		},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}
	if err := assets.DeleteAttributeValue(ctx, "model", "news"); err != nil {
		t.Fatalf("delete value: %v", err)
	}
	asset, err := assets.GetByID(ctx, "a1")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	for _, attr := range asset.Attributes {
		if attr.Key == "model" {
			t.Fatalf("model should be deleted: %+v", asset.Attributes)
		}
	}
	if err := assets.DeleteAttributeKey(ctx, "kind"); err != nil {
		t.Fatalf("delete key: %v", err)
	}
	asset, err = assets.GetByID(ctx, "a1")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if len(asset.Attributes) != 0 {
		t.Fatalf("attributes should be empty: %+v", asset.Attributes)
	}
}

func TestApplyAssetAttributesBulk(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	for _, id := range []string{"a1", "a2"} {
		if err := db.Create(&store.Asset{
			ID: id, Filename: id + ".png", StorageDriver: "local", StorageKey: id,
			MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create asset %s: %v", id, err)
		}
		if err := store.ReplaceAssetAttributes(
			db.WithContext(ctx),
			id,
			[]domain.AssetAttribute{{Key: "kind", Value: "logo"}},
		); err != nil {
			t.Fatalf("replace %s: %v", id, err)
		}
	}

	if err := assets.CreateAttribute(ctx, "model", ""); err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := assets.ApplyAssetAttributesBulk(
		ctx, []string{"a1", "a2"}, "model", []string{"news"}, "add",
	); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := assets.ApplyAssetAttributesBulk(
		ctx, []string{"a1", "a2"}, "model", []string{"sports"}, "add",
	); err != nil {
		t.Fatalf("add second: %v", err)
	}
	for _, id := range []string{"a1", "a2"} {
		asset, err := assets.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		values := make(map[string]bool)
		for _, attr := range asset.Attributes {
			values[attr.Key+":"+attr.Value] = true
		}
		if !values["kind:logo"] || !values["model:news"] || !values["model:sports"] {
			t.Fatalf("%s attributes = %+v", id, asset.Attributes)
		}
	}

	if err := assets.ApplyAssetAttributesBulk(
		ctx, []string{"a1"}, "model", []string{"weather"}, "replace",
	); err != nil {
		t.Fatalf("replace: %v", err)
	}
	asset, err := assets.GetByID(ctx, "a1")
	if err != nil {
		t.Fatalf("get a1: %v", err)
	}
	for _, attr := range asset.Attributes {
		if attr.Key == "model" && attr.Value != "weather" {
			t.Fatalf("replace left %+v", asset.Attributes)
		}
	}
}

func TestAttributeUnassignedLabel(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := store.ReplaceAssetAttributes(
		db.WithContext(ctx),
		"a1",
		[]domain.AssetAttribute{{Key: "model", Value: "news"}},
	); err != nil {
		t.Fatalf("replace attributes: %v", err)
	}
	if err := assets.SetAttributeUnassigned(ctx, "model", true); err != nil {
		t.Fatalf("include unassigned: %v", err)
	}
	if err := assets.SetAttributeUnassignedLabel(ctx, "model", "No model"); err != nil {
		t.Fatalf("label: %v", err)
	}
	facets, err := assets.AttributeFacets(ctx)
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	for _, value := range facets[0].Values {
		if value.Value == domain.AttributeUnassigned {
			if value.Label != "No model" {
				t.Fatalf("label = %q", value.Label)
			}
			return
		}
	}
	t.Fatalf("missing unassigned value: %+v", facets[0].Values)
}

func TestAttributeTreeNestsChildren(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	attrs := map[string][]domain.AssetAttribute{
		"a1": {{Key: "site", Value: "a"}, {Key: "model", Value: "news"}},
		"a2": {{Key: "site", Value: "a"}, {Key: "model", Value: "sports"}},
		"a3": {{Key: "site", Value: "b"}, {Key: "model", Value: "news"}},
	}
	for id, list := range attrs {
		if err := db.Create(&store.Asset{
			ID: id, Filename: id + ".png", StorageDriver: "local", StorageKey: id,
			MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		if err := store.ReplaceAssetAttributes(db.WithContext(ctx), id, list); err != nil {
			t.Fatalf("attributes %s: %v", id, err)
		}
	}
	if err := assets.SetAttributeParent(ctx, "model", "site"); err != nil {
		t.Fatalf("set parent: %v", err)
	}

	trees, err := assets.AttributeTree(ctx)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	var site *domain.AttributeTree
	for i := range trees {
		if trees[i].Key == "site" {
			site = &trees[i]
		}
	}
	if site == nil {
		t.Fatalf("missing site tree: %+v", trees)
	}
	if len(site.Values) != 2 {
		t.Fatalf("site values = %+v", site.Values)
	}
	for _, value := range site.Values {
		if value.Value != "a" {
			continue
		}
		if value.Count != 2 || len(value.Children) != 2 {
			t.Fatalf("site a = %+v", value)
		}
		counts := make(map[string]int64)
		for _, child := range value.Children {
			if child.Key != "model" {
				t.Fatalf("child key = %q", child.Key)
			}
			counts[child.Value] = child.Count
		}
		if counts["news"] != 1 || counts["sports"] != 1 {
			t.Fatalf("child counts = %+v", counts)
		}
	}
}

func TestAttributeTreeRejectsCycle(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.InsertAttributeValues(db.WithContext(ctx), "a", []string{"1"}, now); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if err := store.InsertAttributeValues(db.WithContext(ctx), "b", []string{"1"}, now); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	if err := assets.SetAttributeParent(ctx, "a", "b"); err != nil {
		t.Fatalf("set a parent: %v", err)
	}
	if err := assets.SetAttributeParent(ctx, "b", "a"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestAttributeValuesLockEnforced(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := assets.CreateAttribute(ctx, "model", ""); err != nil {
		t.Fatalf("create attribute: %v", err)
	}
	if err := assets.AddAttributeValues(ctx, "model", []string{"news"}); err != nil {
		t.Fatalf("add news: %v", err)
	}
	if err := assets.SetAttributeValuesLocked(ctx, "model", true); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := assets.AddAttributeValues(ctx, "model", []string{"sports"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("locked add error = %v", err)
	}
	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := assets.SetAssetAttributeValues(ctx, "a1", "model", []string{"sports"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("locked assign error = %v", err)
	}
	if _, err := assets.SetAssetAttributeValues(ctx, "a1", "model", []string{"news"}); err != nil {
		t.Fatalf("locked existing value: %v", err)
	}
	if err := assets.SetAttributeValuesLocked(ctx, "model", false); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := assets.AddAttributeValues(ctx, "model", []string{"sports"}); err != nil {
		t.Fatalf("unlocked add: %v", err)
	}
}

func TestAttributeSingleValueEnforced(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	if err := assets.CreateAttribute(ctx, "model", ""); err != nil {
		t.Fatalf("create attribute: %v", err)
	}
	if err := assets.SetAttributeMulti(ctx, "model", false); err != nil {
		t.Fatalf("set single: %v", err)
	}
	if err := db.Create(&store.Asset{
		ID: "a1", Filename: "a1.png", StorageDriver: "local", StorageKey: "a1",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if _, err := assets.SetAssetAttributeValues(ctx, "a1", "model", []string{"news", "sports"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("multi assign error = %v", err)
	}
	if _, err := assets.SetAssetAttributeValues(ctx, "a1", "model", []string{"news"}); err != nil {
		t.Fatalf("single assign: %v", err)
	}
	if err := assets.ApplyAssetAttributesBulk(ctx, []string{"a1"}, "model", []string{"sports"}, "add"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bulk single add error = %v", err)
	}
}

func TestUploadAttributeStampingPolicy(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.Create(&store.Instance{
		ID: "i1", Name: "Test", Color: "f97316",
		UploadAttributeKeys: true, UploadAttributeValues: true,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if err := assets.validateUploadAttributes(ctx, []domain.AssetAttribute{
		{Key: "model", Value: "news"},
	}); err != nil {
		t.Fatalf("default stamping: %v", err)
	}
	if err := assets.CreateAttribute(ctx, "model", ""); err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := assets.SetAttributeMulti(ctx, "model", false); err != nil {
		t.Fatalf("set single: %v", err)
	}
	if err := assets.validateUploadAttributes(ctx, []domain.AssetAttribute{
		{Key: "model", Value: "news"}, {Key: "model", Value: "sports"},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("single upload error = %v", err)
	}
	if err := db.Model(&store.Instance{}).Where("id = ?", "i1").
		Updates(map[string]any{"upload_attribute_keys": false, "upload_attribute_values": false}).Error; err != nil {
		t.Fatalf("disable stamping: %v", err)
	}
	if err := assets.validateUploadAttributes(ctx, []domain.AssetAttribute{
		{Key: "country", Value: "de"},
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("new key upload error = %v", err)
	}
	if err := assets.validateUploadAttributes(ctx, []domain.AssetAttribute{
		{Key: "model", Value: "sports"},
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("new value upload error = %v", err)
	}
}

func TestAttributeDescriptionsColorsAndOrder(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()

	if err := assets.CreateAttribute(ctx, "model", "Model"); err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := assets.CreateAttribute(ctx, "site", "Site"); err != nil {
		t.Fatalf("create site: %v", err)
	}
	if err := assets.AddAttributeValues(ctx, "model", []string{"news", "sports", "weather"}); err != nil {
		t.Fatalf("add values: %v", err)
	}
	if err := assets.SetAttributeDescription(ctx, "model", "", "Content model"); err != nil {
		t.Fatalf("key description: %v", err)
	}
	if err := assets.SetAttributeDescription(ctx, "model", "news", "Newsroom"); err != nil {
		t.Fatalf("value description: %v", err)
	}
	if err := assets.SetAttributeColor(ctx, "model", "", "8b5cf6"); err != nil {
		t.Fatalf("key color: %v", err)
	}
	if err := assets.SetAttributeColor(ctx, "model", "sports", "10b981"); err != nil {
		t.Fatalf("value color: %v", err)
	}
	if err := assets.SetAttributeSort(ctx, "model", domain.AttributeSortCustom); err != nil {
		t.Fatalf("custom sort: %v", err)
	}
	if err := assets.SetAttributeValueOrder(ctx, "model", []string{"weather", "sports", "news"}); err != nil {
		t.Fatalf("value order: %v", err)
	}
	if err := assets.SetAttributeKeyOrder(ctx, []string{"model", "site"}); err != nil {
		t.Fatalf("key order: %v", err)
	}

	facets, err := assets.AttributeCatalog(ctx)
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(facets) < 2 || facets[0].Key != "model" || facets[1].Key != "site" {
		t.Fatalf("facet order = %+v", facets)
	}
	model := facets[0]
	if model.Description != "Content model" || model.Color != "8b5cf6" {
		t.Fatalf("key metadata = %+v", model)
	}
	if len(model.Values) != 3 {
		t.Fatalf("model values = %+v", model.Values)
	}
	positions := make(map[string]int)
	for _, value := range model.Values {
		positions[value.Value] = value.Position
		switch value.Value {
		case "news":
			if value.Description != "Newsroom" {
				t.Fatalf("news description = %q", value.Description)
			}
		case "sports":
			if value.Color != "10b981" {
				t.Fatalf("sports color = %q", value.Color)
			}
		}
	}
	if positions["weather"] != 0 || positions["sports"] != 1 || positions["news"] != 2 {
		t.Fatalf("custom positions = %+v", positions)
	}
}
