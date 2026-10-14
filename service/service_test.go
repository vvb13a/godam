package service

import (
	"context"
	"path/filepath"
	"testing"

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
