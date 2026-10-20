package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
)

// UploadChunkSize is the maximum number of bytes accepted by one chunk request.
// Clients read the value from the session response and slice large files
// accordingly.
const UploadChunkSize int64 = 8 << 20

const uploadSessionTTL = 24 * time.Hour

// UploadSession is the client-facing description of an in-progress upload.
type UploadSession struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	Received  int64     `json:"received"`
	ChunkSize int64     `json:"chunk_size"`
	CreatedAt time.Time `json:"created_at"`
}

type uploadSession struct {
	UploadSession
	UserID       string
	Driver       string
	Path         string
	Done         bool
	LastActivity time.Time
	mu           sync.Mutex
}

type uploadManager struct {
	dir      string
	mu       sync.Mutex
	sessions map[string]*uploadSession
}

func newUploadManager(dir string) *uploadManager {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "godam-uploads")
	}
	return &uploadManager{
		dir:      dir,
		sessions: make(map[string]*uploadSession),
	}
}

func (m *uploadManager) create(userID, driver, filename, mimeType string, size int64) (*UploadSession, error) {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return nil, fmt.Errorf("create upload staging directory: %w", err)
	}

	id := uuid.NewString()
	path := filepath.Join(m.dir, id+".part")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create upload staging file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}

	now := time.Now().UTC()
	session := &uploadSession{
		UploadSession: UploadSession{
			ID:        id,
			Filename:  filename,
			MimeType:  mimeType,
			Size:      size,
			ChunkSize: UploadChunkSize,
			CreatedAt: now,
		},
		UserID:       userID,
		Driver:       driver,
		Path:         path,
		LastActivity: now,
	}

	m.mu.Lock()
	m.pruneLocked(now)
	m.sessions[id] = session
	m.mu.Unlock()

	return session.snapshot(), nil
}

func (m *uploadManager) lookup(id, userID string) (*uploadSession, error) {
	m.mu.Lock()
	session := m.sessions[id]
	m.mu.Unlock()
	if session == nil || session.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return session, nil
}

// detach removes a session from the active map and marks it as finished so
// concurrent chunk writes cannot race with completion.
func (m *uploadManager) detach(id, userID string) (*uploadSession, error) {
	session, err := m.lookup(id, userID)
	if err != nil {
		return nil, err
	}

	session.mu.Lock()
	if session.Done {
		session.mu.Unlock()
		return nil, domain.ErrNotFound
	}
	session.Done = true
	session.mu.Unlock()

	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
	return session, nil
}

func (s *uploadSession) snapshot() *UploadSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.UploadSession
	copy.Received = s.Received
	return &copy
}

// pruneLocked removes expired sessions and orphan staging files. The caller
// must hold m.mu.
func (m *uploadManager) pruneLocked(now time.Time) {
	for id, session := range m.sessions {
		session.mu.Lock()
		expired := now.Sub(session.LastActivity) > uploadSessionTTL
		if expired {
			session.Done = true
		}
		session.mu.Unlock()
		if expired {
			delete(m.sessions, id)
			_ = os.Remove(session.Path)
		}
	}

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) <= uploadSessionTTL {
			continue
		}
		_ = os.Remove(filepath.Join(m.dir, entry.Name()))
	}
}

func (m *uploadManager) write(id, userID string, offset int64, reader io.Reader) (int64, error) {
	session, err := m.lookup(id, userID)
	if err != nil {
		return 0, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	if session.Done {
		return 0, domain.ErrNotFound
	}
	if offset != session.Received {
		return 0, fmt.Errorf("%w: offset %d does not match received %d", ErrInvalidInput, offset, session.Received)
	}

	remaining := session.Size - offset
	if remaining <= 0 {
		return 0, fmt.Errorf("%w: upload already complete", ErrInvalidInput)
	}

	file, err := os.OpenFile(session.Path, os.O_WRONLY, 0)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	written, copyErr := io.Copy(file, io.LimitReader(reader, remaining+1))
	if copyErr != nil {
		_ = file.Truncate(offset)
		return 0, copyErr
	}
	if written > remaining {
		_ = file.Truncate(offset)
		return 0, fmt.Errorf("%w: chunk exceeds expected upload size", ErrInvalidInput)
	}
	if err := file.Truncate(offset + written); err != nil {
		return 0, err
	}

	session.Received += written
	session.LastActivity = time.Now().UTC()
	return written, nil
}

func (m *uploadManager) remove(id, userID string) error {
	session, err := m.detach(id, userID)
	if err != nil {
		return err
	}
	return os.Remove(session.Path)
}
