package service

import (
	"context"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
)

func TestAssetListPagination(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	fixtures := []store.Asset{
		{ID: "a1", Filename: "alpha.txt", Title: "Needle", MimeType: "text/plain", ByteSize: 10, CreatedAt: base, UpdatedAt: base},
		{ID: "a2", Filename: "beta.jpg", Note: "Hidden haystack", MimeType: "image/jpeg", ByteSize: 20, CreatedAt: base, UpdatedAt: base.Add(time.Second)},
		{ID: "a3", Filename: "gamma.pdf", MimeType: "application/pdf", ByteSize: 30, CreatedAt: base.Add(2 * time.Second), UpdatedAt: base.Add(2 * time.Second)},
		{ID: "a4", Filename: "delta.mp4", MimeType: "video/mp4", ByteSize: 40, CreatedAt: base.Add(3 * time.Second), UpdatedAt: base.Add(3 * time.Second)},
		{ID: "a5", Filename: "epsilon.mp3", MimeType: "audio/mpeg", ByteSize: 50, CreatedAt: base.Add(4 * time.Second), UpdatedAt: base.Add(4 * time.Second)},
	}
	for i := range fixtures {
		fixtures[i].StorageDriver = "local"
		fixtures[i].StorageKey = fixtures[i].ID
		if err := db.Create(&fixtures[i]).Error; err != nil {
			t.Fatalf("create fixture %s: %v", fixtures[i].ID, err)
		}
	}

	collect := func(filter domain.AssetFilter) ([]string, *domain.AssetPage) {
		t.Helper()
		var names []string
		var first *domain.AssetPage
		seen := map[string]bool{}
		for {
			page, err := assets.List(ctx, filter)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if first == nil {
				first = page
			}
			for _, asset := range page.Items {
				if seen[asset.ID] {
					t.Fatalf("asset %s returned twice", asset.ID)
				}
				seen[asset.ID] = true
				names = append(names, asset.Filename)
			}
			if page.NextCursor == "" {
				break
			}
			filter.Cursor = page.NextCursor
		}
		return names, first
	}

	names, page := collect(domain.AssetFilter{Sort: "name-asc", Limit: 2})
	want := []string{"alpha.txt", "beta.jpg", "delta.mp4", "epsilon.mp3", "gamma.pdf"}
	if len(names) != len(want) {
		t.Fatalf("name sort names = %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("name sort names = %v", names)
		}
	}
	if page.Total != 5 || page.TotalBytes != 150 || page.TotalCapped {
		t.Fatalf("unexpected page metadata: %+v", page)
	}

	names, _ = collect(domain.AssetFilter{Sort: "size-asc", Limit: 2})
	want = []string{"alpha.txt", "beta.jpg", "gamma.pdf", "delta.mp4", "epsilon.mp3"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("size sort names = %v", names)
		}
	}

	names, _ = collect(domain.AssetFilter{Sort: "date-desc", Limit: 2})
	want = []string{"epsilon.mp3", "delta.mp4", "gamma.pdf", "beta.jpg", "alpha.txt"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("date sort names = %v", names)
		}
	}

	for _, tc := range []struct {
		term string
		want string
	}{
		{"needle", "alpha.txt"},
		{"haystack", "beta.jpg"},
		{"GAMMA", "gamma.pdf"},
	} {
		page, err := assets.List(ctx, domain.AssetFilter{Search: tc.term})
		if err != nil {
			t.Fatalf("search %q: %v", tc.term, err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Filename != tc.want {
			t.Fatalf("search %q = %+v", tc.term, page)
		}
	}

	for _, tc := range []struct {
		kind string
		want int64
	}{
		{"image", 1},
		{"video", 1},
		{"audio", 1},
		{"document", 2},
		{"other", 0},
	} {
		page, err := assets.List(ctx, domain.AssetFilter{Types: []string{tc.kind}})
		if err != nil {
			t.Fatalf("type %q: %v", tc.kind, err)
		}
		if page.Total != tc.want {
			t.Fatalf("type %q total = %d, want %d", tc.kind, page.Total, tc.want)
		}
	}
}

func TestAssetListTotalCap(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()

	for _, bounds := range [][2]int{{1, 501}, {502, 1002}} {
		if err := db.Exec(`
			WITH RECURSIVE seq(n) AS (
				SELECT ? UNION ALL SELECT n + 1 FROM seq WHERE n < ?
			)
			INSERT INTO assets (
				id, uploaded_by, filename, storage_driver, storage_key,
				mime_type, byte_size, created_at, updated_at
			)
			SELECT lower(hex(randomblob(16))), '', printf('cap-%04d.txt', n),
			       'local', '', 'text/plain', 1,
			       '2026-01-01 00:00:00', '2026-01-01 00:00:00'
			FROM seq`,
			bounds[0], bounds[1],
		).Error; err != nil {
			t.Fatalf("seed cap rows: %v", err)
		}
	}

	page, err := assets.List(ctx, domain.AssetFilter{Limit: 1, Sort: "name-asc"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 1000 || !page.TotalCapped {
		t.Fatalf("total = %d capped=%v, want 1000/true", page.Total, page.TotalCapped)
	}
	if page.TotalBytes != 1002 {
		t.Fatalf("total bytes = %d, want 1002", page.TotalBytes)
	}
	if page.NextCursor == "" {
		t.Fatal("expected a next cursor")
	}
}

func TestAssetSearchFTSSync(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	for _, asset := range []store.Asset{
		{ID: "s1", Filename: "alpha.txt", Title: "100% coverage", Note: `quote"star`, MimeType: "text/plain", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "s2", Filename: "beta.txt", MimeType: "text/plain", ByteSize: 1, CreatedAt: now, UpdatedAt: now},
	} {
		asset.StorageDriver = "local"
		asset.StorageKey = asset.ID
		if err := db.Create(&asset).Error; err != nil {
			t.Fatalf("create %s: %v", asset.ID, err)
		}
	}

	count := func(term string) int64 {
		t.Helper()
		page, err := assets.List(ctx, domain.AssetFilter{Search: term})
		if err != nil {
			t.Fatalf("search %q: %v", term, err)
		}
		return page.Total
	}

	for term, want := range map[string]int64{
		"lph":        1,
		"100%":       1,
		`quote"star`: 1,
		"be":         1,
	} {
		if got := count(term); got != want {
			t.Fatalf("search %q total = %d, want %d", term, got, want)
		}
	}

	if err := db.Model(&store.Asset{}).Where("id = ?", "s1").Updates(map[string]any{
		"title":      "Renamed",
		"updated_at": now.Add(time.Second),
	}).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := count("renamed"); got != 1 {
		t.Fatalf("updated title search = %d, want 1", got)
	}
	if got := count("coverage"); got != 0 {
		t.Fatalf("stale title search = %d, want 0", got)
	}

	if err := db.Delete(&store.Asset{}, "id = ?", "s1").Error; err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := count("renamed"); got != 0 {
		t.Fatalf("deleted asset search = %d, want 0", got)
	}

	if err := db.Create(&store.Asset{
		ID: "s3", Filename: "fresh.txt", Title: "Fresh start",
		StorageDriver: "local", StorageKey: "s3", MimeType: "text/plain",
		ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create fresh: %v", err)
	}
	if got := count("fresh"); got != 1 {
		t.Fatalf("inserted asset search = %d, want 1", got)
	}
}

func TestAssetListColorFilter(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []string{"color-red", "color-blue", "color-mixed"} {
		if err := db.Create(&store.Asset{
			ID: id, Filename: id + ".png", StorageDriver: "local", StorageKey: id,
			MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	assets.saveColors(ctx, "color-red", []domain.AssetColor{{Hex: "#ff0000", Bucket: "red", Percentage: 100}})
	assets.saveColors(ctx, "color-blue", []domain.AssetColor{{Hex: "#0000ff", Bucket: "blue", Percentage: 100}})
	assets.saveColors(ctx, "color-mixed", []domain.AssetColor{
		{Hex: "#fadc04", Bucket: "yellow", Percentage: 63.7},
		{Hex: "#fbdc17", Bucket: "yellow", Percentage: 11.8},
		{Hex: "#5f7844", Bucket: "green", Percentage: 6.3},
		{Hex: "#465a2a", Bucket: "green", Percentage: 5.0},
		{Hex: "#403008", Bucket: "brown", Percentage: 3.8},
		{Hex: "#f0d252", Bucket: "yellow", Percentage: 3.4},
		{Hex: "#889a69", Bucket: "green", Percentage: 2.9},
		{Hex: "#171305", Bucket: "black", Percentage: 1.6},
		{Hex: "#9c7f0b", Bucket: "yellow", Percentage: 1.4},
	})

	for bucket, want := range map[string]string{"red": "color-red", "blue": "color-blue", "green": "color-mixed", "yellow": "color-mixed"} {
		page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{bucket}})
		if err != nil {
			t.Fatalf("filter %s: %v", bucket, err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != want {
			t.Fatalf("filter %s = %+v", bucket, page.Items)
		}
	}
	for _, bucket := range []string{"black", "brown"} {
		if page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{bucket}}); err != nil || page.Total != 0 {
			t.Fatalf("%s filter = %+v err=%v", bucket, page, err)
		}
	}
	if page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{"red", "blue"}}); err != nil || page.Total != 2 {
		t.Fatalf("red OR blue filter = %+v err=%v", page, err)
	}
}

func TestAssetListColorFilterAggregatesBucketPercentages(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []string{"split", "tiny"} {
		if err := db.Create(&store.Asset{
			ID: id, Filename: id + ".png", StorageDriver: "local", StorageKey: id,
			MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	assets.saveColors(ctx, "split", []domain.AssetColor{
		{Hex: "#ee0000", Bucket: "red", Percentage: 5},
		{Hex: "#dd0000", Bucket: "red", Percentage: 5},
	})
	assets.saveColors(ctx, "tiny", []domain.AssetColor{
		{Hex: "#cc0000", Bucket: "red", Percentage: 4},
	})

	page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{"red"}})
	if err != nil {
		t.Fatalf("filter red: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != "split" {
		t.Fatalf("red filter = %+v", page.Items)
	}
}

func TestAssetListColorFilterIgnoresMinorRedShare(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.Create(&store.Asset{
		ID: "pale", Filename: "pale.png", StorageDriver: "local", StorageKey: "pale",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create pale: %v", err)
	}
	assets.saveColors(ctx, "pale", []domain.AssetColor{
		{Hex: "#f8f6fa", Bucket: "white", Percentage: 66.3},
		{Hex: "#eae7f2", Bucket: "white", Percentage: 14.3},
		{Hex: "#e0dae8", Bucket: "gray", Percentage: 9.8},
		{Hex: "#ae8b87", Bucket: "red", Percentage: 2.8},
		{Hex: "#cfc7d3", Bucket: "gray", Percentage: 2.4},
		{Hex: "#c5a5a6", Bucket: "red", Percentage: 2.2},
		{Hex: "#725356", Bucket: "red", Percentage: 1.3},
		{Hex: "#351c20", Bucket: "black", Percentage: 0.9},
	})

	if page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{"red"}}); err != nil || page.Total != 0 {
		t.Fatalf("red filter = %+v err=%v", page, err)
	}
	for _, bucket := range []string{"white", "gray"} {
		if page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{bucket}}); err != nil || page.Total != 1 {
			t.Fatalf("%s filter = %+v err=%v", bucket, page, err)
		}
	}
}

func TestAssetListColorFilterIgnoresMinorSecondaryBuckets(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.Create(&store.Asset{
		ID: "brown", Filename: "brown.png", StorageDriver: "local", StorageKey: "brown",
		MimeType: "image/png", ByteSize: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create brown: %v", err)
	}
	assets.saveColors(ctx, "brown", []domain.AssetColor{
		{Hex: "#764a1d", Bucket: "brown", Percentage: 58.9},
		{Hex: "#936b43", Bucket: "brown", Percentage: 19.1},
		{Hex: "#845a2f", Bucket: "brown", Percentage: 12.4},
		{Hex: "#ab8865", Bucket: "orange", Percentage: 4.8},
		{Hex: "#a27c57", Bucket: "orange", Percentage: 2.9},
		{Hex: "#b79573", Bucket: "orange", Percentage: 2.0},
	})

	if page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{"orange"}}); err != nil || page.Total != 0 {
		t.Fatalf("orange filter = %+v err=%v", page, err)
	}
	if page, err := assets.List(ctx, domain.AssetFilter{Colors: []string{"brown"}}); err != nil || page.Total != 1 {
		t.Fatalf("brown filter = %+v err=%v", page, err)
	}
}

func TestRecomputeColorBuckets(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	if err := db.Create(&store.Asset{
		ID: "recompute", Filename: "recompute.png", StorageDriver: "local",
		StorageKey: "recompute", MimeType: "image/png", ByteSize: 1,
	}).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	rows := []store.AssetColor{
		{AssetID: "recompute", Position: 0, Color: "#232c36", Bucket: "blue", Percentage: 50},
		{AssetID: "recompute", Position: 1, Color: "#f8f6fa", Bucket: "purple", Percentage: 50},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create colors: %v", err)
	}
	assets.recomputeColorBuckets(ctx)
	var stored []store.AssetColor
	if err := db.Where("asset_id = ?", "recompute").Order("position").Find(&stored).Error; err != nil {
		t.Fatalf("load colors: %v", err)
	}
	if stored[0].Bucket != "black" || stored[1].Bucket != "white" {
		t.Fatalf("buckets = %q, %q", stored[0].Bucket, stored[1].Bucket)
	}
}

func TestAssetListMultiValueAndRangeFilters(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u1, u2 := "u1", "u2"
	d10, d20 := 10.0, 20.0
	p5, p10 := 5, 10
	w100, w200, w300 := 100, 200, 300
	h50, h100, h150 := 50, 100, 150

	rows := []store.Asset{
		{ID: "a1", Filename: "a1.png", UploadedBy: &u1, MimeType: "image/png", ByteSize: 100, Width: &w100, Height: &h50, DurationSec: &d10, CreatedAt: base, UpdatedAt: base},
		{ID: "a2", Filename: "a2.pdf", UploadedBy: &u2, MimeType: "application/pdf", ByteSize: 200, Width: &w200, Height: &h100, DurationSec: &d20, PageCount: &p5, CreatedAt: base.Add(24 * time.Hour), UpdatedAt: base.Add(24 * time.Hour)},
		{ID: "a3", Filename: "a3.txt", UploadedBy: &u1, MimeType: "text/plain", ByteSize: 300, Width: &w300, Height: &h150, PageCount: &p10, CreatedAt: base.Add(48 * time.Hour), UpdatedAt: base.Add(48 * time.Hour)},
	}
	for i := range rows {
		rows[i].StorageDriver = "local"
		rows[i].StorageKey = rows[i].ID
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create %s: %v", rows[i].ID, err)
		}
	}
	for _, collection := range []store.Collection{
		{ID: "c1", Name: "C1", CreatedAt: base, UpdatedAt: base},
		{ID: "c2", Name: "C2", CreatedAt: base, UpdatedAt: base},
	} {
		if err := db.Create(&collection).Error; err != nil {
			t.Fatalf("create collection: %v", err)
		}
	}
	for _, tag := range []store.Tag{
		{ID: "t1", Name: "T1", CreatedAt: base, UpdatedAt: base},
		{ID: "t2", Name: "T2", CreatedAt: base, UpdatedAt: base},
	} {
		if err := db.Create(&tag).Error; err != nil {
			t.Fatalf("create tag: %v", err)
		}
	}
	for _, link := range []struct{ asset, collection string }{{"a1", "c1"}, {"a2", "c2"}} {
		if err := db.Exec("INSERT INTO asset_collections (asset_id, collection_id) VALUES (?, ?)", link.asset, link.collection).Error; err != nil {
			t.Fatalf("link collection: %v", err)
		}
	}
	for _, link := range []struct{ asset, tag string }{{"a1", "t1"}, {"a2", "t2"}} {
		if err := db.Exec("INSERT INTO asset_tags (asset_id, tag_id) VALUES (?, ?)", link.asset, link.tag).Error; err != nil {
			t.Fatalf("link tag: %v", err)
		}
	}

	ids := func(filter domain.AssetFilter) map[string]bool {
		t.Helper()
		page, err := assets.List(ctx, filter)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		found := make(map[string]bool, len(page.Items))
		for _, asset := range page.Items {
			found[asset.ID] = true
		}
		return found
	}
	expect := func(filter domain.AssetFilter, want ...string) {
		t.Helper()
		got := ids(filter)
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for _, id := range want {
			if !got[id] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}

	expect(domain.AssetFilter{CollectionIDs: []string{"c1", "c2"}}, "a1", "a2")
	expect(domain.AssetFilter{CollectionIDs: []string{"c1"}, Uncategorized: true}, "a1", "a3")
	expect(domain.AssetFilter{TagIDs: []string{"t1", "t2"}}, "a1", "a2")
	expect(domain.AssetFilter{TagIDs: []string{"t1"}, Untagged: true}, "a1", "a3")
	expect(domain.AssetFilter{Types: []string{"image", "video"}}, "a1")
	expect(domain.AssetFilter{Types: []string{"image", "document"}}, "a1", "a2", "a3")
	expect(domain.AssetFilter{MimeTypes: []string{"image/png", "application/pdf"}}, "a1", "a2")
	expect(domain.AssetFilter{UploadedBy: []string{"u1"}}, "a1", "a3")

	minSize, maxSize := int64(150), int64(250)
	expect(domain.AssetFilter{MinSize: &minSize, MaxSize: &maxSize}, "a2")
	minDuration := 15.0
	expect(domain.AssetFilter{MinDuration: &minDuration}, "a2")
	minPages, maxPages := 5, 5
	expect(domain.AssetFilter{MinPages: &minPages, MaxPages: &maxPages}, "a2")
	minWidth, maxWidth := 150, 250
	expect(domain.AssetFilter{MinWidth: &minWidth, MaxWidth: &maxWidth}, "a2")
	maxHeight := 100
	expect(domain.AssetFilter{MaxHeight: &maxHeight}, "a1", "a2")

	addedAfter := base.Add(24 * time.Hour)
	expect(domain.AssetFilter{AddedAfter: &addedAfter}, "a2", "a3")
	addedBefore := base.Add(48 * time.Hour)
	expect(domain.AssetFilter{AddedBefore: &addedBefore}, "a1", "a2")
	modifiedAfter := base.Add(24 * time.Hour)
	expect(domain.AssetFilter{ModifiedAfter: &modifiedAfter}, "a2", "a3")
}

func TestAssetListPresetFilter(t *testing.T) {
	db := newTestDB(t)
	assets := &AssetService{db: db}
	ctx := context.Background()
	base := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)

	for _, id := range []string{"a1", "a2", "a3"} {
		asset := store.Asset{
			ID: id, Filename: id + ".jpg", MimeType: "image/jpeg", ByteSize: 1,
			StorageDriver: "local", StorageKey: id, CreatedAt: base, UpdatedAt: base,
		}
		if err := db.Create(&asset).Error; err != nil {
			t.Fatalf("create asset: %v", err)
		}
	}

	collection, err := NewCollectionService(db).Create(ctx, nil, "Campaign", "", "")
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	tag, err := NewTagService(db).GetOrCreate(ctx, "Needs thumbnail")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := db.Exec(`INSERT INTO asset_collections (asset_id, collection_id) VALUES (?, ?)`, "a1", collection.ID).Error; err != nil {
		t.Fatalf("link collection: %v", err)
	}
	if err := db.Exec(`INSERT INTO asset_tags (asset_id, tag_id) VALUES (?, ?)`, "a2", tag.ID).Error; err != nil {
		t.Fatalf("link tag: %v", err)
	}

	presets := NewPresetService(db)
	restricted, err := presets.Create(ctx, "user", "Restricted", "restricted", testPresetParameters(), false, []string{collection.ID}, []string{tag.ID})
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}

	ids := func(filter domain.AssetFilter) []string {
		t.Helper()
		page, err := assets.List(ctx, filter)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		out := make([]string, 0, len(page.Items))
		for _, asset := range page.Items {
			out = append(out, asset.ID)
		}
		return out
	}
	assertIDs := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("ids = %v, want %v", got, want)
		}
		seen := map[string]bool{}
		for _, id := range got {
			seen[id] = true
		}
		for _, id := range want {
			if !seen[id] {
				t.Fatalf("ids = %v, want %v", got, want)
			}
		}
	}

	assertIDs(ids(domain.AssetFilter{PresetID: restricted.ID, Sort: "name-asc"}), "a1", "a2")
	assertIDs(ids(domain.AssetFilter{PresetID: restricted.ID, PresetExclude: true, Sort: "name-asc"}), "a3")

	global, err := presets.Create(ctx, "user", "Global", "global", testPresetParameters(), true, nil, nil)
	if err != nil {
		t.Fatalf("create global preset: %v", err)
	}
	assertIDs(ids(domain.AssetFilter{PresetID: global.ID, Sort: "name-asc"}), "a1", "a2", "a3")
}
