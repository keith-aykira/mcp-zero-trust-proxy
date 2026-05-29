package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

func makeIdentity(clientID, email string) *proxy.ClientIdentity {
	return &proxy.ClientIdentity{
		ClientID: clientID,
		Email:    email,
		Role:     "readonly",
	}
}

func TestSessionCreateReturnsUniqueID(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	id1 := makeIdentity("client-1", "a@example.com")
	id2 := makeIdentity("client-2", "b@example.com")

	s1, err := store.Create(id1)
	if err != nil {
		t.Fatalf("Create(client-1) error: %v", err)
	}
	s2, err := store.Create(id2)
	if err != nil {
		t.Fatalf("Create(client-2) error: %v", err)
	}

	if s1.ID == s2.ID {
		t.Error("two sessions should have different IDs")
	}
	if s1.ID == "" || s2.ID == "" {
		t.Error("session IDs must not be empty")
	}
}

func TestSessionCreateSetsIdentitySessionID(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	identity := makeIdentity("user-x", "x@example.com")
	session, err := store.Create(identity)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if identity.SessionID != session.ID {
		t.Errorf("identity.SessionID = %q, want %q", identity.SessionID, session.ID)
	}
}

func TestSessionGetValid(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	identity := makeIdentity("user-get", "get@example.com")
	created, _ := store.Create(identity)

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("got.ID = %q, want %q", got.ID, created.ID)
	}
	if got.ClientID != "user-get" {
		t.Errorf("got.ClientID = %q, want user-get", got.ClientID)
	}
	if got.Email != "get@example.com" {
		t.Errorf("got.Email = %q, want get@example.com", got.Email)
	}
}

func TestSessionGetUnknownID(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	_, err := store.Get("nonexistent-session-id")
	if err == nil {
		t.Error("Get() with unknown ID should return an error")
	}
}

func TestSessionIsolation(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	idA := makeIdentity("client-A", "a@example.com")
	idB := makeIdentity("client-B", "b@example.com")

	sessA, _ := store.Create(idA)
	sessB, _ := store.Create(idB)

	// Verify IDs are different
	if sessA.ID == sessB.ID {
		t.Error("client A and client B should have different session IDs")
	}

	// Write to A's Data, verify B's Data is unaffected
	sessA.Data["key"] = "value-from-A"
	if sessB.Data["key"] != nil {
		t.Errorf("client B's session Data should not see client A's data, got %v", sessB.Data["key"])
	}

	// A cannot access B's session by B's ID
	_, err := store.Get(sessB.ID)
	if err != nil {
		t.Fatalf("Get(B.ID) should succeed, got error: %v", err)
	}
	// B's data should still not have A's key
	gotB, _ := store.Get(sessB.ID)
	if gotB.Data["key"] != nil {
		t.Error("client B's session still should not have client A's data key")
	}
}

func TestSessionClientACannotReadClientBSessionByAID(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	idA := makeIdentity("client-A", "a@example.com")
	idB := makeIdentity("client-B", "b@example.com")

	sessA, _ := store.Create(idA)
	_, _ = store.Create(idB)

	// Lookup with A's session ID should return A's data, not B's
	got, err := store.Get(sessA.ID)
	if err != nil {
		t.Fatalf("Get(A.ID) error: %v", err)
	}
	if got.ClientID != "client-A" {
		t.Errorf("Get(A.ID).ClientID = %q, want client-A", got.ClientID)
	}
}

func TestSessionExpiry(t *testing.T) {
	// Use very short TTL
	store := newSessionStoreForTest(50 * time.Millisecond)
	defer store.Stop()

	identity := makeIdentity("user-exp", "exp@example.com")
	session, _ := store.Create(identity)

	// Session should be accessible immediately
	_, err := store.Get(session.ID)
	if err != nil {
		t.Fatalf("Get() immediately after create should succeed, got: %v", err)
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	_, err = store.Get(session.ID)
	if err == nil {
		t.Error("Get() after TTL should return an error (session expired)")
	}
}

func TestSessionRefreshOnAccess(t *testing.T) {
	// Use moderate TTL
	store := newSessionStoreForTest(200 * time.Millisecond)
	defer store.Stop()

	identity := makeIdentity("user-refresh", "refresh@example.com")
	session, _ := store.Create(identity)

	// Access every 100ms — each Get should extend expiry, session stays alive
	for i := 0; i < 3; i++ {
		time.Sleep(100 * time.Millisecond)
		_, err := store.Get(session.ID)
		if err != nil {
			t.Fatalf("Get() at iteration %d should succeed (TTL refreshed on access): %v", i, err)
		}
	}
}

func TestSessionDelete(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	identity := makeIdentity("user-del", "del@example.com")
	session, _ := store.Create(identity)

	store.Delete(session.ID)

	_, err := store.Get(session.ID)
	if err == nil {
		t.Error("Get() after Delete() should return error")
	}
}

func TestSessionDeleteNoOp(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	// Should not panic on unknown ID
	store.Delete("nonexistent-id")
}

func TestSessionGetByClientID(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	// Create 3 sessions for client-multi, 1 for other client
	for i := 0; i < 3; i++ {
		store.Create(makeIdentity("client-multi", fmt.Sprintf("user%d@example.com", i))) //nolint:errcheck
	}
	store.Create(makeIdentity("other-client", "other@example.com")) //nolint:errcheck

	sessions, err := store.GetByClientID("client-multi")
	if err != nil {
		t.Fatalf("GetByClientID() error: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("GetByClientID(client-multi) returned %d sessions, want 3", len(sessions))
	}
	for _, s := range sessions {
		if s.ClientID != "client-multi" {
			t.Errorf("GetByClientID returned session with ClientID %q, want client-multi", s.ClientID)
		}
	}
}

func TestSessionConcurrentCreation(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	const goroutines = 100
	var wg sync.WaitGroup
	ids := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			identity := makeIdentity(fmt.Sprintf("client-%d", n), fmt.Sprintf("user%d@example.com", n))
			session, err := store.Create(identity)
			if err != nil {
				t.Errorf("concurrent Create() error: %v", err)
				return
			}
			ids <- session.ID
		}(i)
	}

	wg.Wait()
	close(ids)

	// Verify all IDs are unique
	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate session ID found: %q", id)
		}
		seen[id] = true
	}
	if len(seen) != goroutines {
		t.Errorf("expected %d unique session IDs, got %d", goroutines, len(seen))
	}
}

func TestSessionStoresClientMetadata(t *testing.T) {
	store := newSessionStoreForTest(1 * time.Hour)
	defer store.Stop()

	identity := makeIdentity("meta-user", "meta@example.com")
	identity.Role = "admin"

	session, err := store.Create(identity)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if session.ClientID != "meta-user" {
		t.Errorf("session.ClientID = %q, want meta-user", session.ClientID)
	}
	if session.Role != "admin" {
		t.Errorf("session.Role = %q, want admin", session.Role)
	}
	if session.Email != "meta@example.com" {
		t.Errorf("session.Email = %q, want meta@example.com", session.Email)
	}
}
