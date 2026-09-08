package mcpserver_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/config"
	"github.com/Mustenaka/astro-go-blog/api/internal/content"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/mcpserver"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

// newInMemorySession wires the server to a client over an in-memory transport with a temp
// content directory (schemas copied from the repository) and temp storage/database.
func newInMemorySession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	repo, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	schemaSrc := filepath.Join(repo, "content", "schema")
	if _, err := os.Stat(filepath.Join(schemaSrc, "posts.schema.json")); err != nil {
		t.Skip("content/schema missing; run pnpm -C site schema")
	}
	tmp := t.TempDir()
	root := filepath.Join(tmp, "content")
	for _, d := range []string{"posts", "works", "schema"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(schemaSrc)
	for _, e := range entries {
		raw, _ := os.ReadFile(filepath.Join(schemaSrc, e.Name()))
		_ = os.WriteFile(filepath.Join(root, "schema", e.Name()), raw, 0o644)
	}
	store, err := content.NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	local, err := storage.NewLocal(filepath.Join(tmp, "assets"), "http://localhost:8080/assets", "http://localhost:8080/_local/upload")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.FromEnv(func(k string) string {
		return map[string]string{"DATA_DIR": tmp, "ADMIN_PASSWORD_HASH": "x", "SESSION_SECRET": strings.Repeat("s", 40)}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runner := build.New(database, "", filepath.Join(tmp, "builds"))
	runner.Start(ctx)

	srv := mcpserver.New(mcpserver.Deps{Config: cfg, DB: database, Storage: local, Content: store, Builds: runner, SiteDir: filepath.Join(repo, "site"), Logger: slog.New(slog.DiscardHandler)})
	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) (map[string]any, string, bool) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	var out map[string]any
	if res.StructuredContent != nil {
		raw, _ := json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(raw, &out)
	}
	return out, text, res.IsError
}

func TestEveryToolRejectsUnknownArguments(t *testing.T) {
	s := newInMemorySession(t)
	tools, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 12 {
		t.Fatalf("want 12 tools, got %d", len(tools.Tools))
	}
	for _, tool := range tools.Tools {
		_, text, isErr := callTool(t, s, tool.Name, map[string]any{"definitelyUnknownArg": 1})
		if !isErr || !strings.Contains(text, "definitelyUnknownArg") {
			t.Errorf("%s: unknown argument must be rejected by name, got isError=%v text=%q", tool.Name, isErr, text)
		}
	}
}

func TestToolValidationAndSuccessPaths(t *testing.T) {
	s := newInMemorySession(t)

	// create_post: missing required, then success.
	if _, text, isErr := callTool(t, s, "create_post", map[string]any{"title": "x", "tags": []string{}, "categories": []string{}, "body": "b"}); !isErr || !strings.Contains(text, "description") {
		t.Fatalf("missing description: %v %q", isErr, text)
	}
	if _, text, isErr := callTool(t, s, "create_post", map[string]any{"title": "x", "description": "d", "date": "09/08/2026", "tags": []string{}, "categories": []string{}, "body": "b"}); !isErr || !strings.Contains(text, "/date") {
		t.Fatalf("bad date: %v %q", isErr, text)
	}
	out, text, isErr := callTool(t, s, "create_post", map[string]any{"title": "SPH 公式总结", "description": "d", "date": "2026-09-08", "tags": []string{"sph"}, "categories": []string{"物理仿真"}, "body": "正文"})
	if isErr || out["slug"] != "sph-gong-shi-zong-jie" || out["path"] != "content/posts/2026/sph-gong-shi-zong-jie.mdx" {
		t.Fatalf("create_post: %v %q", out, text)
	}

	// get_post / list_posts
	if out, _, isErr := callTool(t, s, "get_post", map[string]any{"slug": "sph-gong-shi-zong-jie"}); isErr || out["body"] != "正文\n" && out["body"] != "正文" {
		t.Fatalf("get_post: %v", out)
	}
	if _, text, isErr := callTool(t, s, "get_post", map[string]any{"slug": "missing"}); !isErr || !strings.Contains(text, "not found") {
		t.Fatalf("get_post missing: %v %q", isErr, text)
	}
	if out, _, _ := callTool(t, s, "list_posts", map[string]any{"category": "物理仿真"}); out["count"].(float64) != 1 {
		t.Fatalf("list_posts: %v", out)
	}
	if _, text, isErr := callTool(t, s, "list_posts", map[string]any{"draft": "maybe"}); !isErr || !strings.Contains(text, "draft") {
		t.Fatalf("list_posts bad draft filter: %v %q", isErr, text)
	}

	// update_post: nothing to do, bad field, success with unset.
	if _, text, isErr := callTool(t, s, "update_post", map[string]any{"slug": "sph-gong-shi-zong-jie"}); !isErr || !strings.Contains(text, "nothing to update") {
		t.Fatalf("update_post empty: %v %q", isErr, text)
	}
	if _, text, isErr := callTool(t, s, "update_post", map[string]any{"slug": "sph-gong-shi-zong-jie", "frontmatter": map[string]any{"tags": "sph"}}); !isErr || !strings.Contains(text, "/tags") {
		t.Fatalf("update_post bad type: %v %q", isErr, text)
	}
	if _, text, isErr := callTool(t, s, "update_post", map[string]any{"slug": "sph-gong-shi-zong-jie", "frontmatter": map[string]any{"series": "SPH"}, "unset": []string{"draft"}}); isErr {
		t.Fatalf("update_post: %q", text)
	}

	// create_work / update_work
	work := map[string]any{"title": "Demo Work", "summary": "s", "period": map[string]any{"from": "2025-01"}, "stack": []string{"Go"}, "cover": "/assets/img/2026/09/c.webp", "status": "wip", "body": "## 简介"}
	out, text, isErr = callTool(t, s, "create_work", work)
	if isErr || out["slug"] != "demo-work" {
		t.Fatalf("create_work: %v %q", out, text)
	}
	work["status"] = "done"
	if _, text, isErr := callTool(t, s, "create_work", work); !isErr || !strings.Contains(text, "status") {
		t.Fatalf("create_work bad status: %v %q", isErr, text)
	}
	if _, text, isErr := callTool(t, s, "update_work", map[string]any{"slug": "demo-work", "frontmatter": map[string]any{"demo": map[string]any{"kind": "island", "name": "particles"}, "featured": true}}); isErr {
		t.Fatalf("update_work: %q", text)
	}
	if _, text, isErr := callTool(t, s, "update_work", map[string]any{"slug": "demo-work", "frontmatter": map[string]any{"demo": map[string]any{"kind": "video"}}}); !isErr || !strings.Contains(text, "/demo") {
		t.Fatalf("update_work bad demo: %v %q", isErr, text)
	}

	// upload_asset validation: missing file, bad prefix.
	if _, text, isErr := callTool(t, s, "upload_asset", map[string]any{"path": "does-not-exist.png"}); !isErr || !strings.Contains(text, "does-not-exist.png") {
		t.Fatalf("upload_asset missing file: %v %q", isErr, text)
	}
	if _, text, isErr := callTool(t, s, "upload_asset", map[string]any{"path": "x.png", "prefix": "uploads"}); !isErr {
		t.Fatalf("upload_asset bad prefix must fail: %q", text)
	}
	if out, _, isErr := callTool(t, s, "list_assets", map[string]any{}); isErr || out["count"].(float64) != 0 {
		t.Fatalf("list_assets: %v", out)
	}

	// Builds without a release script are refused clearly; status of nothing is empty.
	if _, text, isErr := callTool(t, s, "trigger_build", map[string]any{}); !isErr || !strings.Contains(text, "RELEASE_SCRIPT") {
		t.Fatalf("trigger_build without script: %v %q", isErr, text)
	}
	if out, _, isErr := callTool(t, s, "build_status", map[string]any{}); isErr || out["job"] != nil {
		t.Fatalf("build_status empty: %v", out)
	}
	if _, text, isErr := callTool(t, s, "build_status", map[string]any{"id": 42}); !isErr || !strings.Contains(text, "not found") {
		t.Fatalf("build_status unknown id: %v %q", isErr, text)
	}
	if out, _, isErr := callTool(t, s, "get_stats", map[string]any{}); isErr || out["totalViews"].(float64) != 0 {
		t.Fatalf("get_stats: %v", out)
	}
}
