package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoContentDir returns the real content/ directory (schemas are committed there).
func repoContentDir(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "..", "content"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p, "schema", "posts.schema.json")); err != nil {
		t.Skipf("content/schema not found: %v", err)
	}
	return p
}

// tempStore copies the schema dir into a temp content root with empty posts/works.
func tempStore(t *testing.T) *Store {
	t.Helper()
	src := repoContentDir(t)
	root := filepath.Join(t.TempDir(), "content")
	for _, d := range []string{"posts", "works", "schema"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(src, "schema"))
	for _, e := range entries {
		raw, _ := os.ReadFile(filepath.Join(src, "schema", e.Name()))
		if err := os.WriteFile(filepath.Join(root, "schema", e.Name()), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const sample = `---
title: 你好，Astro
description: 第一篇示例文章。
date: 2026-09-01
tags: [astro, 博客]
categories: [站点建设]
cover: /assets/img/2026/09/sample.webp
legacyUrls:
  - /index.php/2026/09/01/hello-astro/
---

正文第一段。

## 标题

- 列表
`

func TestParseSerializeRoundTrip(t *testing.T) {
	doc, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter["date"] != "2026-09-01" {
		t.Fatalf("date normalised wrong: %v (%T)", doc.Frontmatter["date"], doc.Frontmatter["date"])
	}
	if tags := stringSlice(doc.Frontmatter["tags"]); len(tags) != 2 || tags[1] != "博客" {
		t.Fatalf("tags: %v", tags)
	}
	if !strings.HasPrefix(doc.Body, "正文第一段。") {
		t.Fatalf("body: %q", doc.Body)
	}
	out, err := doc.Serialize(PostKeyOrder)
	if err != nil {
		t.Fatal(err)
	}
	if out != sample {
		t.Fatalf("round trip changed the file:\n--- want\n%s\n--- got\n%s", sample, out)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World!":        "hello-world",
		"SPH 公式总结":            "sph-gong-shi-zong-jie",
		"你好，Astro":            "ni-hao-astro",
		"  Unity   WebGL 2024": "unity-webgl-2024",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreateUpdateValidate(t *testing.T) {
	s := tempStore(t)
	fm := map[string]any{
		"title": "公式与代码", "description": "测试", "date": "2026-09-08",
		"tags": []any{"katex"}, "categories": []any{"物理仿真"}, "math": true,
	}
	path, err := s.CreatePost(fm, "正文 $E=mc^2$", "")
	if err != nil {
		t.Fatal(err)
	}
	if path != "content/posts/2026/gong-shi-yu-dai-ma.mdx" {
		t.Fatalf("path %q", path)
	}
	if _, err := s.CreatePost(fm, "again", "gong-shi-yu-dai-ma"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate slug must be rejected, got %v", err)
	}

	// Unknown field is rejected and named.
	bad := cloneMap(fm)
	bad["bogusField"] = 1
	_, err = s.CreatePost(bad, "x", "bad-1")
	if err == nil || !strings.Contains(err.Error(), "bogusField") {
		t.Fatalf("unknown field must be rejected by name, got %v", err)
	}
	// Wrong type is rejected.
	bad = cloneMap(fm)
	bad["tags"] = "katex"
	if _, err := s.CreatePost(bad, "x", "bad-2"); err == nil || !strings.Contains(err.Error(), "/tags") {
		t.Fatalf("wrong type must be rejected with the field path, got %v", err)
	}
	// Missing required field.
	bad = cloneMap(fm)
	delete(bad, "description")
	if _, err := s.CreatePost(bad, "x", "bad-3"); err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("missing required must be rejected, got %v", err)
	}
	// Bad date format.
	bad = cloneMap(fm)
	bad["date"] = "next tuesday"
	if _, err := s.CreatePost(bad, "x", "bad-4"); err == nil || !strings.Contains(err.Error(), "/date") {
		t.Fatalf("bad date must be rejected, got %v", err)
	}

	// Update: patch, unset, body.
	body := "新的正文"
	path, err = s.UpdatePost("gong-shi-yu-dai-ma", map[string]any{"draft": true, "cover": "/assets/img/2026/09/a.webp"}, []string{"math"}, &body)
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := s.GetPost("gong-shi-yu-dai-ma")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter["draft"] != true || doc.Frontmatter["cover"] != "/assets/img/2026/09/a.webp" || doc.Frontmatter["math"] != nil || strings.TrimSpace(doc.Body) != body {
		t.Fatalf("update not applied: %v %q", doc.Frontmatter, doc.Body)
	}
	if _, err := s.UpdatePost("gong-shi-yu-dai-ma", map[string]any{"cover": "img/no-prefix.webp"}, nil, nil); err == nil {
		t.Fatal("cover without /assets/ prefix must be rejected")
	}
	if _, err := s.UpdatePost("missing", map[string]any{"draft": true}, nil, nil); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	list, err := s.ListPosts()
	if err != nil || len(list) != 1 || !list[0].Draft {
		t.Fatalf("list: %v %v", list, err)
	}
	_ = path

	// Works: nested validation.
	work := map[string]any{
		"title": "AdaptorPhysX", "summary": "适配层", "period": map[string]any{"from": "2024-06"},
		"stack": []any{"Unity"}, "cover": "/assets/img/2026/09/c.webp", "status": "active",
		"demo": map[string]any{"kind": "island", "name": "particles"},
	}
	if _, err := s.CreateWork(work, "## 简介", "adaptor-physx"); err != nil {
		t.Fatal(err)
	}
	work["period"] = map[string]any{"from": "2024/06"}
	if _, err := s.CreateWork(work, "", "bad-work"); err == nil || !strings.Contains(err.Error(), "/period/from") {
		t.Fatalf("bad period must be rejected with path, got %v", err)
	}
	if _, err := s.UpdateWork("adaptor-physx", map[string]any{"status": "done"}, nil, nil); err == nil || !strings.Contains(err.Error(), "/status") {
		t.Fatalf("bad enum must be rejected, got %v", err)
	}
	doc, _, err = s.GetWork("adaptor-physx")
	if err != nil || doc.Frontmatter["status"] != "active" {
		t.Fatalf("work unchanged after failed update: %v %v", doc, err)
	}
}
