package store

import (
	"encoding/json"
	"time"

	"github.com/vvb13a/godam/domain"
)

// Instance mirrors the singleton "instance" table.
type Instance struct {
	ID               string    `gorm:"column:id;primaryKey;type:text"`
	Name             string    `gorm:"column:name;type:text;not null"`
	Color            string    `gorm:"column:color;type:text;not null;default:'8b5cf6'"`
	CoverAssetID     *string   `gorm:"column:cover_asset_id;type:text"`
	WatermarkAssetID *string   `gorm:"column:watermark_asset_id;type:text"`
	AllowedMimeTypes string    `gorm:"column:allowed_mime_types;type:text;not null;default:''"`
	MaxUploadBytes   int64     `gorm:"column:max_upload_bytes;not null;default:0"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (Instance) TableName() string { return "instance" }

func InstanceModel(i *domain.Instance) *Instance {
	var cover *string
	if i.CoverAssetID != "" {
		cover = &i.CoverAssetID
	}
	var watermark *string
	if i.WatermarkAssetID != "" {
		watermark = &i.WatermarkAssetID
	}
	allowed := ""
	if len(i.AllowedMimeTypes) > 0 {
		if encoded, err := json.Marshal(i.AllowedMimeTypes); err == nil {
			allowed = string(encoded)
		}
	}
	return &Instance{
		ID:               i.ID,
		Name:             i.Name,
		Color:            i.Color,
		CoverAssetID:     cover,
		WatermarkAssetID: watermark,
		AllowedMimeTypes: allowed,
		MaxUploadBytes:   i.MaxUploadBytes,
		CreatedAt:        i.CreatedAt,
		UpdatedAt:        i.UpdatedAt,
	}
}

func (m *Instance) ToDomain() *domain.Instance {
	var cover string
	if m.CoverAssetID != nil {
		cover = *m.CoverAssetID
	}
	var watermark string
	if m.WatermarkAssetID != nil {
		watermark = *m.WatermarkAssetID
	}
	var allowed []string
	if m.AllowedMimeTypes != "" {
		_ = json.Unmarshal([]byte(m.AllowedMimeTypes), &allowed)
	}
	return &domain.Instance{
		ID:               m.ID,
		Name:             m.Name,
		Color:            m.Color,
		CoverAssetID:     cover,
		WatermarkAssetID: watermark,
		AllowedMimeTypes: allowed,
		MaxUploadBytes:   m.MaxUploadBytes,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

// BackupSettings mirrors the singleton "backup_settings" table.
type BackupSettings struct {
	ID        int        `gorm:"column:id;primaryKey;not null"`
	Enabled   bool       `gorm:"column:enabled;not null;default:false"`
	Driver    string     `gorm:"column:driver;type:text;not null;default:'sftp'"`
	Config    string     `gorm:"column:config;type:text;not null;default:''"`
	LastRunAt *time.Time `gorm:"column:last_run_at"`
	LastError string     `gorm:"column:last_error;type:text;not null;default:''"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (BackupSettings) TableName() string { return "backup_settings" }
