package domain

import "time"

// BackupSettings is the API-facing view of a library's backup destination.
// Password material is never exposed; HasPassword only tells the UI whether a
// secret is already stored.
type BackupSettings struct {
	Enabled            bool       `json:"enabled"`
	Driver             string     `json:"driver"`
	Host               string     `json:"host"`
	Port               int        `json:"port"`
	User               string     `json:"user"`
	BasePath           string     `json:"base_path"`
	KeyPath            string     `json:"key_path"`
	HostKeyFingerprint string     `json:"host_key_fingerprint"`
	HasPassword        bool       `json:"has_password"`
	LastRunAt          *time.Time `json:"last_run_at,omitempty"`
	LastError          string     `json:"last_error"`
}

// BackupStatus summarises how many assets are copied to the library's backup
// destination. Pending includes assets that have never been attempted.
type BackupStatus struct {
	Enabled   bool       `json:"enabled"`
	Total     int64      `json:"total"`
	Synced    int64      `json:"synced"`
	Pending   int64      `json:"pending"`
	Failed    int64      `json:"failed"`
	LastError string     `json:"last_error"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
}
