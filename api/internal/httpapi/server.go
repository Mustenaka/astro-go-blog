// Package httpapi wires the HTTP surface: public counters, admin API, GitHub webhook,
// local storage endpoints and the embedded admin UI.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Mustenaka/astro-go-blog/api/internal/adminui"
	"github.com/Mustenaka/astro-go-blog/api/internal/auth"
	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/config"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

// Server holds every dependency the handlers need.
type Server struct {
	cfg      *config.Config
	db       *db.DB
	store    storage.Storage
	local    *storage.Local // non-nil only for local storage
	sessions *auth.Sessions
	logins   *auth.LoginLimiter
	builds   *build.Runner
	log      *slog.Logger

	publicLimiter *rateLimiter
	loginLimiter  *rateLimiter
	views         *viewDedupe
}

// Options customise construction; zero values are fine.
type Options struct {
	Logger *slog.Logger
}

// New builds a Server. store may also be a *storage.Local, in which case the upload and
// asset routes are mounted.
func New(cfg *config.Config, d *db.DB, store storage.Storage, builds *build.Runner, opts Options) *Server {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		cfg:           cfg,
		db:            d,
		store:         store,
		sessions:      auth.NewSessions(d),
		logins:        auth.NewLoginLimiter(5, 15*time.Minute),
		builds:        builds,
		log:           log,
		publicLimiter: newRateLimiter(30, 60), // 30 requests/min per client, burst 60
		loginLimiter:  newRateLimiter(10, 10),
		views:         newViewDedupe(10 * time.Minute),
	}
	if l, ok := store.(*storage.Local); ok {
		s.local = l
	}
	return s
}

// Handler returns the routed handler with global middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": db.Now()})
	})

	// Public API (CORS + rate limit on writes).
	public := http.NewServeMux()
	public.HandleFunc("GET /api/v1/counters", s.handleGetCounters)
	public.HandleFunc("POST /api/v1/counters/{slug}/view", s.limited(s.publicLimiter, s.handleView))
	public.HandleFunc("POST /api/v1/likes/{slug}", s.limited(s.publicLimiter, s.handleLike))
	mux.Handle("/api/v1/", s.cors(public))

	// Admin API (session required except login).
	mux.HandleFunc("POST /admin/api/login", s.limited(s.loginLimiter, s.handleLogin))
	mux.HandleFunc("POST /admin/api/logout", s.requireSession(s.handleLogout))
	mux.HandleFunc("GET /admin/api/me", s.requireSession(s.handleMe))
	mux.HandleFunc("POST /admin/api/assets/presign", s.requireSession(s.handlePresign))
	mux.HandleFunc("POST /admin/api/assets/complete", s.requireSession(s.handleComplete))
	mux.HandleFunc("GET /admin/api/assets", s.requireSession(s.handleListAssets))
	mux.HandleFunc("DELETE /admin/api/assets/{id}", s.requireSession(s.handleDeleteAsset))
	mux.HandleFunc("POST /admin/api/build", s.requireSession(s.handleTriggerBuild))
	mux.HandleFunc("GET /admin/api/build", s.requireSession(s.handleRecentBuilds))
	mux.HandleFunc("GET /admin/api/build/{id}", s.requireSession(s.handleGetBuild))
	mux.HandleFunc("GET /admin/api/stats", s.requireSession(s.handleStats))

	// Embedded admin SPA.
	if adminui.Available() {
		mux.Handle("/admin/", adminui.Handler())
		mux.Handle("GET /admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
	} else {
		mux.HandleFunc("/admin/", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "admin UI not embedded; run `pnpm build:admin` and rebuild the server", http.StatusNotFound)
		})
	}

	// GitHub webhook.
	mux.HandleFunc("POST /hooks/github", s.handleGitHubWebhook)

	// Local storage simulation of the CDN and the pre-signed upload target.
	if s.local != nil {
		mux.Handle("PUT /_local/upload/{token}", s.local.UploadHandler())
		mux.Handle("GET /assets/{key...}", s.local.ServeHandler())
		mux.Handle("HEAD /assets/{key...}", s.local.ServeHandler())
		mux.HandleFunc("OPTIONS /assets/{key...}", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
		})
	}

	return s.recoverer(s.securityHeaders(s.requestLog(mux)))
}

/* ---------- middleware ---------- */

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Assets may be embedded in iframes (Unity builds); everything else must not be framed.
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			h.Set("X-Frame-Options", "DENY")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "err", rec, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors allows browsers on CORS_ORIGINS to call the public API. Admin and hooks get no CORS.
func (s *Server) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range s.cfg.CORSOrigins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limited(l *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(s.ipHash(r)) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "too many requests")
			return
		}
		next(w, r)
	}
}

const sessionCookie = "admin_session"

func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		user, err := s.sessions.Lookup(r.Context(), c.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser{}, user)
		next(w, r.WithContext(ctx))
	}
}

type ctxUser struct{}

/* ---------- helpers ---------- */

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,99}$`)

// clientIP trusts proxy headers only when the direct peer is loopback (Nginx on the same host).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
			return real
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	return host
}

func (s *Server) ipHash(r *http.Request) string {
	return auth.HashIP(s.cfg.SessionSecret, clientIP(r))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("unexpected trailing data")
	}
	return nil
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

/* ---------- token bucket rate limiter ---------- */

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	last    time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
}

func newRateLimiter(perMinute, burst int) *rateLimiter {
	return &rateLimiter{buckets: map[string]*bucket{}, rate: float64(perMinute) / 60, burst: float64(burst), last: time.Now()}
}

func (l *rateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.last) > 5*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.at) > 5*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.last = now
	}
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.at).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

/* ---------- view dedupe (in memory, 10 minutes) ---------- */

type viewDedupe struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

func newViewDedupe(ttl time.Duration) *viewDedupe {
	return &viewDedupe{seen: map[string]time.Time{}, ttl: ttl}
}

// First reports whether key has not been seen within the TTL, recording it if so.
func (v *viewDedupe) First(key string) bool {
	now := time.Now()
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.seen) > 10000 {
		for k, t := range v.seen {
			if now.Sub(t) > v.ttl {
				delete(v.seen, k)
			}
		}
	}
	if t, ok := v.seen[key]; ok && now.Sub(t) < v.ttl {
		return false
	}
	v.seen[key] = now
	return true
}
