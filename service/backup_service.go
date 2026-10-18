package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/storage"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

const (
	backupDestination = "sftp"
	replicaPending    = "pending"
	replicaSynced     = "synced"
	replicaFailed     = "failed"
)

// SFTPBackupConfig is the secret-bearing payload encrypted in
// backup_settings.config.
type SFTPBackupConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password,omitempty"`
	KeyPath            string `json:"key_path,omitempty"`
	BasePath           string `json:"base_path,omitempty"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`
}

// BackupSettingsRequest is the API update payload. An empty Password keeps the
// stored password unless ClearPassword is set.
type BackupSettingsRequest struct {
	Enabled            bool
	Driver             string
	Host               string
	Port               int
	User               string
	Password           string
	ClearPassword      bool
	KeyPath            string
	BasePath           string
	HostKeyFingerprint string
}

// BackupService mirrors originals to the instance's SFTP destination. Previews
// stay in local storage; only the original blobs are replicated.
type BackupService struct {
	db       *gorm.DB
	registry *storage.Registry
	secret   *SecretBox

	driverMu sync.Mutex
	driver   *storage.SFTP
	wake     chan struct{}
}

func NewBackupService(db *gorm.DB, registry *storage.Registry, secret *SecretBox) *BackupService {
	return &BackupService{
		db:       db,
		registry: registry,
		secret:   secret,
		wake:     make(chan struct{}, 1),
	}
}

func (s *BackupService) Start(ctx context.Context) {
	go func() {
		s.reconcile(ctx)
		jobs := time.NewTicker(10 * time.Second)
		full := time.NewTicker(6 * time.Hour)
		defer jobs.Stop()
		defer full.Stop()
		for {
			select {
			case <-ctx.Done():
				s.closeDriver()
				return
			case <-s.wake:
				s.processJobs(ctx)
			case <-jobs.C:
				s.processJobs(ctx)
			case <-full.C:
				s.reconcile(ctx)
			}
		}
	}()
}

func (s *BackupService) GetSettings(ctx context.Context) (*domain.BackupSettings, error) {
	row, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &domain.BackupSettings{Driver: backupDestination, Port: 22}, nil
	}
	cfg, err := s.configFromRow(row)
	if err != nil {
		return nil, err
	}
	return settingsFromRow(row, cfg), nil
}

func (s *BackupService) SetSettings(ctx context.Context, req BackupSettingsRequest) (*domain.BackupSettings, error) {
	row, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	existing := SFTPBackupConfig{}
	if row != nil {
		existing, _ = s.configFromRow(row)
	}
	cfg := mergeBackupConfig(existing, req)
	if req.Driver != "" && req.Driver != backupDestination {
		return nil, fmt.Errorf("%w: only sftp backups are supported", ErrInvalidInput)
	}
	if req.Enabled {
		if err := validateBackupConfig(cfg); err != nil {
			return nil, err
		}
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	encrypted, err := s.secret.Encrypt(string(encoded))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if row == nil {
		row = &store.BackupSettings{ID: 1, CreatedAt: now}
	}
	row.Enabled = req.Enabled
	row.Driver = backupDestination
	row.Config = encrypted
	row.UpdatedAt = now
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	s.invalidateDriver()
	return s.GetSettings(ctx)
}

func (s *BackupService) TestConnection(ctx context.Context, req BackupSettingsRequest) error {
	row, err := s.settings(ctx)
	if err != nil {
		return err
	}
	existing := SFTPBackupConfig{}
	if row != nil {
		existing, err = s.configFromRow(row)
		if err != nil {
			return err
		}
	}
	if req.Host == "" {
		req.Host = existing.Host
	}
	if req.User == "" {
		req.User = existing.User
	}
	if req.Port == 0 {
		req.Port = existing.Port
	}
	if req.KeyPath == "" {
		req.KeyPath = existing.KeyPath
	}
	if req.BasePath == "" {
		req.BasePath = existing.BasePath
	}
	if req.HostKeyFingerprint == "" {
		req.HostKeyFingerprint = existing.HostKeyFingerprint
	}
	cfg := mergeBackupConfig(existing, req)
	if err := validateBackupConfig(cfg); err != nil {
		return err
	}
	driver, err := storage.NewSFTP(toSFTPConfig(cfg))
	if err != nil {
		return err
	}
	return driver.Close()
}

// BackupNow queues every asset, replacing existing jobs so the run refreshes
// the whole remote copy.
func (s *BackupService) BackupNow(ctx context.Context) (int, error) {
	row, err := s.settings(ctx)
	if err != nil {
		return 0, err
	}
	if row == nil || !row.Enabled {
		return 0, fmt.Errorf("%w: backups are not enabled", ErrInvalidInput)
	}
	type assetRow struct {
		ID            string
		StorageDriver string
		StorageKey    string
		ByteSize      int64
	}
	var assets []assetRow
	if err := s.db.WithContext(ctx).
		Table("assets").
		Select("id, storage_driver, storage_key, byte_size").
		Find(&assets).Error; err != nil {
		return 0, err
	}
	for _, asset := range assets {
		job := store.StorageJob{
			AssetID:       asset.ID,
			StorageDriver: asset.StorageDriver,
			StorageKey:    backupKey(asset.ID, asset.StorageKey),
			Action:        "put",
			Destination:   backupDestination,
		}
		if err := s.enqueue(ctx, job, true, asset.ByteSize); err != nil {
			return 0, err
		}
	}
	return len(assets), nil
}

func (s *BackupService) Status(ctx context.Context) (*domain.BackupStatus, error) {
	row, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	status := &domain.BackupStatus{Enabled: row != nil && row.Enabled}
	if row != nil {
		status.LastRunAt = row.LastRunAt
		status.LastError = row.LastError
	}
	if err := s.db.WithContext(ctx).Model(&store.Asset{}).Count(&status.Total).Error; err != nil {
		return nil, err
	}
	type countRow struct {
		Status string
		Count  int64
	}
	var rows []countRow
	if err := s.db.WithContext(ctx).
		Table("asset_replicas").
		Select("status, COUNT(*) as count").
		Where("destination = ?", backupDestination).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		switch row.Status {
		case replicaSynced:
			status.Synced = row.Count
		case replicaFailed:
			status.Failed = row.Count
		}
	}
	status.Pending = status.Total - status.Synced - status.Failed
	if status.Pending < 0 {
		status.Pending = 0
	}
	return status, nil
}

// EnqueueAsset queues an upsert of one asset's original.
func (s *BackupService) EnqueueAsset(ctx context.Context, assetID, storageDriver, storageKey string, byteSize int64) error {
	row, err := s.settings(ctx)
	if err != nil || row == nil || !row.Enabled {
		return err
	}
	job := store.StorageJob{
		AssetID:       assetID,
		StorageDriver: storageDriver,
		StorageKey:    backupKey(assetID, storageKey),
		Action:        "put",
		Destination:   backupDestination,
	}
	return s.enqueue(ctx, job, true, byteSize)
}

// EnqueueDelete queues removal of an asset's remote copy.
func (s *BackupService) EnqueueDelete(ctx context.Context, assetID, storageKey string) error {
	row, err := s.settings(ctx)
	if err != nil || row == nil || !row.Enabled {
		return err
	}
	job := store.StorageJob{
		AssetID:     assetID,
		StorageKey:  backupKey(assetID, storageKey),
		Action:      "delete",
		Destination: backupDestination,
	}
	return s.enqueue(ctx, job, true, 0)
}

func (s *BackupService) enqueue(ctx context.Context, job store.StorageJob, replace bool, byteSize int64) error {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if replace {
			if err := tx.Where("asset_id = ? AND destination = ?", job.AssetID, job.Destination).
				Delete(&store.StorageJob{}).Error; err != nil {
				return err
			}
		} else {
			var count int64
			if err := tx.Model(&store.StorageJob{}).
				Where("asset_id = ? AND destination = ?", job.AssetID, job.Destination).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return nil
			}
		}
		job.ID = uuid.NewString()
		job.CreatedAt = now
		job.RunAfter = now
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		if job.Action == "put" {
			replica := store.AssetReplica{
				AssetID:     job.AssetID,
				Destination: job.Destination,
				Status:      replicaPending,
				StorageKey:  job.StorageKey,
				ByteSize:    byteSize,
				UpdatedAt:   now,
			}
			return tx.Where("asset_id = ? AND destination = ?", job.AssetID, job.Destination).
				Assign(store.AssetReplica{
					Status:     replicaPending,
					StorageKey: job.StorageKey,
					ByteSize:   byteSize,
					LastError:  "",
					UpdatedAt:  now,
				}).
				FirstOrCreate(&replica).Error
		}
		return nil
	})
	if err == nil {
		s.signal()
	}
	return err
}

func (s *BackupService) processJobs(ctx context.Context) {
	now := time.Now().UTC()
	var jobs []store.StorageJob
	if err := s.db.WithContext(ctx).
		Where("run_after <= ?", now).
		Order("run_after ASC").
		Limit(10).
		Find(&jobs).Error; err != nil {
		log.Printf("backup: list jobs: %v", err)
		return
	}
	for _, job := range jobs {
		jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		s.processJob(jobCtx, job)
		cancel()
	}
}

func (s *BackupService) processJob(ctx context.Context, job store.StorageJob) {
	row, err := s.settings(ctx)
	if err != nil {
		s.failJob(ctx, job, err)
		return
	}
	if row == nil || !row.Enabled {
		_ = s.db.WithContext(ctx).Delete(&store.StorageJob{}, "id = ?", job.ID).Error
		return
	}
	driver, err := s.sftp(ctx)
	if err != nil {
		s.failJob(ctx, job, err)
		return
	}
	now := time.Now().UTC()
	switch job.Action {
	case "delete":
		if err := driver.Delete(ctx, job.StorageKey); err != nil {
			s.failJob(ctx, job, err)
			return
		}
		_ = s.db.WithContext(ctx).
			Where("asset_id = ? AND destination = ?", job.AssetID, job.Destination).
			Delete(&store.AssetReplica{}).Error
	case "put":
		var asset store.Asset
		if err := s.db.WithContext(ctx).First(&asset, "id = ?", job.AssetID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_ = s.db.WithContext(ctx).Delete(&store.StorageJob{}, "id = ?", job.ID).Error
				return
			}
			s.failJob(ctx, job, err)
			return
		}
		primary, err := s.registry.Get(asset.StorageDriver)
		if err != nil {
			s.failJob(ctx, job, err)
			return
		}
		rc, err := primary.Open(ctx, asset.StorageKey)
		if err != nil {
			s.failJob(ctx, job, err)
			return
		}
		err = driver.Save(ctx, job.StorageKey, rc)
		_ = rc.Close()
		if err != nil {
			s.failJob(ctx, job, err)
			return
		}
		_ = s.db.WithContext(ctx).
			Model(&store.AssetReplica{}).
			Where("asset_id = ? AND destination = ?", job.AssetID, job.Destination).
			Updates(map[string]any{
				"status":      replicaSynced,
				"storage_key": job.StorageKey,
				"byte_size":   asset.ByteSize,
				"last_error":  "",
				"updated_at":  now,
			}).Error
	default:
		_ = s.db.WithContext(ctx).Delete(&store.StorageJob{}, "id = ?", job.ID).Error
		return
	}
	_ = s.db.WithContext(ctx).Delete(&store.StorageJob{}, "id = ?", job.ID).Error
	_ = s.db.WithContext(ctx).
		Model(&store.BackupSettings{}).
		Where("id = 1").
		Updates(map[string]any{"last_run_at": now, "last_error": "", "updated_at": now}).Error
}

func (s *BackupService) failJob(ctx context.Context, job store.StorageJob, cause error) {
	attempts := job.Attempts + 1
	backoff := time.Duration(1<<min(attempts, 10)) * time.Minute
	if backoff > 6*time.Hour {
		backoff = 6 * time.Hour
	}
	now := time.Now().UTC()
	message := cause.Error()
	_ = s.db.WithContext(ctx).
		Model(&store.StorageJob{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"attempts":   attempts,
			"run_after":  now.Add(backoff),
			"last_error": message,
		}).Error
	if job.Action == "put" {
		_ = s.db.WithContext(ctx).
			Model(&store.AssetReplica{}).
			Where("asset_id = ? AND destination = ?", job.AssetID, job.Destination).
			Updates(map[string]any{
				"status":     replicaFailed,
				"last_error": message,
				"updated_at": now,
			}).Error
	}
	_ = s.db.WithContext(ctx).
		Model(&store.BackupSettings{}).
		Where("id = 1").
		Updates(map[string]any{"last_error": message, "updated_at": now}).Error
}

func (s *BackupService) reconcile(ctx context.Context) {
	row, err := s.settings(ctx)
	if err != nil || row == nil || !row.Enabled {
		return
	}
	type assetRow struct {
		ID            string
		StorageDriver string
		StorageKey    string
		ByteSize      int64
	}
	var rows []assetRow
	if err := s.db.WithContext(ctx).
		Table("assets").
		Select("assets.id, assets.storage_driver, assets.storage_key, assets.byte_size").
		Joins("LEFT JOIN asset_replicas ON asset_replicas.asset_id = assets.id AND asset_replicas.destination = ?", backupDestination).
		Where("asset_replicas.asset_id IS NULL OR asset_replicas.status <> ?", replicaSynced).
		Scan(&rows).Error; err != nil {
		log.Printf("backup: reconcile: %v", err)
		return
	}
	for _, asset := range rows {
		job := store.StorageJob{
			AssetID:       asset.ID,
			StorageDriver: asset.StorageDriver,
			StorageKey:    backupKey(asset.ID, asset.StorageKey),
			Action:        "put",
			Destination:   backupDestination,
		}
		if err := s.enqueue(ctx, job, false, asset.ByteSize); err != nil {
			log.Printf("backup: queue %s: %v", asset.ID, err)
		}
	}
}

func (s *BackupService) settings(ctx context.Context) (*store.BackupSettings, error) {
	var row store.BackupSettings
	err := s.db.WithContext(ctx).First(&row, "id = 1").Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *BackupService) configFromRow(row *store.BackupSettings) (SFTPBackupConfig, error) {
	if row.Config == "" {
		return SFTPBackupConfig{}, nil
	}
	plain, err := s.secret.Decrypt(row.Config)
	if err != nil {
		return SFTPBackupConfig{}, err
	}
	var cfg SFTPBackupConfig
	if err := json.Unmarshal([]byte(plain), &cfg); err != nil {
		return SFTPBackupConfig{}, err
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	return cfg, nil
}

func (s *BackupService) sftp(ctx context.Context) (*storage.SFTP, error) {
	s.driverMu.Lock()
	if s.driver != nil {
		driver := s.driver
		s.driverMu.Unlock()
		return driver, nil
	}
	s.driverMu.Unlock()

	row, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.New("backup destination is not configured")
	}
	cfg, err := s.configFromRow(row)
	if err != nil {
		return nil, err
	}
	driver, err := storage.NewSFTP(toSFTPConfig(cfg))
	if err != nil {
		return nil, err
	}
	s.driverMu.Lock()
	defer s.driverMu.Unlock()
	if s.driver != nil {
		_ = driver.Close()
		return s.driver, nil
	}
	s.driver = driver
	return driver, nil
}

func (s *BackupService) invalidateDriver() {
	s.driverMu.Lock()
	defer s.driverMu.Unlock()
	if s.driver != nil {
		_ = s.driver.Close()
		s.driver = nil
	}
}

func (s *BackupService) closeDriver() {
	s.driverMu.Lock()
	defer s.driverMu.Unlock()
	if s.driver != nil {
		_ = s.driver.Close()
		s.driver = nil
	}
}

func (s *BackupService) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func settingsFromRow(row *store.BackupSettings, cfg SFTPBackupConfig) *domain.BackupSettings {
	return &domain.BackupSettings{
		Enabled:            row.Enabled,
		Driver:             row.Driver,
		Host:               cfg.Host,
		Port:               cfg.Port,
		User:               cfg.User,
		BasePath:           cfg.BasePath,
		KeyPath:            cfg.KeyPath,
		HostKeyFingerprint: cfg.HostKeyFingerprint,
		HasPassword:        cfg.Password != "",
		LastRunAt:          row.LastRunAt,
		LastError:          row.LastError,
	}
}

func mergeBackupConfig(existing SFTPBackupConfig, req BackupSettingsRequest) SFTPBackupConfig {
	cfg := SFTPBackupConfig{
		Host:               req.Host,
		Port:               req.Port,
		User:               req.User,
		KeyPath:            req.KeyPath,
		BasePath:           req.BasePath,
		HostKeyFingerprint: req.HostKeyFingerprint,
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if req.ClearPassword {
		cfg.Password = ""
	} else if req.Password != "" {
		cfg.Password = req.Password
	} else {
		cfg.Password = existing.Password
	}
	return cfg
}

func validateBackupConfig(cfg SFTPBackupConfig) error {
	if cfg.Host == "" || cfg.User == "" {
		return fmt.Errorf("%w: host and user are required", ErrInvalidInput)
	}
	if cfg.Password == "" && cfg.KeyPath == "" {
		return fmt.Errorf("%w: a password or SSH key path is required", ErrInvalidInput)
	}
	return nil
}

func toSFTPConfig(cfg SFTPBackupConfig) storage.SFTPConfig {
	return storage.SFTPConfig{
		Host:               cfg.Host,
		Port:               cfg.Port,
		User:               cfg.User,
		Password:           cfg.Password,
		KeyPath:            cfg.KeyPath,
		BasePath:           cfg.BasePath,
		HostKeyFingerprint: cfg.HostKeyFingerprint,
	}
}

func backupKey(assetID, storageKey string) string {
	ext := filepath.Ext(storageKey)
	return path.Join("assets", assetID+ext)
}
