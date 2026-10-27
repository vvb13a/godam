package store

import (
	"time"

	"gorm.io/gorm"
)

// AttributeMetadata mirrors attributes_metadata. Rows with an empty Value are
// key-level metadata; rows with a Value override that value's label.
type AttributeMetadata struct {
	Key               string `gorm:"column:key;primaryKey;type:text"`
	Value             string `gorm:"column:value;primaryKey;type:text"`
	Label             string `gorm:"column:label;type:text"`
	Sort              string `gorm:"column:sort;type:text"`
	Render            string `gorm:"column:render;type:text"`
	Searchable        bool   `gorm:"column:searchable"`
	Multi             bool   `gorm:"column:multi"`
	IncludeUnassigned bool   `gorm:"column:include_unassigned"`
	UnassignedLabel   string `gorm:"column:unassigned_label;type:text"`
	ParentKey         string `gorm:"column:parent_key;type:text"`
	ValuesLocked      bool   `gorm:"column:values_locked"`
	ValueSearch       bool   `gorm:"column:value_search"`
	Description       string `gorm:"column:description;type:text"`
	Color             string `gorm:"column:color;type:text"`
	Position          int    `gorm:"column:position"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (AttributeMetadata) TableName() string { return "attributes_metadata" }

// ListAttributeMetadata returns every metadata row. Callers apply only the rows
// for facets the current actor may see.
func ListAttributeMetadata(db *gorm.DB) ([]AttributeMetadata, error) {
	var rows []AttributeMetadata
	err := db.Order("key ASC, value ASC").Find(&rows).Error
	return rows, err
}

// UpsertAttributeLabel stores a label override for a key (value == "") or one
// of its values without touching the row's sort setting.
func UpsertAttributeLabel(
	db *gorm.DB,
	key, value, label string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, created_at, updated_at)
		 VALUES (?, ?, ?, '', '', ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   label = excluded.label,
		   updated_at = excluded.updated_at`,
		key, value, label, now, now,
	).Error
}

// UpsertAttributeSort stores the key-level sort setting without touching the
// key's label override.
func UpsertAttributeSort(
	db *gorm.DB,
	key, sort string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, created_at, updated_at)
		 VALUES (?, '', '', ?, '', ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   sort = excluded.sort,
		   updated_at = excluded.updated_at`,
		key, sort, now, now,
	).Error
}

// InsertAttributeValues adds metadata-only values for a key. Existing rows are
// left untouched, so labels and other metadata are never overwritten.
func InsertAttributeValues(
	db *gorm.DB,
	key string,
	values []string,
	now time.Time,
) error {
	for _, value := range values {
		if err := db.Exec(
			`INSERT INTO attributes_metadata (key, value, label, sort, render, position, created_at, updated_at)
			 VALUES (?, ?, '', '', '',
			   (SELECT COALESCE(MAX(position), -1) + 1 FROM attributes_metadata WHERE key = ? AND value <> ''),
			   ?, ?)
			 ON CONFLICT(key, value) DO NOTHING`,
			key, value, key, now, now,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpsertAttributeRender stores the key-level render setting without touching
// the key's label or sort metadata.
func UpsertAttributeRender(
	db *gorm.DB,
	key, render string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, created_at, updated_at)
		 VALUES (?, '', '', '', ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   render = excluded.render,
		   updated_at = excluded.updated_at`,
		key, render, now, now,
	).Error
}

// UpsertAttributeSearchable stores the key-level search indexing flag.
func UpsertAttributeSearchable(
	db *gorm.DB,
	key string,
	searchable bool,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, created_at, updated_at)
		 VALUES (?, '', '', '', '', ?, 1, 0, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   searchable = excluded.searchable,
		   updated_at = excluded.updated_at`,
		key, searchable, now, now,
	).Error
}

// UpsertAttributeMulti stores the key-level single/multi selection flag.
func UpsertAttributeMulti(
	db *gorm.DB,
	key string,
	multi bool,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, created_at, updated_at)
		 VALUES (?, '', '', '', '', 0, ?, 0, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   multi = excluded.multi,
		   updated_at = excluded.updated_at`,
		key, multi, now, now,
	).Error
}

// UpsertAttributeUnassigned stores whether the key exposes an Unassigned
// option in renderers.
func UpsertAttributeUnassigned(
	db *gorm.DB,
	key string,
	include bool,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, created_at, updated_at)
		 VALUES (?, '', '', '', '', 0, 1, ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   include_unassigned = excluded.include_unassigned,
		   updated_at = excluded.updated_at`,
		key, include, now, now,
	).Error
}

// DeleteAttributeValue removes a value's metadata and every asset assignment.
func DeleteAttributeValue(tx *gorm.DB, key, value string) error {
	if err := tx.Exec(
		`DELETE FROM asset_attributes WHERE key = ? AND value = ?`,
		key, value,
	).Error; err != nil {
		return err
	}
	return tx.Exec(
		`DELETE FROM attributes_metadata WHERE key = ? AND value = ?`,
		key, value,
	).Error
}

// DeleteAttributeKey removes all metadata and every asset assignment for a key.
func DeleteAttributeKey(tx *gorm.DB, key string) error {
	if err := tx.Exec(
		`DELETE FROM asset_attributes WHERE key = ?`,
		key,
	).Error; err != nil {
		return err
	}
	return tx.Exec(
		`DELETE FROM attributes_metadata WHERE key = ?`,
		key,
	).Error
}

// UpsertAttributeUnassignedLabel overrides the display label of the synthetic
// Unassigned option.
func UpsertAttributeUnassignedLabel(
	db *gorm.DB,
	key, label string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, unassigned_label, created_at, updated_at)
		 VALUES (?, '', '', '', '', 0, 1, 0, ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   unassigned_label = excluded.unassigned_label,
		   updated_at = excluded.updated_at`,
		key, label, now, now,
	).Error
}

// UpsertAttributeParent stores the key-level parent used by the sidebar tree.
func UpsertAttributeParent(
	db *gorm.DB,
	key, parent string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, unassigned_label, parent_key, created_at, updated_at)
		 VALUES (?, '', '', '', '', 0, 1, 0, '', ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   parent_key = excluded.parent_key,
		   updated_at = excluded.updated_at`,
		key, parent, now, now,
	).Error
}

// UpsertAttributeValuesLocked toggles creation of new values for a key.
func UpsertAttributeValuesLocked(
	db *gorm.DB,
	key string,
	locked bool,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, unassigned_label, parent_key, values_locked, created_at, updated_at)
		 VALUES (?, '', '', '', '', 0, 1, 0, '', '', ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   values_locked = excluded.values_locked,
		   updated_at = excluded.updated_at`,
		key, locked, now, now,
	).Error
}

// InsertAttributeKey creates a key-level metadata row.
func InsertAttributeKey(
	db *gorm.DB,
	key, label string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, unassigned_label, parent_key, values_locked, position, created_at, updated_at)
		 VALUES (?, '', ?, '', '', 0, 1, 0, '', '', 0,
		   (SELECT COALESCE(MAX(position), -1) + 1 FROM attributes_metadata WHERE value = ''),
		   ?, ?)`,
		key, label, now, now,
	).Error
}

// UpsertAttributeDescription stores a key-level or value-level description.
func UpsertAttributeDescription(
	db *gorm.DB,
	key, value, description string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, description, created_at, updated_at)
		 VALUES (?, ?, '', '', '', ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   description = excluded.description,
		   updated_at = excluded.updated_at`,
		key, value, description, now, now,
	).Error
}

// UpsertAttributeColor stores a key-level or value-level color.
func UpsertAttributeColor(
	db *gorm.DB,
	key, value, color string,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, color, created_at, updated_at)
		 VALUES (?, ?, '', '', '', ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   color = excluded.color,
		   updated_at = excluded.updated_at`,
		key, value, color, now, now,
	).Error
}

// SetAttributeValuePositions stores a custom order for a key's values.
func SetAttributeValuePositions(
	db *gorm.DB,
	key string,
	values []string,
	now time.Time,
) error {
	for position, value := range values {
		if value == "" {
			continue
		}
		if err := db.Exec(
			`INSERT INTO attributes_metadata (key, value, label, sort, render, position, created_at, updated_at)
			 VALUES (?, ?, '', '', '', ?, ?, ?)
			 ON CONFLICT(key, value) DO UPDATE SET
			   position = excluded.position,
			   updated_at = excluded.updated_at`,
			key, value, position, now, now,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// SetAttributeKeyPositions stores a custom order for attribute keys.
func SetAttributeKeyPositions(
	db *gorm.DB,
	keys []string,
	now time.Time,
) error {
	for position, key := range keys {
		if key == "" {
			continue
		}
		if err := db.Exec(
			`INSERT INTO attributes_metadata (key, value, label, sort, render, position, created_at, updated_at)
			 VALUES (?, '', '', '', '', ?, ?, ?)
			 ON CONFLICT(key, value) DO UPDATE SET
			   position = excluded.position,
			   updated_at = excluded.updated_at`,
			key, position, now, now,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpsertAttributeValueSearch toggles the per-key sidebar value search box.
func UpsertAttributeValueSearch(
	db *gorm.DB,
	key string,
	enabled bool,
	now time.Time,
) error {
	return db.Exec(
		`INSERT INTO attributes_metadata (key, value, label, sort, render, searchable, multi, include_unassigned, unassigned_label, parent_key, values_locked, value_search, created_at, updated_at)
		 VALUES (?, '', '', '', '', 0, 1, 0, '', '', 0, ?, ?, ?)
		 ON CONFLICT(key, value) DO UPDATE SET
		   value_search = excluded.value_search,
		   updated_at = excluded.updated_at`,
		key, enabled, now, now,
	).Error
}
