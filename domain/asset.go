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
	Tags           []Tag        `json:"tags,omitempty"`
	Collections    []Collection `json:"collections,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// listing to a single library; CollectionID restricts to a single collection;
// TagIDs selects assets carrying any of the given tags; Uncategorized and
// Untagged select assets belonging to no collection or carrying no tags
// respectively. Filters combine (AND).
type AssetFilter struct {
	CollectionID  *string
	TagIDs        []string
	Uncategorized bool
	Untagged      bool
}
