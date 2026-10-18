package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already in use")
	ErrInvalidInput       = errors.New("invalid input")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

const (
	sessionTTL  = 30 * 24 * time.Hour
	devEmail    = "dev@godam.local"
	devUserName = "Yi Sun-sin"
)

type sessionEntry struct {
	userID    string
	expiresAt time.Time
}

// UserService handles accounts and cookie-backed sessions. Active users and
// sessions are held in memory (the app is a single process with a modest user
// base), with the database as a durable write-through backing so sessions
// survive a restart. The maps are guarded by a mutex; cached domain.User
// pointers are replaced, never mutated, on updates.
type UserService struct {
	db       *gorm.DB
	mu       sync.RWMutex
	users    map[string]*domain.User
	sessions map[string]sessionEntry
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		db:       db,
		users:    make(map[string]*domain.User),
		sessions: make(map[string]sessionEntry),
	}
}

// Load primes the in-memory caches from the database. It is called once at
// startup; a failure is non-fatal because every lookup falls back to the
// database and repopulates the cache.
func (s *UserService) Load(ctx context.Context) error {
	var users []store.User
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
		return err
	}
	var sessions []store.Session
	if err := s.db.WithContext(ctx).
		Where("expires_at > ?", time.Now().UTC()).
		Find(&sessions).Error; err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range users {
		s.users[users[i].ID] = users[i].ToDomain()
	}
	for i := range sessions {
		s.sessions[sessions[i].Token] = sessionEntry{
			userID:    sessions[i].UserID,
			expiresAt: sessions[i].ExpiresAt,
		}
	}
	return nil
}

func (s *UserService) cacheUser(user *domain.User) {
	s.mu.Lock()
	s.users[user.ID] = user
	s.mu.Unlock()
}

func (s *UserService) cachedUser(id string) *domain.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[id]
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if user := s.cachedUser(id); user != nil {
		return user, nil
	}
	var m store.User
	err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user := m.ToDomain()
	s.cacheUser(user)
	return user, nil
}

// List returns all users ordered by name.
func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	var models []store.User
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	users := make([]domain.User, 0, len(models))
	for i := range models {
		users = append(users, *models[i].ToDomain())
	}
	return users, nil
}

// Create inserts a user with a bcrypt-hashed password.
func (s *UserService) Create(ctx context.Context, name, email, password string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)
	if name == "" || email == "" {
		return nil, ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	m := store.User{
		ID:           uuid.New().String(),
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	user := m.ToDomain()
	s.cacheUser(user)
	return user, nil
}

// Authenticate verifies an email/password pair. Login is infrequent, so it
// reads through the database rather than an email index.
func (s *UserService) Authenticate(ctx context.Context, email, password string) (*domain.User, error) {
	email = normalizeEmail(email)
	var m store.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if m.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	user := m.ToDomain()
	s.cacheUser(user)
	return user, nil
}

// DevUser returns the seeded developer account, creating it when missing.
func (s *UserService) DevUser(ctx context.Context) (*domain.User, error) {
	var m store.User
	err := s.db.WithContext(ctx).Where("email = ?", devEmail).First(&m).Error
	if err == nil {
		user := m.ToDomain()
		s.cacheUser(user)
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	m = store.User{
		ID:        uuid.New().String(),
		Name:      devUserName,
		Email:     devEmail,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		// A concurrent request may have created it first.
		if e := s.db.WithContext(ctx).Where("email = ?", devEmail).First(&m).Error; e != nil {
			return nil, err
		}
	}
	user := m.ToDomain()
	s.cacheUser(user)
	return user, nil
}

// UpdateProfile changes the display name and email, rejecting duplicates.
func (s *UserService) UpdateProfile(ctx context.Context, id, name, email string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)
	if name == "" || email == "" {
		return nil, ErrInvalidInput
	}
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&store.User{}).
		Where("email = ? AND id <> ?", email, id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrEmailTaken
	}
	res := s.db.WithContext(ctx).
		Model(&store.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"name": name, "email": email, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, domain.ErrNotFound
	}
	var m store.User
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	user := m.ToDomain()
	s.cacheUser(user)
	return user, nil
}

// ChangePassword verifies the current password (unless none is set yet) and
// stores a bcrypt hash of the new one. The hash is not cached (it is not part
// of the domain model), so active sessions remain valid.
func (s *UserService) ChangePassword(ctx context.Context, id, current, next string) error {
	if len(next) < 8 {
		return ErrWeakPassword
	}
	var m store.User
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	if m.PasswordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(current)); err != nil {
			return ErrInvalidCredentials
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).
		Model(&store.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"password_hash": string(hash), "updated_at": time.Now().UTC()}).Error
}

// StartSession creates a durable session row and caches it.
func (s *UserService) StartSession(ctx context.Context, userID string) (string, error) {
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	session := store.Session{
		Token:     token,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(sessionTTL),
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return "", err
	}
	s.mu.Lock()
	s.sessions[token] = sessionEntry{userID: userID, expiresAt: session.ExpiresAt}
	s.mu.Unlock()
	return token, nil
}

// UserForSession resolves a session token to its user. The in-memory cache is
// authoritative; a miss falls back to the database (e.g. sessions created
// before startup) and repopulates the cache. Expired sessions are dropped.
func (s *UserService) UserForSession(ctx context.Context, token string) (*domain.User, error) {
	if token == "" {
		return nil, ErrInvalidCredentials
	}
	now := time.Now().UTC()

	s.mu.RLock()
	entry, ok := s.sessions[token]
	s.mu.RUnlock()
	if ok {
		if now.After(entry.expiresAt) {
			s.dropSession(ctx, token)
			return nil, ErrInvalidCredentials
		}
		return s.GetByID(ctx, entry.userID)
	}

	var session store.Session
	if err := s.db.WithContext(ctx).First(&session, "token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if now.After(session.ExpiresAt) {
		s.dropSession(ctx, token)
		return nil, ErrInvalidCredentials
	}
	s.mu.Lock()
	s.sessions[token] = sessionEntry{userID: session.UserID, expiresAt: session.ExpiresAt}
	s.mu.Unlock()
	return s.GetByID(ctx, session.UserID)
}

func (s *UserService) dropSession(ctx context.Context, token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	_ = s.db.WithContext(ctx).Delete(&store.Session{}, "token = ?", token).Error
}

func (s *UserService) EndSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	return s.db.WithContext(ctx).Delete(&store.Session{}, "token = ?", token).Error
}

func newSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
