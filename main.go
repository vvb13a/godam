package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/vvb13a/godam/api"
	"github.com/vvb13a/godam/service"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
)

// defaultAddr is the HTTP listen address for the API and embedded frontend.
const defaultAddr = ":8080"

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

var (
	appConfig              *service.Config
	assetService           *service.AssetService
	collectionService      *service.CollectionService
	smartCollectionService *service.SmartCollectionService
	tagService             *service.TagService
	tagGroupService        *service.TagGroupService
	instanceService        *service.InstanceService
	userService            *service.UserService
	statsService           *service.StatsService
	backupService          *service.BackupService
	systemService          *service.SystemService
)

func main() {
	addr := os.Getenv("GODAM_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	initDependencies()
	go assetService.BackfillColors(context.Background())
	runServer(addr, api.Deps{
		Assets:           assetService,
		Collections:      collectionService,
		SmartCollections: smartCollectionService,
		Tags:             tagService,
		TagGroups:        tagGroupService,
		Instance:         instanceService,
		Users:            userService,
		Stats:            statsService,
		Backups:          backupService,
		System:           systemService,
		ManagementToken:  os.Getenv("GODAM_MANAGEMENT_TOKEN"),
		MaxUploadBytes:   appConfig.MaxUploadSizeBytes,
	})
}

// runServer starts the HTTP server that serves both the JSON API and the
// embedded Vue frontend.
func runServer(addr string, deps api.Deps) {
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
	smartCollectionService = service.NewSmartCollectionService(db)
	statsService = service.NewStatsService(db)
	userService = service.NewUserService(db)
	if err := userService.Load(context.Background()); err != nil {
		log.Printf("warning: could not prime user session cache: %v", err)
	}
	secretBox, err := service.NewSecretBox(filepath.Join(filepath.Dir(dbPath), "backup.key"))
	if err != nil {
		log.Fatalf("backup secret init failed: %v", err)
	}
	backupService = service.NewBackupService(db, registry, secretBox)

	assetService = service.NewAssetService(
		cfg,
		db,
		registry,
		localStore,
		service.NewCompositeExtractor(),
		service.NewPreviewGenerator(600, 85),
		tagService,
		backupService,
	)
	instanceService = service.NewInstanceService(db, assetService)
	instanceName := os.Getenv("GODAM_LIBRARY_NAME")
	if instanceName == "" {
		instanceName = "Library"
	}
	instance, err := instanceService.Ensure(context.Background(), instanceName)
	if err != nil {
		log.Fatalf("instance init failed: %v", err)
	}
	systemService = service.NewSystemService(db, dbPath, storagePath, version, instanceService, backupService)
	backupService.Start(context.Background())

	log.Printf("godam instance: %s (%s)", instance.Name, instance.ID)
	log.Printf("godam database: %s", dbPath)
	log.Printf("godam storage:  %s", storagePath)
}
