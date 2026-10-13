package main

import (
	"log"
	"net/http"

	"github.com/vvb13a/godam/api"
	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
)

// addr is the HTTP listen address for the API and embedded frontend.
const addr = ":8080"

var (
	appConfig         *service.Config
	assetService      *service.AssetService
	collectionService *service.CollectionService
	tagService        *service.TagService
	tagGroupService   *service.TagGroupService
	statsService      *service.StatsService
)

func main() {
	initDependencies()
	runServer(api.Deps{
		Assets:         assetService,
		Collections:    collectionService,
		Tags:           tagService,
		TagGroups:      tagGroupService,
		Stats:          statsService,
		MaxUploadBytes: appConfig.MaxUploadSizeBytes,
	})
}

// runServer starts the HTTP server that serves both the JSON API and the
// embedded Vue frontend.
func runServer(deps api.Deps) {
	frontend, err := newFrontendHandler()
	if err != nil {
		log.Fatalf("frontend: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewServer(deps))
	mux.Handle("/", frontend)

	log.Printf("godam listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func initDependencies() {
	cfg := service.DefaultConfig()
	appConfig = cfg

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
	tagGroupService = service.NewTagGroupService(db)
	collectionService = service.NewCollectionService(db)
	statsService = service.NewStatsService(db)
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
