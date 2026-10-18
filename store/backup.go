package store

import "time"

// LibraryBackup is the per-library backup destination configuration. Config is
// an encrypted JSON blob; the service layer is the only place that decrypts it.
type LibraryBackup struct {
	LibraryID string     `gorm:"column:library_id;primaryKey;type:text"`
	Enabled   bool       `gorm:"column:enabled;not null;default:false"`
	Driver    string     `gorm:"column:driver;type:text;not null;default:'sftp'"`
	Config    string     `gorm:"column:config;type:text;not null;default:''"`
	LastRunAt *time.Time `gorm:"column:last_run_at"`
	LastError string     `gorm:"column:last_error;type:text;not null;default:''"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (LibraryBackup) TableName() string { return "library_backups" }

// AssetReplica records the state of one asset's copy at a backup destination.
type AssetReplica struct {
	AssetID     string    `gorm:"column:asset_id;primaryKey;type:text"`
	Destination string    `gorm:"column:destination;primaryKey;type:text;not null;default:'sftp'"`
	Status      string    `gorm:"column:status;type:text;not null;default:'pending'"`
	StorageKey  string    `gorm:"column:storage_key;type:text;not null;default:''"`
	ByteSize    int64     `gorm:"column:byte_size;not null;default:0"`
	LastError   string    `gorm:"column:last_error;type:text;not null;default:''"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (AssetReplica) TableName() string { return "asset_replicas" }

// StorageJob is a queued put/delete against a backup destination. The worker
// processes jobs in run-after order and retries failures with backoff.
type StorageJob struct {
	ID            string    `gorm:"column:id;primaryKey;type:text"`
	LibraryID     string    `gorm:"column:library_id;type:text;not null"`
	AssetID       string    `gorm:"column:asset_id;type:text;not null;default:''"`
	StorageDriver string    `gorm:"column:storage_driver;type:text;not null;default:''"`
	StorageKey    string    `gorm:"column:storage_key;type:text;not null"`
	Action        string    `gorm:"column:action;type:text;not null"`
	Destination   string    `gorm:"column:destination;type:text;not null;default:'sftp'"`
	Attempts      int       `gorm:"column:attempts;not null;default:0"`
	RunAfter      time.Time `gorm:"column:run_after"`
	LastError     string    `gorm:"column:last_error;type:text;not null;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (StorageJob) TableName() string { return "storage_jobs" }
