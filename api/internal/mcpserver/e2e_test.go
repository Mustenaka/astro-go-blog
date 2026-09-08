package mcpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gen2brain/webp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestEndToEndOverStdio compiles cmd/mcp, starts it over stdio and drives the full
// "write a post, upload an image, validate, build" flow against the real repository content.
// It needs pnpm (validate_content and the release script use it) and takes about a minute.
func TestEndToEndOverStdio(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	if _, err := exec.LookPath("pnpm"); err != nil {
		t.Skip("pnpm not on PATH")
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "content", "schema", "posts.schema.json")); err != nil {
		t.Skip("content/schema missing; run pnpm -C site schema")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "mcp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	buildCmd := exec.Command("go", "build", "-o", bin, "./cmd/mcp")
	buildCmd.Dir = filepath.Join(repo, "api")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	script := filepath.Join(repo, "deploy", "scripts", "release.dev.ps1")
	if runtime.GOOS != "windows" {
		script = filepath.Join(repo, "deploy", "scripts", "release.dev.sh")
	}
	cmd := exec.Command(bin)
	cmd.Dir = tmp // no api/.env here: configuration comes from the environment only
	cmd.Env = append(os.Environ(),
		"DATA_DIR="+filepath.Join(tmp, "data"),
		"STORAGE_KIND=local",
		"ADMIN_PASSWORD_HASH=unused-by-mcp",
		"SESSION_SECRET="+strings.Repeat("e2e-secret-", 4),
		"CONTENT_DIR="+filepath.Join(repo, "content"),
		"SITE_DIR="+filepath.Join(repo, "site"),
		"RELEASE_SCRIPT="+script,
	)
	cmd.Stderr = os.Stderr

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "e2e", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"list_posts", "get_post", "create_post", "update_post", "create_work", "update_work", "upload_asset", "list_assets", "validate_content", "trigger_build", "build_status", "get_stats"}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, n := range want {
		if !names[n] {
			t.Errorf("tool %s missing", n)
		}
	}

	call := func(name string, args map[string]any) (map[string]any, string, bool) {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error %v", name, err)
		}
		var text string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text += tc.Text
			}
		}
		var structured map[string]any
		if res.StructuredContent != nil {
			raw, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(raw, &structured)
		}
		return structured, text, res.IsError
	}

	slug := fmt.Sprintf("mcp-e2e-%d", time.Now().Unix())
	body := "MCP 端到端测试文章。\n\n行内公式 $E = mc^2$，块公式：\n\n$$\n\\nabla \\cdot \\mathbf{v} = 0\n$$\n\n```go\nfunc main() { println(\"hi\") }\n```\n"

	// 1. create_post with an unknown front matter field must be rejected by name.
	_, text, isErr := call("create_post", map[string]any{
		"title": "MCP 端到端", "description": "测试", "tags": []string{"mcp"}, "categories": []string{"站点建设"},
		"body": body, "slug": slug, "bogusField": 1,
	})
	if !isErr || !strings.Contains(text, "bogusField") {
		t.Fatalf("unknown field must be rejected and named, got isError=%v text=%q", isErr, text)
	}

	// 2. create_post
	out, text, isErr := call("create_post", map[string]any{
		"title": "MCP 端到端", "description": "MCP 端到端测试文章。", "date": "2026-09-08", "tags": []string{"mcp"},
		"categories": []string{"站点建设"}, "math": true, "draft": true, "body": body, "slug": slug,
	})
	if isErr {
		t.Fatalf("create_post: %s", text)
	}
	postPath := filepath.Join(repo, filepath.FromSlash(out["path"].(string)))
	t.Cleanup(func() { os.Remove(postPath) })
	if _, err := os.Stat(postPath); err != nil {
		t.Fatalf("post file missing: %v", err)
	}
	// Creating the same slug again is refused.
	if _, text, isErr := call("create_post", map[string]any{"title": "x", "description": "x", "tags": []string{}, "categories": []string{}, "body": "x", "slug": slug}); !isErr || !strings.Contains(text, "already exists") {
		t.Fatalf("duplicate slug: isError=%v %q", isErr, text)
	}

	// 3. upload_asset: a 2400x1200 PNG becomes a 2000x1000 WebP under img/.
	pngPath := filepath.Join(tmp, "E2E Shot.png")
	img := image.NewRGBA(image.Rect(0, 0, 2400, 1200))
	for y := 0; y < 1200; y += 8 {
		for x := 0; x < 2400; x += 8 {
			c := color.RGBA{uint8(x / 10), uint8(y / 5), 200, 255}
			for dy := 0; dy < 8; dy++ {
				for dx := 0; dx < 8; dx++ {
					img.Set(x+dx, y+dy, c)
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, text, isErr = call("upload_asset", map[string]any{"path": pngPath, "prefix": "img"})
	if isErr {
		t.Fatalf("upload_asset: %s", text)
	}
	assetPath := out["assetPath"].(string)
	if !strings.HasPrefix(assetPath, "/assets/img/") || !strings.HasSuffix(assetPath, ".webp") || out["converted"] != true || out["width"].(float64) != 2000 {
		t.Fatalf("unexpected upload result %v", out)
	}
	stored := filepath.Join(tmp, "data", "assets", filepath.FromSlash(strings.TrimPrefix(assetPath, "/assets/")))
	raw, err := os.ReadFile(stored)
	if err != nil {
		t.Fatalf("stored object missing: %v", err)
	}
	if decoded, err := webp.Decode(bytes.NewReader(raw)); err != nil || decoded.Bounds().Dx() != 2000 {
		t.Fatalf("stored file is not a 2000px webp: %v", err)
	}
	if _, text, isErr := call("upload_asset", map[string]any{"path": filepath.Join(repo, "README.md"), "prefix": "img"}); !isErr || !strings.Contains(text, "not allowed") {
		t.Fatalf("README.md under img/ must be rejected: isError=%v %q", isErr, text)
	}

	// 4. update_post: put the image into the post.
	newBody := body + "\n![测试图](" + assetPath + ")\n"
	_, text, isErr = call("update_post", map[string]any{"slug": slug, "frontmatter": map[string]any{"cover": assetPath}, "body": newBody})
	if isErr {
		t.Fatalf("update_post: %s", text)
	}
	written, _ := os.ReadFile(postPath)
	if !strings.Contains(string(written), "cover: "+assetPath) || !strings.Contains(string(written), "![测试图]("+assetPath+")") {
		t.Fatalf("update not written:\n%s", written)
	}
	if _, text, isErr := call("update_post", map[string]any{"slug": slug, "frontmatter": map[string]any{"nope": true}}); !isErr || !strings.Contains(text, "nope") {
		t.Fatalf("unknown front matter field on update must be rejected: %v %q", isErr, text)
	}

	// 5. get_post / list_posts see the new post.
	out, _, _ = call("get_post", map[string]any{"slug": slug})
	if !strings.Contains(out["body"].(string), assetPath) {
		t.Fatalf("get_post body: %v", out["body"])
	}
	out, _, _ = call("list_posts", map[string]any{"draft": "only", "tag": "mcp"})
	if out["count"].(float64) < 1 {
		t.Fatalf("list_posts did not find the draft: %v", out)
	}
	out, _, _ = call("list_assets", map[string]any{"prefix": "img"})
	if out["count"].(float64) != 1 {
		t.Fatalf("list_assets: %v", out)
	}

	// 6. validate_content
	out, text, isErr = call("validate_content", map[string]any{})
	if isErr || out["ok"] != true {
		t.Fatalf("validate_content failed: %s\n%v", text, out["steps"])
	}

	// 7. trigger_build + build_status until done
	out, text, isErr = call("trigger_build", map[string]any{})
	if isErr {
		t.Fatalf("trigger_build: %s", text)
	}
	jobID := int64(out["job"].(map[string]any)["id"].(float64))
	var status string
	var logText string
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		st, _, _ := call("build_status", map[string]any{"id": jobID})
		status = st["job"].(map[string]any)["status"].(string)
		logText, _ = st["log"].(string)
		if status == "succeeded" || status == "failed" {
			break
		}
	}
	if status != "succeeded" {
		t.Fatalf("build status %q\n%s", status, logText)
	}
	if !strings.Contains(logText, "page(s) built") {
		t.Fatalf("build log lacks astro output:\n%s", logText)
	}

	// 8. get_stats works on an empty database.
	out, text, isErr = call("get_stats", map[string]any{})
	if isErr || out["totalViews"].(float64) != 0 {
		t.Fatalf("get_stats: %s %v", text, out)
	}
}
