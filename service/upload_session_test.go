package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
)

func newUploadTestService(t *testing.T) *AssetService {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "uploads.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	local, err := storage.NewLocal(filepath.Join(dir, "assets"))
	if err != nil {
		t.Fatalf("local storage: %v", err)
	}
	registry := storage.NewRegistry("local")
	registry.Register("local", local)
	cfg := DefaultConfig()
	cfg.StoragePath = filepath.Join(dir, "assets")
	return NewAssetService(
		cfg, db, registry, local,
		NewCompositeExtractor(), NewPreviewGenerator(600, 85),
		NewTagService(db), nil,
	)
}

func TestChunkedUploadAssemblesAsset(t *testing.T) {
	ctx := context.Background()
	assets := newUploadTestService(t)
	content := bytes.Repeat(testPNG(t), 100)

	session, err := assets.StartUpload(ctx, "user-1", "", "notes.png", "image/png", int64(len(content)))
	if err != nil {
		t.Fatalf("start upload: %v", err)
	}
	if session.ChunkSize != UploadChunkSize || session.Size != int64(len(content)) {
		t.Fatalf("unexpected session: %+v", session)
	}

	offset := int64(0)
	for _, chunk := range [][]byte{content[:700], content[700:1600], content[1600:]} {
		written, err := assets.WriteUploadChunk(ctx, session.ID, "user-1", offset, bytes.NewReader(chunk))
		if err != nil {
			t.Fatalf("write chunk at %d: %v", offset, err)
		}
		if written != int64(len(chunk)) {
			t.Fatalf("wrote %d bytes, want %d", written, len(chunk))
		}
		offset += written
	}

	asset, err := assets.CompleteUpload(ctx, session.ID, "user-1")
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if asset.ByteSize != int64(len(content)) || asset.Filename != "notes.png" {
		t.Fatalf("unexpected asset: %+v", asset)
	}

	reader, err := assets.Open(ctx, asset.ID)
	if err != nil {
		t.Fatalf("open asset: %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatal("stored content does not match uploaded chunks")
	}

	if _, err := assets.CompleteUpload(ctx, session.ID, "user-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second complete should be not found, got %v", err)
	}
}

func TestChunkedUploadPersistsTokenUploaderName(t *testing.T) {
	assets := newUploadTestService(t)
	ctx := domain.WithActor(context.Background(), &domain.Actor{
		ID:    "cms-user-42",
		Name:  "Ada Lovelace",
		Token: true,
	})
	content := testPNG(t)

	session, err := assets.StartUpload(ctx, "cms-user-42", "", "portrait.png", "image/png", int64(len(content)))
	if err != nil {
		t.Fatalf("start upload: %v", err)
	}
	if _, err := assets.WriteUploadChunk(ctx, session.ID, "cms-user-42", 0, bytes.NewReader(content)); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	asset, err := assets.CompleteUpload(ctx, session.ID, "cms-user-42")
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if asset.UploadedBy != "cms-user-42" || asset.UploadedByName != "Ada Lovelace" {
		t.Fatalf("uploader not persisted: %+v", asset)
	}
}

func TestChunkedUploadRejectsWrongOwnerAndIncomplete(t *testing.T) {
	ctx := context.Background()
	assets := newUploadTestService(t)
	content := testPNG(t)

	session, err := assets.StartUpload(ctx, "user-1", "", "hello.png", "image/png", int64(len(content)))
	if err != nil {
		t.Fatalf("start upload: %v", err)
	}

	if _, err := assets.WriteUploadChunk(ctx, session.ID, "user-2", 0, bytes.NewReader(content)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong owner should be not found, got %v", err)
	}

	if _, err := assets.WriteUploadChunk(ctx, session.ID, "user-1", 0, bytes.NewReader(content[:4])); err != nil {
		t.Fatalf("write partial chunk: %v", err)
	}
	if _, err := assets.CompleteUpload(ctx, session.ID, "user-1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("incomplete upload should be invalid, got %v", err)
	}
}
