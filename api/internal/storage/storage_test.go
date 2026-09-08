package storage

import (
	"strings"
	"testing"
	"time"
)

func TestValidateFile(t *testing.T) {
	ok := []struct{ prefix, name, ct string; size int64 }{
		{"img", "a.webp", "image/webp", 1000},
		{"img", "Shot.PNG", "image/png", 1000},
		{"video", "clip.mp4", "video/mp4", 50 << 20},
		{"demos", "Build/game.wasm.br", "application/octet-stream", 100 << 20},
		{"demos", "index.html", "text/html; charset=utf-8", 2000},
		{"models", "robot.glb", "model/gltf-binary", 1 << 20},
		{"files", "paper.pdf", "application/pdf", 1 << 20},
	}
	for _, c := range ok {
		if err := ValidateFile(c.prefix, c.name, c.ct, c.size); err != nil {
			t.Errorf("%v: unexpected error %v", c, err)
		}
	}
	bad := []struct{ prefix, name, ct string; size int64 }{
		{"img", "a.exe", "application/octet-stream", 10},
		{"img", "a.webp", "text/html", 10},
		{"img", "a.webp", "image/webp", 21 << 20},
		{"img", "a.webp", "image/webp", 0},
		{"uploads", "a.webp", "image/webp", 10},
		{"files", "script.sh", "text/plain", 10},
		{"img", "noext", "image/webp", 10},
	}
	for _, c := range bad {
		if err := ValidateFile(c.prefix, c.name, c.ct, c.size); err == nil {
			t.Errorf("%v: expected an error", c)
		}
	}
}

func TestBuildKey(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	key, err := BuildKey(KeyOptions{Prefix: "img", Filename: "My Photo (1).WEBP", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "img/2026/09/my-photo-1-") || !strings.HasSuffix(key, ".webp") || !ValidKey(key) {
		t.Fatalf("bad key %q", key)
	}
	key, err = BuildKey(KeyOptions{Prefix: "img", Filename: "截图.png", Now: now})
	if err != nil || !strings.HasPrefix(key, "img/2026/09/file-") {
		t.Fatalf("non-latin name should fall back to 'file': %q %v", key, err)
	}
	key, err = BuildKey(KeyOptions{Prefix: "demos", DemoName: "cube", DemoVersion: "1.0.0", RelativePath: "Build\\cube.wasm.br"})
	if err != nil || key != "demos/cube/1.0.0/Build/cube.wasm.br" {
		t.Fatalf("demo key: %q %v", key, err)
	}
	if _, err := BuildKey(KeyOptions{Prefix: "demos", DemoName: "cube", DemoVersion: "1.0.0", RelativePath: "../../etc/passwd"}); err == nil {
		t.Fatal("traversal in relativePath must be rejected")
	}
	if _, err := BuildKey(KeyOptions{Prefix: "demos", DemoName: "Cube Game", DemoVersion: "1", RelativePath: "index.html"}); err == nil {
		t.Fatal("demo name with spaces/uppercase must be rejected")
	}
	for _, k := range []string{"img/2026/13/a.webp", "img/2026/09/../x.webp", "etc/passwd", "img/a.webp", "demos/x/1/../../a"} {
		if ValidKey(k) {
			t.Errorf("%q should be invalid", k)
		}
	}
}

func TestContentTypeFor(t *testing.T) {
	cases := map[string]string{
		"demos/c/1/Build/c.wasm.br":         "application/wasm",
		"demos/c/1/Build/c.framework.js.br": "text/javascript",
		"demos/c/1/Build/c.data.br":         "application/octet-stream",
		"demos/c/1/index.html":              "text/html; charset=utf-8",
		"img/2026/09/a.webp":                "image/webp",
		"video/2026/09/a.mp4":               "video/mp4",
		"models/2026/09/a.glb":              "model/gltf-binary",
	}
	for key, want := range cases {
		if got := ContentTypeFor(key); got != want {
			t.Errorf("%s: want %q, got %q", key, want, got)
		}
	}
}

func TestCOSPresignOffline(t *testing.T) {
	c, err := NewCOS(COSConfig{Bucket: "blog-1250000000", Region: "ap-guangzhou", SecretID: "AKIDexample", SecretKey: "secretexample", PublicBase: "https://cdn.example.com/assets"})
	if err != nil {
		t.Fatal(err)
	}
	up, err := c.Presign(t.Context(), "img/2026/09/a-abc123.webp", "image/webp", 1234)
	if err != nil {
		t.Fatal(err)
	}
	if up.Method != "PUT" || !strings.Contains(up.URL, "blog-1250000000.cos.ap-guangzhou.myqcloud.com/img/2026/09/a-abc123.webp") {
		t.Fatalf("unexpected presigned url %q", up.URL)
	}
	if !strings.Contains(up.URL, "q-signature=") || !strings.Contains(up.URL, "q-ak=AKIDexample") {
		t.Fatalf("presigned url lacks signature params: %q", up.URL)
	}
	if up.Headers["Content-Type"] != "image/webp" {
		t.Fatalf("headers: %v", up.Headers)
	}
	if got := c.PublicURL("img/2026/09/a-abc123.webp"); got != "https://cdn.example.com/assets/img/2026/09/a-abc123.webp" {
		t.Fatalf("public url %q", got)
	}
}
