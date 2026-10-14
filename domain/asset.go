package domain

import "time"

type Asset struct {
	ID            string       `json:"id"`
	LibraryID     string       `json:"library_id"`
	Filename      string       `json:"filename"`
	StorageDriver string       `json:"storage_driver"`
	StorageKey    string       `json:"storage_key"`
	MimeType      string       `json:"mime_type"`
	ByteSize      int64        `json:"byte_size"`
	Metadata      Metadata     `json:"metadata"`
	Tags          []Tag        `json:"tags,omitempty"`
	Collections   []Collection `json:"collections,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
}

// AssetFilter narrows an asset listing. LibraryID is mandatory and scopes the
// listing to a single library; CollectionID restricts to a single collection;
// TagIDs selects assets carrying any of the given tags; Uncategorized and
// Untagged select assets belonging to no collection or carrying no tags
// respectively. Filters combine (AND).
type AssetFilter struct {
	LibraryID     string
	CollectionID  *string
	TagIDs        []string
	Uncategorized bool
	Untagged      bool
}
