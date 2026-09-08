package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/Mustenaka/astro-go-blog/api/internal/db"
)

// SessionTTL is how long an admin session stays valid without re-login.
const SessionTTL = 12 * time.Hour

// Sessions persists admin sessions in the sessions table.
type Sessions struct {
	db *db.DB
}

func NewSessions(d *db.DB) *Sessions { return &Sessions{db: d} }

// Create stores a new session and returns its opaque id (the cookie value).
func (s *Sessions) Create(ctx context.Context, username, ipHash string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw)
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (id, username, ip_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, username, ipHash, now.Format(time.RFC3339), now.Add(SessionTTL).Format(time.RFC3339))
	return id, err
}

// Lookup returns the username for a valid session id, or ErrNoSession.
func (s *Sessions) Lookup(ctx context.Context, id string) (string, error) {
	if len(id) != 64 {
		return "", ErrNoSession
	}
	var username, expires string
	err := s.db.QueryRowContext(ctx, `SELECT username, expires_at FROM sessions WHERE id = ?`, id).Scan(&username, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoSession
	}
	if err != nil {
		return "", err
	}
	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil || time.Now().UTC().After(exp) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
		return "", ErrNoSession
	}
	return username, nil
}

// Delete removes a session (logout).
func (s *Sessions) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// Purge drops expired sessions.
func (s *Sessions) Purge(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}

// ErrNoSession means the cookie is missing, unknown or expired.
var ErrNoSession = errors.New("no valid session")

// HashIP returns a keyed hash of a client IP so that raw addresses are never stored.
func HashIP(secret, ip string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// LoginLimiter locks an account/IP pair after too many failed attempts.
type LoginLimiter struct {
	mu       sync.Mutex
	failures map[string]*loginState
	Max      int
	Lock     time.Duration
	now      func() time.Time
}

type loginState struct {
	count       int
	lockedUntil time.Time
}

func NewLoginLimiter(maxFailures int, lock time.Duration) *LoginLimiter {
	return &LoginLimiter{failures: map[string]*loginState{}, Max: maxFailures, Lock: lock, now: time.Now}
}

// Locked reports whether the key is currently locked out and for how long.
func (l *LoginLimiter) Locked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.failures[key]
	if st == nil {
		return false, 0
	}
	if remaining := st.lockedUntil.Sub(l.now()); remaining > 0 {
		return true, remaining
	}
	return false, 0
}

// Fail records a failed attempt; the fifth failure locks the key.
func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.failures[key]
	if st == nil {
		st = &loginState{}
		l.failures[key] = st
	}
	st.count++
	if st.count >= l.Max {
		st.lockedUntil = l.now().Add(l.Lock)
		st.count = 0
	}
}

// Reset clears the failure count after a successful login.
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}
