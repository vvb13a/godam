package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/vvb13a/godam/internal/adapters/extractor"
	sqlrepo "github.com/vvb13a/godam/internal/adapters/repository/sql"
	"github.com/vvb13a/godam/internal/adapters/repository/sqlite"
	"github.com/vvb13a/godam/internal/adapters/storage"
	"github.com/vvb13a/godam/internal/config"
	"github.com/vvb13a/godam/internal/ports"
	"github.com/vvb13a/godam/internal/service"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var (
	assetService      *service.AssetService
	collectionService *service.CollectionService
	tagService        *service.TagService

	rootCmd = &cobra.Command{
		Use:   "godam",
		Short: "godam - Lightweight Digital Asset Manager",
	}
)

func initDependencies() {
	_ = godotenv.Load()
	cfg := config.DefaultConfig()

	db, err := sql.Open("sqlite", cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatalf("foreign keys failed: %v", err)
	}
	if err := sqlrepo.RunMigrations(db, "sqlite3"); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	localStore, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}

	storageRegistry := ports.NewStorageRegistry("local")
	storageRegistry.Register("local", localStore)

	if sftpHost := os.Getenv("DAM_SFTP_HOST"); sftpHost != "" {
		port := 22
		if p := os.Getenv("DAM_SFTP_PORT"); p != "" {
			if parsedPort, err := strconv.Atoi(p); err == nil && parsedPort > 0 {
				port = parsedPort
			}
		}

		basePath := os.Getenv("DAM_SFTP_BASE_PATH")
		if basePath == "" {
			basePath = "upload"
		}

		sftpStore, err := storage.NewSFTPStorage(storage.SFTPConfig{
			Host:     sftpHost,
			Port:     port,
			User:     os.Getenv("DAM_SFTP_USER"),
			Password: os.Getenv("DAM_SFTP_PASS"),
			BasePath: basePath,
		})
		if err != nil {
			log.Printf("⚠️ SFTP disabled (failed to connect: %v)", err)
		} else {
			storageRegistry.Register("sftp", sftpStore)
		}
	}

	compositeExtractor := extractor.NewCompositeExtractor()
	previewGen := service.NewPreviewGenerator(600, 85)

	tagRepo := sqlite.NewTagRepo(db)
	assetRepo := sqlite.NewAssetRepo(db)
	collectionRepo := sqlite.NewCollectionRepo(db)

	tagService = service.NewTagService(tagRepo)
	assetService = service.NewAssetService(cfg, assetRepo, storageRegistry, localStore, compositeExtractor, tagRepo, previewGen)
	collectionService = service.NewCollectionService(collectionRepo)
}
