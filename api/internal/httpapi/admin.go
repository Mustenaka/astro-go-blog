package httpapi

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Mustenaka/astro-go-blog/api/internal/auth"
	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

/* ---------- auth ---------- */

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /admin/api/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := s.ipHash(r) + "|" + req.Username
	if locked, remaining := s.logins.Locked(key); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(remaining.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("too many failed logins; try again in %d minutes", int(remaining.Minutes())+1))
		return
	}
	if req.Username != s.cfg.AdminUser || !auth.VerifyPassword(s.cfg.AdminPasswordHash, req.Password) {
		s.logins.Fail(key)
		s.log.Warn("login failed", "user", req.Username)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	s.logins.Reset(key)
	id, err := s.sessions.Create(r.Context(), req.Username, s.ipHash(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(auth.SessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": req.Username})
}

// POST /admin/api/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.sessions.Delete(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /admin/api/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(ctxUser{}).(string)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "storage": s.cfg.StorageKind, "assetBase": s.cfg.AssetPublicBase, "prefixes": storage.Prefixes})
}

/* ---------- assets ---------- */

type presignRequest struct {
	Prefix       string `json:"prefix"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	DemoName     string `json:"demoName,omitempty"`
	DemoVersion  string `json:"demoVersion,omitempty"`
	RelativePath string `json:"relativePath,omitempty"`
}

// POST /admin/api/assets/presign
func (s *Server) handlePresign(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name := req.Filename
	if req.Prefix == storage.PrefixDemos && req.RelativePath != "" {
		name = req.RelativePath
	}
	if err := storage.ValidateFile(req.Prefix, name, req.ContentType, req.Size); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key, err := storage.BuildKey(storage.KeyOptions{
		Prefix: req.Prefix, Filename: req.Filename,
		DemoName: req.DemoName, DemoVersion: req.DemoVersion, RelativePath: req.RelativePath,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	upload, err := s.store.Presign(r.Context(), key, req.ContentType, req.Size)
	if err != nil {
		s.log.Error("presign", "err", err)
		writeError(w, http.StatusInternalServerError, "could not presign upload")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key":       key,
		"assetPath": "/assets/" + key,
		"publicUrl": s.store.PublicURL(key),
		"upload":    upload,
	})
}

type completeRequest struct {
	Key          string `json:"key"`
	OriginalName string `json:"originalName"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	SHA256       string `json:"sha256"`
}

var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Asset is an assets row.
type Asset struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	AssetPath    string `json:"assetPath"`
	PublicURL    string `json:"publicUrl"`
	Prefix       string `json:"prefix"`
	OriginalName string `json:"originalName"`
	Size         int64  `json:"size"`
	ContentType  string `json:"contentType"`
	SHA256       string `json:"sha256"`
	CreatedAt    string `json:"createdAt"`
}

// POST /admin/api/assets/complete — called after the client finished the PUT.
func (s *Server) handleComplete(w http.ResponseWriter, r *http.Request) {
	var req completeRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !storage.ValidKey(req.Key) {
		writeError(w, http.StatusBadRequest, "invalid key")
		return
	}
	if !sha256Re.MatchString(strings.ToLower(req.SHA256)) {
		writeError(w, http.StatusBadRequest, "sha256 must be 64 hex characters")
		return
	}
	info, err := s.store.Stat(r.Context(), req.Key)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusBadRequest, "object has not been uploaded")
		return
	}
	if err != nil {
		s.log.Error("stat", "err", err)
		writeError(w, http.StatusBadGateway, "could not verify upload")
		return
	}
	if info.Size != req.Size {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("stored size %d does not match reported size %d", info.Size, req.Size))
		return
	}
	now := db.Now()
	res, err := s.db.ExecContext(r.Context(), `INSERT INTO assets (key, prefix, original_name, size, mime, sha256, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET size = excluded.size, mime = excluded.mime, sha256 = excluded.sha256`,
		req.Key, storage.PrefixOf(req.Key), req.OriginalName, req.Size, req.ContentType, strings.ToLower(req.SHA256), now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	id, _ := res.LastInsertId()
	asset, err := s.assetByKey(r, req.Key)
	if err != nil {
		asset = &Asset{ID: id, Key: req.Key}
	}
	writeJSON(w, http.StatusCreated, asset)
}

// GET /admin/api/assets?prefix=&q=&page=
func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prefix := q.Get("prefix")
	search := strings.TrimSpace(q.Get("q"))
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	const perPage = 20
	where := "WHERE 1=1"
	args := []any{}
	if prefix != "" {
		where += " AND prefix = ?"
		args = append(args, prefix)
	}
	if search != "" {
		where += " AND (key LIKE ? OR original_name LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM assets `+where, args...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT id, key, prefix, original_name, size, mime, sha256, created_at FROM assets `+where+
		` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	items := []Asset{}
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.Key, &a.Prefix, &a.OriginalName, &a.Size, &a.ContentType, &a.SHA256, &a.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		a.AssetPath = "/assets/" + a.Key
		a.PublicURL = s.store.PublicURL(a.Key)
		items = append(items, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "perPage": perPage, "total": total})
}

// DELETE /admin/api/assets/{id}
func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var key string
	err = s.db.QueryRowContext(r.Context(), `SELECT key FROM assets WHERE id = ?`, id).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if err := s.store.Delete(r.Context(), key); err != nil {
		s.log.Error("delete object", "key", key, "err", err)
		writeError(w, http.StatusBadGateway, "could not delete object")
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM assets WHERE id = ?`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": key})
}

func (s *Server) assetByKey(r *http.Request, key string) (*Asset, error) {
	var a Asset
	err := s.db.QueryRowContext(r.Context(), `SELECT id, key, prefix, original_name, size, mime, sha256, created_at FROM assets WHERE key = ?`, key).
		Scan(&a.ID, &a.Key, &a.Prefix, &a.OriginalName, &a.Size, &a.ContentType, &a.SHA256, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.AssetPath = "/assets/" + a.Key
	a.PublicURL = s.store.PublicURL(a.Key)
	return &a, nil
}

/* ---------- builds ---------- */

// POST /admin/api/build
func (s *Server) handleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	job, err := s.builds.Trigger(r.Context(), "admin")
	if errors.Is(err, build.ErrBusy) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

// GET /admin/api/build/{id}
func (s *Server) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	job, err := s.builds.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job, "log": s.builds.LogTail(job, 16<<10)})
}

// GET /admin/api/build
func (s *Server) handleRecentBuilds(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.builds.Recent(r.Context(), 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobs == nil {
		jobs = []*build.Job{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

/* ---------- stats ---------- */

// GET /admin/api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	top := func(order string) ([]Counter, error) {
		rows, err := s.db.QueryContext(r.Context(), `SELECT slug, views, likes FROM counters ORDER BY `+order+` DESC, slug LIMIT 20`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Counter{}
		for rows.Next() {
			var c Counter
			if err := rows.Scan(&c.Slug, &c.Views, &c.Likes); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	views, err := top("views")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	likes, err := top("likes")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	var totalViews, totalLikes int64
	_ = s.db.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(views),0), COALESCE(SUM(likes),0) FROM counters`).Scan(&totalViews, &totalLikes)
	writeJSON(w, http.StatusOK, map[string]any{
		"topViews": views, "topLikes": likes,
		"totalViews": totalViews, "totalLikes": totalLikes,
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}
