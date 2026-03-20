package auth

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
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

// SessionStore is a thread-safe in-memory store for active sessions.
type SessionStore struct {
	mu      sync.RWMutex
	sessions map[string]*Session
	ttl     time.Duration
	stop    chan struct{}
}

// NewSessionStore creates a SessionStore with the given TTL and starts a background
// cleanup goroutine that removes expired sessions every minute.
func NewSessionStore(ttl time.Duration) *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]*Session),
		ttl:     ttl,
		stop:    make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
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
	return result, nil
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
func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stop:
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
