package domain

// Stats is the aggregate view of the library shown on the dashboard.
type Stats struct {
	Assets       int         `json:"assets"`
	Collections  int         `json:"collections"`
	Tags         int         `json:"tags"`
	StorageBytes int64       `json:"storage_bytes"`
	Types        []TypeCount `json:"types"`
}

// TypeCount is the asset count and storage volume for one MIME type.
type TypeCount struct {
	MimeType string `json:"mime_type"`
	Count    int    `json:"count"`
	Bytes    int64  `json:"bytes"`
}
