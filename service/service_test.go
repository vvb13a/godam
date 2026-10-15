package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestLibraryMembership(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	users := NewUserService(db)
	libs := NewLibraryService(db, nil)

	user1, err := users.Create(ctx, "One", "one@example.com", "password1")
	if err != nil {
		t.Fatalf("create user1: %v", err)
	}
	user2, err := users.Create(ctx, "Two", "two@example.com", "password1")
	if err != nil {
		t.Fatalf("create user2: %v", err)
	}

	lib, err := libs.Create(ctx, user1.ID, "Shared", "8b5cf6", 0)
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	list, err := libs.List(ctx, user1.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("creator should see library: n=%d err=%v", len(list), err)
	}
	other, err := libs.List(ctx, user2.ID)
	if err != nil || len(other) != 0 {
		t.Fatalf("non-member should not see library: n=%d err=%v", len(other), err)
	}

	if err := libs.VerifyOwned(ctx, lib.ID, user1.ID); err != nil {
		t.Fatalf("member should be verified: %v", err)
	}
	if err := libs.VerifyOwned(ctx, lib.ID, user2.ID); err == nil {
		t.Fatal("non-member should not be verified")
	}
	if err := libs.Reorder(ctx, user2.ID, []LibrarySort{{ID: lib.ID, Sort: 5}}); err == nil {
		t.Fatal("non-member reorder should fail")
	}
	if err := libs.Reorder(ctx, user1.ID, []LibrarySort{{ID: lib.ID, Sort: 5}}); err != nil {
		t.Fatalf("member reorder should work: %v", err)
	}
}

func TestSessionCacheDurability(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	users := NewUserService(db)

	user, err := users.Create(ctx, "Dev", "dev2@example.com", "password1")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := users.StartSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	users.mu.RLock()
	_, cached := users.sessions[token]
	users.mu.RUnlock()
	if !cached {
		t.Fatal("session should be held in memory")
	}

	// A fresh service instance models a restart: Load must restore the session.
	restarted := NewUserService(db)
	if err := restarted.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got, err := restarted.UserForSession(ctx, token); err != nil || got.ID != user.ID {
		t.Fatalf("session should survive restart: got=%v err=%v", got, err)
	}

	if err := restarted.EndSession(ctx, token); err != nil {
		t.Fatalf("end session: %v", err)
	}
	if _, err := restarted.UserForSession(ctx, token); err == nil {
		t.Fatal("session should be invalid after logout")
	}
}

func TestSmartCollections(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	users := NewUserService(db)
	libs := NewLibraryService(db, nil)
	smart := NewSmartCollectionService(db)

	user, err := users.Create(ctx, "U", "smart@example.com", "password1")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	lib, err := libs.Create(ctx, user.ID, "L", "", 0)
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	created, err := smart.Create(ctx, lib.ID, "Videos", "3b82f6", domain.SmartCollectionFilter{
		Type: "video",
		Sort: "name-asc",
	})
	if err != nil {
		t.Fatalf("create smart collection: %v", err)
	}

	// Creating a library seeds the built-in Uncategorized/Untagged presets, so
	// the created collection is listed alongside them.
	list, err := smart.List(ctx, lib.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("list: n=%d err=%v", len(list), err)
	}
	var videos *domain.SmartCollection
	for i := range list {
		if list[i].Name == "Videos" {
			videos = &list[i]
		}
	}
	if videos == nil {
		t.Fatalf("created smart collection missing from list: %+v", list)
	}
	if videos.Filter.Type != "video" || videos.Filter.Sort != "name-asc" {
		t.Fatalf("filter not persisted: %+v", videos.Filter)
	}

	if _, err := smart.Create(ctx, lib.ID, "Videos", "3b82f6", domain.SmartCollectionFilter{Type: "video"}); err != nil {
		t.Fatalf("duplicate name should be auto-suffixed: %v", err)
	}

	updated, err := smart.Update(ctx, created.ID, "Clips", "ef4444")
	if err != nil || updated.Name != "Clips" || updated.Color != "ef4444" {
		t.Fatalf("update: %+v err=%v", updated, err)
	}

	if err := smart.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := smart.GetByID(ctx, created.ID); err == nil {
		t.Fatal("expected deleted smart collection to be gone")
	}
}

func TestCollectionHierarchy(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	users := NewUserService(db)
	libs := NewLibraryService(db, nil)
	cols := NewCollectionService(db)

	user, err := users.Create(ctx, "U", "collections@example.com", "password1")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	lib, err := libs.Create(ctx, user.ID, "L", "", 0)
	if err != nil {
		t.Fatalf("create library: %v", err)
	}

	root, err := cols.Create(ctx, lib.ID, nil, "Root", "8b5cf6")
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	child, err := cols.Create(ctx, lib.ID, &root.ID, "Child", "")
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	grandchild, err := cols.Create(ctx, lib.ID, &child.ID, "Grandchild", "")
	if err != nil {
		t.Fatalf("create grandchild: %v", err)
	}

	if _, err := cols.Create(ctx, lib.ID, nil, "Root", ""); err != nil {
		t.Fatalf("duplicate name should be auto-suffixed: %v", err)
	}
	list, err := cols.List(ctx, lib.ID)
	if err != nil || len(list) != 4 {
		t.Fatalf("expected 4 collections, got %d err=%v", len(list), err)
	}

	// Moving a root under its own descendant must be rejected.
	if err := cols.Move(ctx, root.ID, &grandchild.ID, 1); err == nil {
		t.Fatal("cycle should be rejected")
	}

	// Reparent the grandchild to the root and insert it first.
	if err := cols.Move(ctx, grandchild.ID, nil, 0); err != nil {
		t.Fatalf("move to root: %v", err)
	}
	moved, err := cols.GetByID(ctx, grandchild.ID)
	if err != nil || moved.ParentID != nil {
		t.Fatalf("grandchild should be a root now: %+v err=%v", moved, err)
	}

	// Sibling ordering: the moved node should be first among roots.
	var firstRoot struct {
		ID string
	}
	if err := db.Model(&struct{}{}).
		Table("collections").
		Select("id").
		Where("library_id = ? AND parent_id IS NULL", lib.ID).
		Order("sort ASC").
		Limit(1).
		Scan(&firstRoot).Error; err != nil {
		t.Fatalf("query first root: %v", err)
	}
	if firstRoot.ID != grandchild.ID {
		t.Fatalf("expected moved node first, got %s", firstRoot.ID)
	}
}

func TestBulkAssetRelations(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	users := NewUserService(db)
	libs := NewLibraryService(db, nil)
	cols := NewCollectionService(db)
	tags := NewTagService(db)
	assets := &AssetService{db: db}

	user, err := users.Create(ctx, "U", "bulk@example.com", "password1")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	lib, err := libs.Create(ctx, user.ID, "L", "", 0)
	if err != nil {
		t.Fatalf("create library: %v", err)
	}
	col, err := cols.Create(ctx, lib.ID, nil, "C", "")
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	tag1, err := tags.GetOrCreate(ctx, lib.ID, "One")
	if err != nil {
		t.Fatalf("create tag1: %v", err)
	}
	tag2, err := tags.GetOrCreate(ctx, lib.ID, "Two")
	if err != nil {
		t.Fatalf("create tag2: %v", err)
	}
	for _, id := range []string{"a1", "a2"} {
		if err := db.Create(&store.Asset{
			ID: id, LibraryID: lib.ID, Filename: id + ".jpg",
			StorageDriver: "local", StorageKey: id, MimeType: "image/jpeg",
			ByteSize: 1, CreatedAt: time.Now().UTC(),
		}).Error; err != nil {
			t.Fatalf("create asset %s: %v", id, err)
		}
	}

	countRows := func(table, column, value string) int64 {
		t.Helper()
		var n int64
		if err := db.Table(table).Where(column+" = ?", value).Count(&n).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}

	// Bulk add to collection links both assets in one call.
	if err := assets.AddToCollectionBulk(ctx, col.ID, []string{"a1", "a2"}); err != nil {
		t.Fatalf("add to collection bulk: %v", err)
	}
	if n := countRows("asset_collections", "collection_id", col.ID); n != 2 {
		t.Fatalf("expected 2 collection links, got %d", n)
	}

	// Bulk add tags merges the tag into both assets and is idempotent.
	if err := assets.AddTagsBulk(ctx, lib.ID, []string{"a1", "a2"}, []string{tag1.ID}); err != nil {
		t.Fatalf("add tags bulk: %v", err)
	}
	if err := assets.AddTagsBulk(ctx, lib.ID, []string{"a1", "a2"}, []string{tag1.ID}); err != nil {
		t.Fatalf("add tags bulk repeat: %v", err)
	}
	if n := countRows("asset_tags", "tag_id", tag1.ID); n != 2 {
		t.Fatalf("expected 2 tag links, got %d", n)
	}

	// Replace clears the old set then applies the new one.
	if err := assets.ReplaceTagsBulk(ctx, lib.ID, []string{"a1", "a2"}, []string{tag2.ID}); err != nil {
		t.Fatalf("replace tags bulk: %v", err)
	}
	if n := countRows("asset_tags", "tag_id", tag1.ID); n != 0 {
		t.Fatalf("expected old tags cleared, got %d", n)
	}
	if n := countRows("asset_tags", "tag_id", tag2.ID); n != 2 {
		t.Fatalf("expected 2 replacement tag links, got %d", n)
	}

	// An empty replacement clears every tag for the assets.
	if err := assets.ReplaceTagsBulk(ctx, lib.ID, []string{"a1", "a2"}, nil); err != nil {
		t.Fatalf("clear tags bulk: %v", err)
	}
	if n := countRows("asset_tags", "tag_id", tag2.ID); n != 0 {
		t.Fatalf("expected cleared tags, got %d", n)
	}
}
