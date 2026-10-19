package domain

import "time"

// Conversion is one rendered derivative of an asset. Conversions are derived
// files in the conversion store, not rows in the asset database. Kind is
// "preset", "size" or "signed"; Preset is the preset name for preset
// conversions and Label its display label when the preset still exists.
type Conversion struct {
	Key        string    `json:"key"`
	URL        string    `json:"url"`
	Kind       string    `json:"kind"`
	Preset     string    `json:"preset,omitempty"`
	Label      string    `json:"label,omitempty"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Format     string    `json:"format"`
	ByteSize   int64     `json:"byte_size"`
	ModifiedAt time.Time `json:"modified_at"`
}
