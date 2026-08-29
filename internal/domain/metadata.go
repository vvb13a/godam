package domain

type Metadata struct {
	// Image / Video properties
	Width       *int     `json:"width,omitempty"`
	Height      *int     `json:"height,omitempty"`
	DurationSec *float64 `json:"duration_sec,omitempty"`

	// Document properties
	PageCount *int `json:"page_count,omitempty"`

	// Unstructured / format-specific extra metadata
	RawJSON string `json:"raw_json,omitempty"`
}
