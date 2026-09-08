package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/Mustenaka/astro-go-blog/api/internal/db"
)

// Counter is the public view of a slug's numbers.
type Counter struct {
	Slug  string `json:"slug"`
	Views int64  `json:"views"`
	Likes int64  `json:"likes"`
}

// GET /api/v1/counters?slugs=a,b,c
func (s *Server) handleGetCounters(w http.ResponseWriter, r *http.Request) {
	var slugs []string
	for _, raw := range strings.Split(r.URL.Query().Get("slugs"), ",") {
		if slug := strings.TrimSpace(raw); slug != "" && slugRe.MatchString(slug) {
			slugs = append(slugs, slug)
		}
		if len(slugs) >= 50 {
			break
		}
	}
	out := make([]Counter, 0, len(slugs))
	for _, slug := range slugs {
		c := Counter{Slug: slug}
		err := s.db.QueryRowContext(r.Context(), `SELECT views, likes FROM counters WHERE slug = ?`, slug).Scan(&c.Views, &c.Likes)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"counters": out})
}

// POST /api/v1/counters/{slug}/view — one view per client per 10 minutes.
func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "invalid slug")
		return
	}
	counted := false
	if s.views.First(slug + "|" + s.ipHash(r)) {
		if _, err := s.db.ExecContext(r.Context(), `INSERT INTO counters (slug, views, likes, updated_at) VALUES (?, 1, 0, ?)
			ON CONFLICT(slug) DO UPDATE SET views = views + 1, updated_at = excluded.updated_at`, slug, db.Now()); err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		counted = true
	}
	c, err := s.counter(r, slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slug": slug, "views": c.Views, "likes": c.Likes, "counted": counted})
}

// POST /api/v1/likes/{slug} — one like per client per UTC day; repeats return the current value.
func (s *Server) handleLike(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugRe.MatchString(slug) {
		writeError(w, http.StatusBadRequest, "invalid slug")
		return
	}
	ctx := r.Context()
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO like_events (slug, ip_hash, day, created_at) VALUES (?, ?, ?, ?)`,
		slug, s.ipHash(r), db.Today(), db.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	liked := false
	if n, _ := res.RowsAffected(); n == 1 {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO counters (slug, views, likes, updated_at) VALUES (?, 0, 1, ?)
			ON CONFLICT(slug) DO UPDATE SET likes = likes + 1, updated_at = excluded.updated_at`, slug, db.Now()); err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		liked = true
	}
	c, err := s.counter(r, slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slug": slug, "views": c.Views, "likes": c.Likes, "liked": liked})
}

func (s *Server) counter(r *http.Request, slug string) (Counter, error) {
	c := Counter{Slug: slug}
	err := s.db.QueryRowContext(r.Context(), `SELECT views, likes FROM counters WHERE slug = ?`, slug).Scan(&c.Views, &c.Likes)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	return c, err
}
