package httpapi_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Mustenaka/astro-go-blog/api/internal/auth"
	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/config"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/httpapi"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

const (
	testUser     = "admin"
	testPassword = "correct horse battery"
)

type env struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client // with cookie jar
	runner *build.Runner
	cfg    *config.Config
	dir    string
}

// newEnv builds a full server on a temp dir with local storage and a fake release script.
func newEnv(t *testing.T, script string) *env {
	t.Helper()
	dir := t.TempDir()
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"DATA_DIR":              dir,
		"STORAGE_KIND":          "local",
		"ADMIN_USER":            testUser,
		"ADMIN_PASSWORD_HASH":   hash,
		"SESSION_SECRET":        strings.Repeat("s", 40),
		"GITHUB_WEBHOOK_SECRET": "hook-secret",
		"CORS_ORIGINS":          "http://localhost:4321",
		"RELEASE_SCRIPT":        script,
	}
	cfg, err := config.FromEnv(func(k string) string { return values[k] })
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	local, err := storage.NewLocal(filepath.Join(dir, "assets"), "http://placeholder/assets", "http://placeholder/_local/upload")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runner := build.New(database, script, filepath.Join(dir, "builds"))
	runner.Start(ctx)

	s := httpapi.New(cfg, database, local, runner, httpapi.Options{Logger: slog.New(slog.DiscardHandler)})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	local.SetBases(srv.URL+"/assets", srv.URL+"/_local/upload")

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	return &env{t: t, srv: srv, client: client, runner: runner, cfg: cfg, dir: dir}
}

func (e *env) do(method, path string, body any, headers map[string]string) (*http.Response, map[string]any) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			reader = bytes.NewReader(b)
		case string:
			reader = strings.NewReader(b)
		default:
			raw, _ := json.Marshal(b)
			reader = bytes.NewReader(raw)
		}
	}
	req, err := http.NewRequest(method, e.srv.URL+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	if parsed == nil {
		parsed = map[string]any{"_raw": string(raw)}
	}
	return resp, parsed
}

func (e *env) login() {
	e.t.Helper()
	resp, body := e.do("POST", "/admin/api/login", map[string]string{"username": testUser, "password": testPassword}, nil)
	if resp.StatusCode != 200 {
		e.t.Fatalf("login: %d %v", resp.StatusCode, body)
	}
	// httptest serves plain http; the Secure cookie must still be stored for the test client.
	u := resp.Request.URL
	for _, c := range resp.Cookies() {
		c.Secure = false
		e.client.Jar.SetCookies(u, []*http.Cookie{c})
	}
}

// writeScript creates a release script that prints marker and sleeps `sleep` (ms).
func writeScript(t *testing.T, marker string, sleepMs int) string {
	t.Helper()
	dir := t.TempDir()
	var path, body string
	if runtime.GOOS == "windows" {
		path = filepath.Join(dir, "release.ps1")
		body = fmt.Sprintf("Write-Output '%s'\nStart-Sleep -Milliseconds %d\nexit 0\n", marker, sleepMs)
	} else {
		path = filepath.Join(dir, "release.sh")
		body = fmt.Sprintf("#!/bin/sh\necho '%s'\nsleep %.3f\nexit 0\n", marker, float64(sleepMs)/1000)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoginSuccessAndFailureLockout(t *testing.T) {
	e := newEnv(t, "")
	resp, _ := e.do("GET", "/admin/api/me", nil, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("unauthenticated /me: want 401, got %d", resp.StatusCode)
	}
	for i := 1; i <= 5; i++ {
		resp, _ := e.do("POST", "/admin/api/login", map[string]string{"username": testUser, "password": "wrong"}, nil)
		if resp.StatusCode != 401 {
			t.Fatalf("attempt %d: want 401, got %d", i, resp.StatusCode)
		}
	}
	resp, body := e.do("POST", "/admin/api/login", map[string]string{"username": testUser, "password": testPassword}, nil)
	if resp.StatusCode != 429 {
		t.Fatalf("after 5 failures the correct password must be locked out: got %d %v", resp.StatusCode, body)
	}

	// A fresh server: success path.
	e2 := newEnv(t, "")
	e2.login()
	resp, body = e2.do("GET", "/admin/api/me", nil, nil)
	if resp.StatusCode != 200 || body["user"] != testUser {
		t.Fatalf("/me after login: %d %v", resp.StatusCode, body)
	}
	resp, _ = e2.do("POST", "/admin/api/logout", nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	resp, _ = e2.do("GET", "/admin/api/me", nil, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("/me after logout: want 401, got %d", resp.StatusCode)
	}
}

func TestAdminRoutesRequireSession(t *testing.T) {
	e := newEnv(t, "")
	for _, route := range []struct{ method, path string }{
		{"POST", "/admin/api/assets/presign"}, {"POST", "/admin/api/assets/complete"}, {"GET", "/admin/api/assets"},
		{"DELETE", "/admin/api/assets/1"}, {"POST", "/admin/api/build"}, {"GET", "/admin/api/build"},
		{"GET", "/admin/api/build/1"}, {"GET", "/admin/api/stats"}, {"POST", "/admin/api/logout"},
	} {
		resp, _ := e.do(route.method, route.path, map[string]any{}, nil)
		if resp.StatusCode != 401 {
			t.Errorf("%s %s without session: want 401, got %d", route.method, route.path, resp.StatusCode)
		}
	}
}

func TestPresignRejectsInvalidFiles(t *testing.T) {
	e := newEnv(t, "")
	e.login()
	cases := []struct {
		name string
		req  map[string]any
	}{
		{"bad extension", map[string]any{"prefix": "img", "filename": "evil.exe", "size": 100, "contentType": "application/octet-stream"}},
		{"oversize", map[string]any{"prefix": "img", "filename": "big.webp", "size": 21 << 20, "contentType": "image/webp"}},
		{"mime mismatch", map[string]any{"prefix": "img", "filename": "a.webp", "size": 100, "contentType": "text/html"}},
		{"unknown prefix", map[string]any{"prefix": "uploads", "filename": "a.webp", "size": 100, "contentType": "image/webp"}},
		{"demo without name", map[string]any{"prefix": "demos", "filename": "index.html", "size": 100, "contentType": "text/html", "relativePath": "index.html"}},
	}
	for _, c := range cases {
		resp, body := e.do("POST", "/admin/api/assets/presign", c.req, nil)
		if resp.StatusCode != 400 {
			t.Errorf("%s: want 400, got %d %v", c.name, resp.StatusCode, body)
		}
	}
}

func TestLocalUploadChain(t *testing.T) {
	e := newEnv(t, "")
	e.login()
	content := []byte("RIFF....WEBPVP8 fake image bytes")
	sum := sha256.Sum256(content)

	resp, pre := e.do("POST", "/admin/api/assets/presign", map[string]any{
		"prefix": "img", "filename": "My Photo (1).webp", "size": len(content), "contentType": "image/webp",
	}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("presign: %d %v", resp.StatusCode, pre)
	}
	key := pre["key"].(string)
	if !strings.HasPrefix(key, "img/") || !strings.HasSuffix(key, ".webp") || !strings.Contains(key, "my-photo-1-") {
		t.Fatalf("unexpected key %q", key)
	}
	upload := pre["upload"].(map[string]any)

	// Complete before upload must fail.
	resp, body := e.do("POST", "/admin/api/assets/complete", map[string]any{
		"key": key, "originalName": "My Photo (1).webp", "size": len(content), "contentType": "image/webp", "sha256": hex.EncodeToString(sum[:]),
	}, nil)
	if resp.StatusCode != 400 {
		t.Fatalf("complete before upload: want 400, got %d %v", resp.StatusCode, body)
	}

	req, _ := http.NewRequest(upload["method"].(string), upload["url"].(string), bytes.NewReader(content))
	req.Header.Set("Content-Type", "image/webp")
	put, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	put.Body.Close()
	if put.StatusCode != 200 {
		t.Fatalf("PUT upload: %d", put.StatusCode)
	}
	// The token is single-use.
	req2, _ := http.NewRequest("PUT", upload["url"].(string), bytes.NewReader(content))
	put2, _ := http.DefaultClient.Do(req2)
	put2.Body.Close()
	if put2.StatusCode != 403 {
		t.Fatalf("second PUT with the same token: want 403, got %d", put2.StatusCode)
	}

	resp, asset := e.do("POST", "/admin/api/assets/complete", map[string]any{
		"key": key, "originalName": "My Photo (1).webp", "size": len(content), "contentType": "image/webp", "sha256": hex.EncodeToString(sum[:]),
	}, nil)
	if resp.StatusCode != 201 || asset["assetPath"] != "/assets/"+key {
		t.Fatalf("complete: %d %v", resp.StatusCode, asset)
	}

	get, err := http.Get(e.srv.URL + "/assets/" + key)
	if err != nil {
		t.Fatal(err)
	}
	served, _ := io.ReadAll(get.Body)
	get.Body.Close()
	if get.StatusCode != 200 || !bytes.Equal(served, content) {
		t.Fatalf("GET asset: %d, %d bytes", get.StatusCode, len(served))
	}
	if ct := get.Header.Get("Content-Type"); ct != "image/webp" {
		t.Fatalf("content type: %q", ct)
	}
	if get.Header.Get("Cross-Origin-Resource-Policy") != "cross-origin" {
		t.Fatalf("missing CORP header")
	}
	if get.Header.Get("X-Frame-Options") != "" {
		t.Fatalf("assets must not carry X-Frame-Options")
	}

	resp, list := e.do("GET", "/admin/api/assets?prefix=img&q=photo", nil, nil)
	if resp.StatusCode != 200 || list["total"].(float64) != 1 {
		t.Fatalf("list: %d %v", resp.StatusCode, list)
	}
	id := int64(asset["id"].(float64))
	resp, _ = e.do("DELETE", fmt.Sprintf("/admin/api/assets/%d", id), nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	get, _ = http.Get(e.srv.URL + "/assets/" + key)
	get.Body.Close()
	if get.StatusCode != 404 {
		t.Fatalf("after delete: want 404, got %d", get.StatusCode)
	}
	// Path traversal must never reach the disk.
	get, _ = http.Get(e.srv.URL + "/assets/img/2026/09/../../../test.db")
	get.Body.Close()
	if get.StatusCode == 200 {
		t.Fatalf("path traversal served a file")
	}
}

func TestBrotliAndDemoHeaders(t *testing.T) {
	e := newEnv(t, "")
	root := filepath.Join(e.dir, "assets", "demos", "cube", "1.0.0", "Build")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cube.wasm.br", "cube.framework.js.br", "cube.data.br"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("compressed"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]string{
		"cube.wasm.br":         "application/wasm",
		"cube.framework.js.br": "text/javascript",
		"cube.data.br":         "application/octet-stream",
	}
	for name, ct := range want {
		resp, err := http.Get(e.srv.URL + "/assets/demos/cube/1.0.0/Build/" + name)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d", name, resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); got != ct {
			t.Errorf("%s content type: want %q, got %q", name, ct, got)
		}
		if resp.Header.Get("Content-Encoding") != "br" {
			t.Errorf("%s: missing Content-Encoding: br", name)
		}
		if resp.Header.Get("Cross-Origin-Opener-Policy") != "same-origin" || resp.Header.Get("Cross-Origin-Embedder-Policy") != "require-corp" {
			t.Errorf("%s: missing COOP/COEP", name)
		}
	}
}

func TestViewDedupeAndCounters(t *testing.T) {
	e := newEnv(t, "")
	for i := 0; i < 3; i++ {
		resp, body := e.do("POST", "/api/v1/counters/hello-astro/view", nil, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("view %d: %d %v", i, resp.StatusCode, body)
		}
		if views := body["views"].(float64); views != 1 {
			t.Fatalf("view %d: views=%v, want 1 (deduped within 10 minutes)", i, views)
		}
	}
	// Another client (via X-Forwarded-For from the loopback peer) counts separately.
	resp, body := e.do("POST", "/api/v1/counters/hello-astro/view", nil, map[string]string{"X-Forwarded-For": "203.0.113.9"})
	if resp.StatusCode != 200 || body["views"].(float64) != 2 {
		t.Fatalf("second client: %d %v", resp.StatusCode, body)
	}
	resp, body = e.do("GET", "/api/v1/counters?slugs=hello-astro,missing,BAD%20SLUG", nil, map[string]string{"Origin": "http://localhost:4321"})
	if resp.StatusCode != 200 {
		t.Fatalf("get counters: %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:4321" {
		t.Fatalf("CORS header missing for allowed origin")
	}
	counters := body["counters"].([]any)
	if len(counters) != 2 {
		t.Fatalf("want 2 counters (invalid slug dropped), got %v", counters)
	}
	first := counters[0].(map[string]any)
	if first["slug"] != "hello-astro" || first["views"].(float64) != 2 {
		t.Fatalf("unexpected counter %v", first)
	}
	resp, _ = e.do("GET", "/api/v1/counters?slugs=hello-astro", nil, map[string]string{"Origin": "http://evil.example"})
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("CORS header leaked to a disallowed origin")
	}
	resp, _ = e.do("POST", "/api/v1/counters/Bad_Slug/view", nil, nil)
	if resp.StatusCode != 400 {
		t.Fatalf("invalid slug: want 400, got %d", resp.StatusCode)
	}
}

func TestLikeOncePerDay(t *testing.T) {
	e := newEnv(t, "")
	resp, body := e.do("POST", "/api/v1/likes/hello-astro", nil, nil)
	if resp.StatusCode != 200 || body["likes"].(float64) != 1 || body["liked"] != true {
		t.Fatalf("first like: %d %v", resp.StatusCode, body)
	}
	resp, body = e.do("POST", "/api/v1/likes/hello-astro", nil, nil)
	if resp.StatusCode != 200 || body["likes"].(float64) != 1 || body["liked"] != false {
		t.Fatalf("second like same day: %d %v", resp.StatusCode, body)
	}
	resp, body = e.do("POST", "/api/v1/likes/hello-astro", nil, map[string]string{"X-Forwarded-For": "198.51.100.7"})
	if resp.StatusCode != 200 || body["likes"].(float64) != 2 {
		t.Fatalf("other client like: %d %v", resp.StatusCode, body)
	}
}

func TestPublicRateLimit(t *testing.T) {
	e := newEnv(t, "")
	var last int
	for i := 0; i < 70; i++ {
		resp, _ := e.do("POST", "/api/v1/likes/rate-limit-test", nil, nil)
		last = resp.StatusCode
		if last == 429 {
			break
		}
	}
	if last != 429 {
		t.Fatalf("expected 429 after exhausting the burst, last status %d", last)
	}
}

func TestGitHubWebhookSignature(t *testing.T) {
	e := newEnv(t, writeScript(t, "hook-build", 50))
	payload := []byte(`{"ref":"refs/heads/main","after":"abc"}`)
	sign := func(secret string) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	resp, _ := e.do("POST", "/hooks/github", payload, map[string]string{"X-Hub-Signature-256": sign("wrong"), "X-GitHub-Event": "push"})
	if resp.StatusCode != 401 {
		t.Fatalf("bad signature: want 401, got %d", resp.StatusCode)
	}
	resp, _ = e.do("POST", "/hooks/github", payload, map[string]string{"X-GitHub-Event": "push"})
	if resp.StatusCode != 401 {
		t.Fatalf("missing signature: want 401, got %d", resp.StatusCode)
	}
	resp, body := e.do("POST", "/hooks/github", payload, map[string]string{"X-Hub-Signature-256": sign("hook-secret"), "X-GitHub-Event": "push"})
	if resp.StatusCode != 202 {
		t.Fatalf("good signature: want 202, got %d %v", resp.StatusCode, body)
	}
	other := []byte(`{"ref":"refs/heads/feature"}`)
	mac := hmac.New(sha256.New, []byte("hook-secret"))
	mac.Write(other)
	resp, body = e.do("POST", "/hooks/github", other, map[string]string{"X-Hub-Signature-256": "sha256=" + hex.EncodeToString(mac.Sum(nil)), "X-GitHub-Event": "push"})
	if resp.StatusCode != 200 || body["ignored"] != true {
		t.Fatalf("other branch: want ignored, got %d %v", resp.StatusCode, body)
	}
	if !e.runner.Wait(10 * time.Second) {
		t.Fatal("build did not finish")
	}
}

func TestBuildMutexAndQueue(t *testing.T) {
	e := newEnv(t, writeScript(t, "build-marker", 700))
	e.login()
	resp, first := e.do("POST", "/admin/api/build", nil, nil)
	if resp.StatusCode != 202 {
		t.Fatalf("first trigger: %d %v", resp.StatusCode, first)
	}
	resp, second := e.do("POST", "/admin/api/build", nil, nil)
	if resp.StatusCode != 202 {
		t.Fatalf("second trigger (queued): %d %v", resp.StatusCode, second)
	}
	resp, third := e.do("POST", "/admin/api/build", nil, nil)
	if resp.StatusCode != 409 {
		t.Fatalf("third trigger must be dropped with 409, got %d %v", resp.StatusCode, third)
	}
	if !e.runner.Wait(30 * time.Second) {
		t.Fatal("builds did not finish")
	}
	resp, list := e.do("GET", "/admin/api/build", nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("recent: %d", resp.StatusCode)
	}
	jobs := list["jobs"].([]any)
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(jobs))
	}
	for _, j := range jobs {
		job := j.(map[string]any)
		if job["status"] != "succeeded" {
			t.Errorf("job %v: status %v", job["id"], job["status"])
		}
	}
	id := int64(first["job"].(map[string]any)["id"].(float64))
	resp, detail := e.do("GET", fmt.Sprintf("/admin/api/build/%d", id), nil, nil)
	if resp.StatusCode != 200 || !strings.Contains(detail["log"].(string), "build-marker") {
		t.Fatalf("build log: %d %v", resp.StatusCode, detail["log"])
	}
	// The first build started before the second.
	firstJob := detail["job"].(map[string]any)
	resp, detail2 := e.do("GET", fmt.Sprintf("/admin/api/build/%d", id+1), nil, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("second job: %d", resp.StatusCode)
	}
	secondJob := detail2["job"].(map[string]any)
	if firstJob["startedAt"].(string) > secondJob["startedAt"].(string) {
		t.Fatalf("jobs ran out of order: %v then %v", firstJob["startedAt"], secondJob["startedAt"])
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newEnv(t, "")
	resp, _ := e.do("GET", "/healthz", nil, nil)
	for h, want := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"Referrer-Policy":         "no-referrer",
	} {
		if got := resp.Header.Get(h); got != want {
			t.Errorf("%s: want %q, got %q", h, want, got)
		}
	}
}
