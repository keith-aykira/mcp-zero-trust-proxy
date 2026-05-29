package auth

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
	"github.com/rs/zerolog/log"
)

// Session holds the authenticated state for a single connected client.
type Session struct {
	// ID is the unique session identifier (UUID v4 format).
	ID string
	// ClientID is the provider-assigned user identifier.
	ClientID string
	// Role is the RBAC role assigned to this session.
	Role string
	// Email is the user's email address from the OAuth provider.
	Email string
	// CreatedAt is when the session was created.
	CreatedAt time.Time
	// LastAccess is updated on each successful Get() call.
	LastAccess time.Time
	// ExpiresAt is the absolute expiry time; extended on each access.
	ExpiresAt time.Time
	// Data is session-scoped key/value storage, isolated per session.
	Data map[string]interface{}
}

// SessionBackend defines the persistence interface for session storage.
// Implementations may use memory, local files, or remote stores.
type SessionBackend interface {
	// Save writes all sessions to persisted storage.
	Save(sessions map[string]*Session) error
	// Restore loads all sessions from persisted storage.
	Restore() (map[string]*Session, error)
}

// inMemoryBackend is the default backend — no persistence beyond process lifetime.
type inMemoryBackend struct{}

func (b *inMemoryBackend) Save(map[string]*Session) error { return nil }
func (b *inMemoryBackend) Restore() (map[string]*Session, error) {
	return make(map[string]*Session), nil
}

// fileBackend persists sessions to a JSON file on disk.
type fileBackend struct {
	path string
}

func (b *fileBackend) Save(sessions map[string]*Session) error {
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling sessions: %w", err)
	}
	tmp := b.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	return os.Rename(tmp, b.path)
}

func (b *fileBackend) Restore() (map[string]*Session, error) {
	sessions := make(map[string]*Session)
	data, err := os.ReadFile(b.path)
	if err != nil {
		if os.IsNotExist(err) {
			return sessions, nil
		}
		return nil, fmt.Errorf("reading session file: %w", err)
	}
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parsing sessions: %w", err)
	}
	return sessions, nil
}

// newSessionBackend creates a backend based on the session config.
// Default: in-memory (no persistence).
func newSessionBackend(cfg config.SessionStoreConfig) SessionBackend {
	switch cfg.Backend {
	case "file":
		path := cfg.Filepath
		if path == "" {
			path = "./sessions.json"
		}
		return &fileBackend{path: path}
	default:
		return &inMemoryBackend{}
	}
}

// SessionStore is a thread-safe store for active sessions with pluggable persistence.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
	stop     chan struct{}
	backend  SessionBackend
}

// NewSessionStore creates a SessionStore with the given TTL and starts a background
// cleanup/persistence goroutine that removes expired sessions every minute.
// If cfg.Backend is "file", sessions are persisted to disk periodically for cross-restart
// durability.
func NewSessionStore(cfg config.SessionStoreConfig) *SessionStore {
	ttl, err := time.ParseDuration(cfg.TTL)
	if err != nil || ttl <= 0 {
		ttl = 24 * time.Hour
	}
	backend := newSessionBackend(cfg)
	sessions, err := backend.Restore()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to restore sessions from backend; starting fresh")
		sessions = make(map[string]*Session)
	}
	s := &SessionStore{
		sessions: sessions,
		ttl:      ttl,
		stop:     make(chan struct{}),
		backend:  backend,
	}
	go s.cleanupLoop()
	return s
}

// newSessionStoreForTest creates a SessionStore with in-memory backend for unit test use.
func newSessionStoreForTest(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
		stop:     make(chan struct{}),
		backend:  &inMemoryBackend{},
	}
}

// Create generates a new session for the given ClientIdentity, stores it,
// and sets identity.SessionID to the new session's ID.
func (s *SessionStore) Create(identity *proxy.ClientIdentity) (*Session, error) {
	id, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("generating session ID: %w", err)
	}

	now := time.Now()
	session := &Session{
		ID:         id,
		ClientID:   identity.ClientID,
		Role:       identity.Role,
		Email:      identity.Email,
		CreatedAt:  now,
		LastAccess: now,
		ExpiresAt:  now.Add(s.ttl),
		Data:       make(map[string]interface{}),
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	identity.SessionID = id
	return session, nil
}

// Get returns the session for the given ID if it exists and has not expired.
// On success it refreshes LastAccess and extends ExpiresAt by the store TTL.
// Returns an error if the session does not exist or has expired.
func (s *SessionStore) Get(sessionID string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	if time.Now().After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		return nil, fmt.Errorf("session %q has expired", sessionID)
	}

	now := time.Now()
	session.LastAccess = now
	session.ExpiresAt = now.Add(s.ttl)

	return session, nil
}

// Delete removes a session by ID (e.g. on logout). No-op if not found.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// GetByClientID returns all active (non-expired) sessions for a given ClientID.
// Returns a copy of the slice to prevent race conditions if caller modifies it.
func (s *SessionStore) GetByClientID(clientID string) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var result []*Session
	for _, session := range s.sessions {
		if session.ClientID == clientID && now.Before(session.ExpiresAt) {
			result = append(result, session)
		}
	}
	// Return a copy to prevent race conditions if caller modifies the slice
	return append([]*Session(nil), result...), nil
}

// Stop halts the background cleanup goroutine for clean shutdown.
func (s *SessionStore) Stop() {
	close(s.stop)
}

// cleanup removes all expired sessions from the store.
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

// cleanupLoop runs cleanup() every minute until Stop() is called.
// After cleanup, persists sessions if a file backend is configured.
func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
			if err := s.backend.Save(s.sessions); err != nil {
				log.Warn().Err(err).Msg("Failed to persist sessions")
			}
		case <-s.stop:
			if err := s.backend.Save(s.sessions); err != nil {
				log.Warn().Err(err).Msg("Failed to persist sessions on shutdown")
			}
			return
		}
	}
}

// generateUUID produces a UUID v4 string using crypto/rand.
// Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (RFC 4122).
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", err
	}
	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	), nil
}
