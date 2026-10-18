package domain

import "time"

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
	Metadata       Metadata     `json:"metadata"`
	Colors         []AssetColor `json:"colors,omitempty"`
	Tags           []Tag        `json:"tags,omitempty"`
	Collections    []Collection `json:"collections,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// AssetFilter narrows an asset listing. CollectionID restricts to a single
// collection; TagIDs selects assets carrying any of the given tags;
// Uncategorized and Untagged select assets belonging to no collection or
// carrying no tags respectively. Search matches title, note and filename.
// Sort is a frontend sort key; Cursor is the opaque keyset position. Filters
// combine (AND).
type AssetColor struct {
	Hex        string  `json:"hex"`
	Bucket     string  `json:"bucket"`
	Percentage float64 `json:"percentage"`
}

type AssetFilter struct {
	CollectionID  *string
	TagIDs        []string
	Uncategorized bool
	Untagged      bool
	Search        string
	Type          string
	Color         string
	Sort          string
	Cursor        string
	Limit         int
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
