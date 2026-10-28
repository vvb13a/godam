package domain

import "time"

// Preset is a named, reusable imgproxy processing pipeline. Name is the
// URL-safe key used in conversion URLs; Label is the display name.
type Preset struct {
	ID         string           `json:"id"`
	Label      string           `json:"label"`
	Name       string           `json:"name"`
	Parameters PresetParameters `json:"parameters"`
	// LibraryWide makes the preset available to every asset. Otherwise it is
	// available to assets whose collection is listed in the join table.
	LibraryWide   bool      `json:"library_wide"`
	CollectionIDs []string  `json:"collection_ids"`
	CreatedAt     time.Time `json:"created_at"`
	ModifiedAt    time.Time `json:"modified_at"`
}

// PresetParameters is the JSON document stored in presets.parameters. Format
// is the output extension; Options are imgproxy processing options in order.
type PresetParameters struct {
	Format  string         `json:"format"`
	Options []PresetOption `json:"options"`
}

type PresetOption struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}
