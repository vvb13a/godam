package api

import (
	"net/http"

	"github.com/vvb13a/godam/service"
)

type libraryBackupRequest struct {
	Enabled            bool   `json:"enabled"`
	Driver             string `json:"driver"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password"`
	ClearPassword      bool   `json:"clear_password"`
	KeyPath            string `json:"key_path"`
	BasePath           string `json:"base_path"`
	HostKeyFingerprint string `json:"host_key_fingerprint"`
}

func (r libraryBackupRequest) toService() service.BackupSettingsRequest {
	return service.BackupSettingsRequest{
		Enabled:            r.Enabled,
		Driver:             r.Driver,
		Host:               r.Host,
		Port:               r.Port,
		User:               r.User,
		Password:           r.Password,
		ClearPassword:      r.ClearPassword,
		KeyPath:            r.KeyPath,
		BasePath:           r.BasePath,
		HostKeyFingerprint: r.HostKeyFingerprint,
	}
}

func (s *Server) handleGetLibraryBackup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.requireOwnedLibrary(w, r, id) {
		return
	}
	settings, err := s.deps.Backups.GetSettings(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleSetLibraryBackup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.requireOwnedLibrary(w, r, id) {
		return
	}
	var req libraryBackupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	settings, err := s.deps.Backups.SetSettings(r.Context(), id, req.toService())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleTestLibraryBackup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.requireOwnedLibrary(w, r, id) {
		return
	}
	var req libraryBackupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.deps.Backups.TestConnection(r.Context(), id, req.toService()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRunLibraryBackup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.requireOwnedLibrary(w, r, id) {
		return
	}
	queued, err := s.deps.Backups.BackupNow(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"queued": queued})
}

func (s *Server) handleLibraryBackupStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.requireOwnedLibrary(w, r, id) {
		return
	}
	status, err := s.deps.Backups.Status(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}
