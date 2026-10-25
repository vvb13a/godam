package store

import (
	"github.com/vvb13a/godam/domain"
	"gorm.io/gorm"
)

// AssetAttribute mirrors asset_attributes. Attributes are dynamic key/value
// pairs supplied by the consuming application; an asset may have multiple
// values for the same key.
type AssetAttribute struct {
	AssetID string `gorm:"column:asset_id;primaryKey;type:text"`
	Key     string `gorm:"column:key;primaryKey;type:text"`
	Value   string `gorm:"column:value;primaryKey;type:text"`
}

func (AssetAttribute) TableName() string { return "asset_attributes" }

func (a *AssetAttribute) ToDomain() domain.AssetAttribute {
	return domain.AssetAttribute{Key: a.Key, Value: a.Value}
}

// ReplaceAssetAttributes stores the exact attribute set for an asset.
func ReplaceAssetAttributes(tx *gorm.DB, assetID string, attrs []domain.AssetAttribute) error {
	if err := tx.Exec(`DELETE FROM asset_attributes WHERE asset_id = ?`, assetID).Error; err != nil {
		return err
	}
	for _, attr := range attrs {
		if attr.Key == "" || attr.Value == "" {
			continue
		}
		if err := tx.Exec(
			`INSERT OR IGNORE INTO asset_attributes (asset_id, key, value) VALUES (?, ?, ?)`,
			assetID, attr.Key, attr.Value,
		).Error; err != nil {
			return err
		}
	}
	return nil
}
