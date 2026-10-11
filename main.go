package main

import (
	"log"

	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
)

var (
	assetService      *service.AssetService
	collectionService *service.CollectionService
	tagService        *service.TagService
)

func main() {
	initDependencies()
	// The JSON API (api/) and the embedded Vue frontend (frontend/) will be
	// wired in here, replacing the former TUI delivery.
	log.Println("godam dependencies initialized")
}

func initDependencies() {
	cfg := service.DefaultConfig()

	// 1. Database lives in the OS-specific per-user location, never inside the
	// app folder.
	dbPath, err := store.DatabasePath()
	if err != nil {
		log.Fatalf("failed to resolve database path: %v", err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// 2. Local asset storage alongside the database.
	storagePath, err := store.StorageDir()
	if err != nil {
		log.Fatalf("failed to resolve storage path: %v", err)
	}
	cfg.StoragePath = storagePath

	localStore, err := storage.NewLocal(storagePath)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}

	registry := storage.NewRegistry("local")
	registry.Register("local", localStore)

	// 3. Application services (gorm persistence, no repository adapters).
	tagService = service.NewTagService(db)
	collectionService = service.NewCollectionService(db)
	assetService = service.NewAssetService(
		cfg,
		db,
		registry,
		localStore,
		service.NewCompositeExtractor(),
		service.NewPreviewGenerator(600, 85),
		tagService,
	)

	log.Printf("godam database: %s", dbPath)
	log.Printf("godam storage:  %s", storagePath)
}
