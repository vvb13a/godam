package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	appConfig         *service.Config
	assetService      *service.AssetService
	conversionService *service.ConversionService
	presetService     *service.PresetService
	instanceService   *service.InstanceService
	statsService      *service.StatsService
	backupService     *service.BackupService
	systemService     *service.SystemService
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
		Conversions:      conversionService,
		Presets:          presetService,
		DeliveryBaseURL:  appConfig.DeliveryBaseURL(),
		DeliveryCacheTTL: appConfig.DeliveryCacheTTL,
		Instance:         instanceService,
		Stats:            statsService,
		Backups:          backupService,
		System:           systemService,
		ManagementToken:  os.Getenv("GODAM_MANAGEMENT_TOKEN"),
		JWTSecret:        os.Getenv("GODAM_JWT_SECRET"),
		JWTIssuer:        os.Getenv("GODAM_JWT_ISSUER"),
		JWTAudience:      os.Getenv("GODAM_JWT_AUDIENCE"),
	})
}

// runServer starts the HTTP server that serves both the JSON API and the
// embedded Vue frontend.
func runServer(addr string, deps api.Deps) {
	frontend, err := newFrontendHandler()
	if err != nil {
		log.Fatalf("frontend: %v", err)
	}

	apiServer := api.NewServer(deps)
	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer)
	// Public delivery routes. /img and /src must be mounted explicitly;
	// otherwise the SPA fallback would swallow them.
	mux.Handle("/img/", apiServer)
	mux.Handle("/src/", apiServer)
	// Internal delivery-cache misses and imgproxy source fetches. Caddy blocks
	// /internal/* from public requests.
	mux.Handle("/internal/", apiServer)
	mux.Handle("/", frontend)

	log.Printf("godam listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func initDependencies() {
	cfg := service.DefaultConfig()
	applyEnvConfig(cfg)
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
	if cfg.ConversionDir == "" {
		cfg.ConversionDir = filepath.Join(storagePath, "conversions")
	}

	localStore, err := storage.NewLocal(storagePath)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}

	registry := storage.NewRegistry("local")
	registry.Register("local", localStore)

	conversionStore, err := newConversionStore(cfg)
	if err != nil {
		log.Fatalf("conversion store init failed: %v", err)
	}

	proxy, err := service.NewImgProxy(cfg.ImgProxyURL, cfg.InternalURL, cfg.ImgProxyKey, cfg.ImgProxySalt)
	if err != nil {
		log.Fatalf("imgproxy init failed: %v", err)
	}
	if !proxy.Signed() {
		log.Printf("warning: imgproxy URL signing is disabled; set GODAM_IMGPROXY_KEY and GODAM_IMGPROXY_SALT")
	}

	// 3. Application services (gorm persistence, no repository adapters).
	presetService = service.NewPresetService(db)
	statsService = service.NewStatsService(db, conversionStore)
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
		service.NewPreviewGenerator(1920, 85),
		backupService,
	)
	conversionService = service.NewConversionService(assetService, presetService, proxy, conversionStore)
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
	log.Printf("godam convert:  %s (driver=%s)", cfg.ConversionDir, cfg.ConversionDriver)
	if base := cfg.DeliveryBaseURL(); base != "" {
		log.Printf("godam delivery: %s", base)
	}
}

// applyEnvConfig reads deployment settings. Only GODAM_DOMAIN is required; the
// delivery domain and the rest have workable defaults.
func applyEnvConfig(cfg *service.Config) {
	cfg.Domain = normalizeDomain(envOrDefault("GODAM_DOMAIN", cfg.Domain))
	cfg.DeliveryDomain = normalizeDomain(envOrDefault("GODAM_DELIVERY_DOMAIN", cfg.Domain))
	cfg.ConversionDir = envOrDefault("GODAM_CONVERSION_DIR", cfg.ConversionDir)
	cfg.ConversionDriver = envOrDefault("GODAM_CONVERSIONS_DRIVER", "local")
	cfg.ImgProxyURL = envOrDefault("GODAM_IMGPROXY_URL", "http://127.0.0.1:8080")
	cfg.ImgProxyKey = strings.TrimSpace(os.Getenv("GODAM_IMGPROXY_KEY"))
	cfg.ImgProxySalt = strings.TrimSpace(os.Getenv("GODAM_IMGPROXY_SALT"))
	cfg.InternalURL = envOrDefault("GODAM_INTERNAL_URL", internalURL(os.Getenv("GODAM_ADDR")))
	cfg.DeliveryCacheTTL = deliveryCacheTTL()
}

const (
	defaultDeliveryCacheTTL = 24 * time.Hour
	minDeliveryCacheTTL     = time.Minute
	maxDeliveryCacheTTL     = 7 * 24 * time.Hour
)

// deliveryCacheTTL reads the delivery cache lifetime in seconds. Invalid
// values fall back to the default; valid values are clamped to 60s–7d.
func deliveryCacheTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GODAM_DELIVERY_CACHE_TTL"))
	if raw == "" {
		return defaultDeliveryCacheTTL
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("invalid GODAM_DELIVERY_CACHE_TTL %q; using %s", raw, defaultDeliveryCacheTTL)
		return defaultDeliveryCacheTTL
	}
	ttl := time.Duration(seconds) * time.Second
	if ttl < minDeliveryCacheTTL {
		return minDeliveryCacheTTL
	}
	if ttl > maxDeliveryCacheTTL {
		return maxDeliveryCacheTTL
	}
	return ttl
}

func newConversionStore(cfg *service.Config) (storage.ConversionStore, error) {
	switch cfg.ConversionDriver {
	case "", "local":
		return storage.NewLocalConversions(cfg.ConversionDir, cfg.DeliveryURL("/img"))
	default:
		return nil, fmt.Errorf("unsupported conversion driver %q", cfg.ConversionDriver)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func normalizeDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	return strings.TrimRight(raw, "/")
}

// internalURL is the origin imgproxy uses to fetch source assets from godam.
func internalURL(addr string) string {
	if addr == "" {
		addr = defaultAddr
	}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		return "http://127.0.0.1:" + port
	}
	return "http://127.0.0.1" + addr
}
