package domain

// Stats is the aggregate view of the library shown in the sidebar.
type Stats struct {
	Assets          int         `json:"assets"`
	Collections     int         `json:"collections"`
	Uncategorized   int         `json:"uncategorized"`
	StorageBytes    int64       `json:"storage_bytes"`
	Conversions     int         `json:"conversions"`
	ConversionBytes int64       `json:"conversion_bytes"`
	Types           []TypeCount `json:"types"`
}

// TypeCount is the asset count and storage volume for one MIME type.
type TypeCount struct {
	MimeType string `json:"mime_type"`
	Count    int    `json:"count"`
	Bytes    int64  `json:"bytes"`
}
