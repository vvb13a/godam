package domain

import "time"

const (
	AssetRoleOriginal = "original"
	AssetRoleVisual   = "visual"
)

type Asset struct {
	ID             string       `json:"id"`
	UploadedBy     string       `json:"uploaded_by,omitempty"`
	UploadedByName string       `json:"uploaded_by_name,omitempty"`
	Filename       string       `json:"filename"`
	Title          string       `json:"title,omitempty"`
	Note           string       `json:"note,omitempty"`
	StorageDriver  string       `json:"storage_driver"`
	StorageKey     string       `json:"storage_key"`
	MimeType       string       `json:"mime_type"`
	ByteSize       int64        `json:"byte_size"`
	Role           string       `json:"role,omitempty"`
	ImageOf        string       `json:"image_of,omitempty"`
	Metadata       Metadata     `json:"metadata"`
	Colors         []AssetColor `json:"colors,omitempty"`
	Tags           []Tag        `json:"tags,omitempty"`
	Collections    []Collection `json:"collections,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// AssetColor is one extracted palette entry.
type AssetColor struct {
	Hex        string  `json:"hex"`
	Bucket     string  `json:"bucket"`
	Percentage float64 `json:"percentage"`
}

// AssetFilter narrows an asset listing. List-valued filters (collections,
// tags, types, colors, MIME types and uploaders) use OR within the list;
// different filters combine with AND. Uncategorized and Untagged include
// assets with no collection or tags in the corresponding OR group. Search
// matches title, note and filename. Sort is a frontend sort key; Cursor is the
// opaque keyset position.

type AssetFilter struct {
	CollectionIDs  []string
	TagIDs         []string
	PresetID       string
	PresetExclude  bool
	Uncategorized  bool
	Untagged       bool
	Search         string
	Types          []string
	Colors         []string
	MimeTypes      []string
	UploadedBy     []string
	MinDuration    *float64
	MaxDuration    *float64
	MinPages       *int
	MaxPages       *int
	MinWidth       *int
	MaxWidth       *int
	MinHeight      *int
	MaxHeight      *int
	MinSize        *int64
	MaxSize        *int64
	AddedAfter     *time.Time
	AddedBefore    *time.Time
	ModifiedAfter  *time.Time
	ModifiedBefore *time.Time
	Sort           string
	Cursor         string
	Limit          int
}

// AssetPage is one cursor-paginated slice of assets. Total is capped at 1000
// and TotalCapped reports that more rows exist.
type AssetPage struct {
	Items       []Asset `json:"items"`
	NextCursor  string  `json:"next_cursor,omitempty"`
	Total       int64   `json:"total"`
	TotalBytes  int64   `json:"total_bytes"`
	TotalCapped bool    `json:"total_capped"`
}
