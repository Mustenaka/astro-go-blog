// Package mcpserver exposes the blog operations as MCP tools over any transport.
// Every tool validates its input twice: strict JSON decoding (unknown arguments are
// rejected by name) and, for content, the JSON Schemas exported from the site.
package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/config"
	"github.com/Mustenaka/astro-go-blog/api/internal/content"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/media"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

// Deps are the shared modules the tools operate on.
type Deps struct {
	Config  *config.Config
	DB      *db.DB
	Storage storage.Storage
	Content *content.Store
	Builds  *build.Runner
	SiteDir string
	Logger  *slog.Logger
}

type server struct {
	Deps
	repoRoot string
}

// New builds the MCP server with every tool registered.
func New(d Deps) *mcp.Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	s := &server{Deps: d, repoRoot: filepath.Dir(d.Content.Root)}
	srv := mcp.NewServer(&mcp.Implementation{Name: "astro-go-blog", Title: "木十的博客", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Tools for operating the blog: content lives in content/ as MDX; assets are uploaded to " +
			"/assets/<key>; builds run the release script. Always call validate_content before trigger_build. " +
			"Front matter fields follow AGENTS.md section 5; unknown fields are rejected.",
	})
	s.register(srv)
	return srv
}

/* ---------- registration helpers ---------- */

func obj(props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func strList(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func addTool[In any](srv *mcp.Server, name, desc string, schema map[string]any, fn func(context.Context, In) (any, error)) {
	srv.AddTool(&mcp.Tool{Name: name, Description: desc, InputSchema: schema}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in In
		raw := req.Params.Arguments
		if len(bytes.TrimSpace(raw)) == 0 {
			raw = []byte("{}")
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return errResult(fmt.Sprintf("invalid arguments for %s: %v", name, err)), nil
		}
		out, err := fn(ctx, in)
		if err != nil {
			return errResult(err.Error()), nil
		}
		text, _ := json.MarshalIndent(out, "", "  ")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(text)}}, StructuredContent: out}, nil
	})
}

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: msg}}}
}

/* ---------- inputs ---------- */

type listPostsIn struct {
	Tag      string `json:"tag,omitempty"`
	Category string `json:"category,omitempty"`
	Draft    string `json:"draft,omitempty"` // all | only | exclude
	Q        string `json:"q,omitempty"`
}

type slugIn struct {
	Slug string `json:"slug"`
}

type createPostIn struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Date        string   `json:"date,omitempty"`
	Updated     string   `json:"updated,omitempty"`
	Tags        []string `json:"tags"`
	Categories  []string `json:"categories"`
	Draft       *bool    `json:"draft,omitempty"`
	Cover       string   `json:"cover,omitempty"`
	Math        *bool    `json:"math,omitempty"`
	Lang        string   `json:"lang,omitempty"`
	Series      string   `json:"series,omitempty"`
	LegacyUrls  []string `json:"legacyUrls,omitempty"`
	Toc         *bool    `json:"toc,omitempty"`
	Body        string   `json:"body"`
	Slug        string   `json:"slug,omitempty"`
}

type updateIn struct {
	Slug        string         `json:"slug"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	Unset       []string       `json:"unset,omitempty"`
	Body        *string        `json:"body,omitempty"`
}

type createWorkIn struct {
	Title    string            `json:"title"`
	Summary  string            `json:"summary"`
	Period   map[string]string `json:"period"`
	Role     string            `json:"role,omitempty"`
	Stack    []string          `json:"stack"`
	Links    map[string]string `json:"links,omitempty"`
	Cover    string            `json:"cover"`
	Gallery  []map[string]any  `json:"gallery,omitempty"`
	Demo     map[string]any    `json:"demo,omitempty"`
	Featured *bool             `json:"featured,omitempty"`
	Order    *float64          `json:"order,omitempty"`
	Status   string            `json:"status"`
	Body     string            `json:"body"`
	Slug     string            `json:"slug,omitempty"`
}

type uploadAssetIn struct {
	Path         string `json:"path"`
	Prefix       string `json:"prefix,omitempty"`
	KeepOriginal bool   `json:"keepOriginal,omitempty"`
	MaxEdge      int    `json:"maxEdge,omitempty"`
	DemoName     string `json:"demoName,omitempty"`
	DemoVersion  string `json:"demoVersion,omitempty"`
	RelativePath string `json:"relativePath,omitempty"`
}

type listAssetsIn struct {
	Prefix string `json:"prefix,omitempty"`
	Q      string `json:"q,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type buildStatusIn struct {
	ID *int64 `json:"id,omitempty"`
}

type emptyIn struct{}

/* ---------- registration ---------- */

func (s *server) register(srv *mcp.Server) {
	addTool(srv, "list_posts", "列出文章（content/posts）。可按标签、分类、草稿状态、关键字筛选，返回 slug、标题、日期、路径。",
		obj(map[string]any{
			"tag":      str("只返回带此标签的文章"),
			"category": str("只返回此分类的文章"),
			"draft":    map[string]any{"type": "string", "enum": []string{"all", "only", "exclude"}, "description": "草稿过滤，默认 all"},
			"q":        str("标题或描述包含的关键字（不区分大小写）"),
		}), s.listPosts)

	addTool(srv, "get_post", "读取一篇文章的 front matter 与正文。",
		obj(map[string]any{"slug": str("文章 slug（文件名，不含扩展名）")}, "slug"), s.getPost)

	addTool(srv, "create_post", "新建文章，写到 content/posts/<年>/<slug>.mdx。参数名与 front matter 字段一致（AGENTS.md 5.1）；未知字段会被拒绝；slug 留空时由标题生成（中文转拼音）；已存在则拒绝。",
		obj(map[string]any{
			"title":       str("标题"),
			"description": str("一两句摘要，用于列表、OG、RSS、llms.txt"),
			"date":        str("发布日期 YYYY-MM-DD，默认今天"),
			"updated":     str("更新日期 YYYY-MM-DD"),
			"tags":        strList("标签，尽量复用已有标签；可为空数组"),
			"categories":  strList("分类，一到两个"),
			"draft":       boolean("草稿，默认 false"),
			"cover":       str("封面，/assets/img/... 路径"),
			"math":        boolean("含 LaTeX 公式时设为 true"),
			"lang":        str("默认 zh-CN"),
			"series":      str("系列名"),
			"legacyUrls":  strList("旧站路径，如 /index.php/2024/12/20/slug/"),
			"toc":         boolean("是否显示目录，默认 true"),
			"body":        str("Markdown/MDX 正文"),
			"slug":        str("可选，指定 slug（小写字母、数字、连字符）"),
		}, "title", "description", "tags", "categories", "body"), s.createPost)

	addTool(srv, "update_post", "按 slug 更新文章：frontmatter 里的字段合并进去（其他字段保留），unset 删除字段，body 整体替换正文。结果仍须通过 schema 校验。",
		obj(map[string]any{
			"slug":        str("文章 slug"),
			"frontmatter": map[string]any{"type": "object", "description": "要更新的 front matter 字段（AGENTS.md 5.1）", "additionalProperties": true},
			"unset":       strList("要删除的字段名"),
			"body":        str("新的完整正文；省略则不改"),
		}, "slug"), s.updatePost)

	addTool(srv, "create_work", "新建作品，写到 content/works/<slug>.mdx。参数名与 front matter 字段一致（AGENTS.md 5.2）。",
		obj(map[string]any{
			"title":    str("作品名"),
			"summary":  str("一句话摘要"),
			"period":   map[string]any{"type": "object", "properties": map[string]any{"from": str("YYYY-MM"), "to": str("YYYY-MM，进行中留空")}, "required": []string{"from"}, "additionalProperties": false},
			"role":     str("角色"),
			"stack":    strList("技术栈"),
			"links":    map[string]any{"type": "object", "properties": map[string]any{"repo": str("仓库 URL"), "demo": str("演示 URL 或站内路径"), "post": str("相关文章路径")}, "additionalProperties": false},
			"cover":    str("封面 /assets/img/... 路径"),
			"gallery":  map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"type": map[string]any{"type": "string", "enum": []string{"image", "video"}}, "src": str("/assets/... 路径"), "poster": str("视频海报"), "caption": str("说明")}, "required": []string{"type", "src"}, "additionalProperties": false}},
			"demo":     map[string]any{"type": "object", "description": "{kind: island, name} 或 {kind: iframe, src, aspect?}", "additionalProperties": true},
			"featured": boolean("首页展示"),
			"order":    map[string]any{"type": "number", "description": "排序，小在前"},
			"status":   map[string]any{"type": "string", "enum": []string{"active", "wip", "archived"}},
			"body":     str("Markdown/MDX 正文"),
			"slug":     str("可选，指定 slug"),
		}, "title", "summary", "period", "stack", "cover", "status", "body"), s.createWork)

	addTool(srv, "update_work", "按 slug 更新作品：frontmatter 合并、unset 删除字段、body 替换正文。",
		obj(map[string]any{
			"slug":        str("作品 slug"),
			"frontmatter": map[string]any{"type": "object", "description": "要更新的 front matter 字段（AGENTS.md 5.2）", "additionalProperties": true},
			"unset":       strList("要删除的字段名"),
			"body":        str("新的完整正文；省略则不改"),
		}, "slug"), s.updateWork)

	addTool(srv, "upload_asset", "上传本地文件到资产存储，返回可写进内容的 /assets/<key>。img/ 前缀的 PNG/JPEG/GIF/WebP 自动转 WebP 并把最长边限制在 2000 像素（keepOriginal 可关闭）。",
		obj(map[string]any{
			"path":         str("本地文件路径，绝对路径或相对仓库根"),
			"prefix":       map[string]any{"type": "string", "enum": storage.Prefixes, "description": "资产前缀，默认 img"},
			"keepOriginal": boolean("不转换、不缩放"),
			"maxEdge":      integer("最长边像素，默认 2000"),
			"demoName":     str("demos/ 前缀必填：demo 名"),
			"demoVersion":  str("demos/ 前缀必填：版本"),
			"relativePath": str("demos/ 前缀必填：文件在构建目录内的相对路径"),
		}, "path"), s.uploadAsset)

	addTool(srv, "list_assets", "按前缀与关键字查资产表。",
		obj(map[string]any{"prefix": str("前缀过滤"), "q": str("key 或原文件名包含的关键字"), "limit": integer("最多返回条数，默认 50")}), s.listAssets)

	addTool(srv, "validate_content", "校验全部内容：跑 astro sync（内容集合与 schema）与 astro check（类型）。发布前先调用。",
		obj(map[string]any{}), s.validateContent)

	addTool(srv, "trigger_build", "触发一次构建（执行 RELEASE_SCRIPT）。同一时间一个在跑、一个排队，第三个被拒绝。", obj(map[string]any{}), s.triggerBuild)

	addTool(srv, "build_status", "查看构建状态与日志尾部；不传 id 返回最近一次。",
		obj(map[string]any{"id": integer("构建任务 id")}), s.buildStatus)

	addTool(srv, "get_stats", "浏览与点赞统计：总量与前二十。", obj(map[string]any{}), s.getStats)
}

/* ---------- content tools ---------- */

func (s *server) listPosts(_ context.Context, in listPostsIn) (any, error) {
	posts, err := s.Content.ListPosts()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(in.Q))
	out := make([]content.Entry, 0, len(posts))
	for _, p := range posts {
		if in.Tag != "" && !containsStr(p.Tags, in.Tag) {
			continue
		}
		if in.Category != "" && !containsStr(p.Categories, in.Category) {
			continue
		}
		switch in.Draft {
		case "only":
			if !p.Draft {
				continue
			}
		case "exclude":
			if p.Draft {
				continue
			}
		case "", "all":
		default:
			return nil, fmt.Errorf("draft must be all, only or exclude")
		}
		if q != "" && !strings.Contains(strings.ToLower(p.Title), q) && !strings.Contains(strings.ToLower(p.Description), q) {
			continue
		}
		out = append(out, p)
	}
	return map[string]any{"count": len(out), "posts": out}, nil
}

func (s *server) getPost(_ context.Context, in slugIn) (any, error) {
	doc, path, err := s.Content.GetPost(in.Slug)
	if err != nil {
		return nil, err
	}
	return map[string]any{"slug": in.Slug, "path": path, "frontmatter": doc.Frontmatter, "body": doc.Body}, nil
}

func (s *server) createPost(_ context.Context, in createPostIn) (any, error) {
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if in.Categories == nil {
		in.Categories = []string{}
	}
	fm, err := toMap(in, "body", "slug")
	if err != nil {
		return nil, err
	}
	path, err := s.Content.CreatePost(fm, in.Body, in.Slug)
	if err != nil {
		return nil, err
	}
	slug := strings.TrimSuffix(filepath.Base(path), ".mdx")
	return map[string]any{"slug": slug, "path": path, "url": "/posts/" + slug + "/"}, nil
}

func (s *server) updatePost(_ context.Context, in updateIn) (any, error) {
	if in.Frontmatter == nil && in.Unset == nil && in.Body == nil {
		return nil, errors.New("nothing to update: pass frontmatter, unset or body")
	}
	path, err := s.Content.UpdatePost(in.Slug, in.Frontmatter, in.Unset, in.Body)
	if err != nil {
		return nil, err
	}
	return map[string]any{"slug": in.Slug, "path": path}, nil
}

func (s *server) createWork(_ context.Context, in createWorkIn) (any, error) {
	if in.Stack == nil {
		in.Stack = []string{}
	}
	fm, err := toMap(in, "body", "slug")
	if err != nil {
		return nil, err
	}
	path, err := s.Content.CreateWork(fm, in.Body, in.Slug)
	if err != nil {
		return nil, err
	}
	slug := strings.TrimSuffix(filepath.Base(path), ".mdx")
	return map[string]any{"slug": slug, "path": path, "url": "/works/" + slug + "/"}, nil
}

func (s *server) updateWork(_ context.Context, in updateIn) (any, error) {
	if in.Frontmatter == nil && in.Unset == nil && in.Body == nil {
		return nil, errors.New("nothing to update: pass frontmatter, unset or body")
	}
	path, err := s.Content.UpdateWork(in.Slug, in.Frontmatter, in.Unset, in.Body)
	if err != nil {
		return nil, err
	}
	return map[string]any{"slug": in.Slug, "path": path}, nil
}

/* ---------- assets ---------- */

func (s *server) uploadAsset(ctx context.Context, in uploadAssetIn) (any, error) {
	if in.Prefix == "" {
		in.Prefix = storage.PrefixImg
	}
	p := in.Path
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.repoRoot, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", in.Path, err)
	}
	name := filepath.Base(p)
	contentType := ""
	converted := false
	width, height := 0, 0
	if in.Prefix == storage.PrefixImg {
		res, err := media.PrepareImage(data, name, media.Options{MaxEdge: in.MaxEdge, KeepOriginal: in.KeepOriginal})
		if err != nil {
			return nil, err
		}
		data, name, contentType, converted, width, height = res.Data, res.Filename, res.ContentType, res.Converted, res.Width, res.Height
	} else {
		contentType = storage.ContentTypeFor(name)
		if i := strings.Index(contentType, ";"); i > 0 {
			contentType = contentType[:i]
		}
	}
	validateName := name
	if in.Prefix == storage.PrefixDemos && in.RelativePath != "" {
		validateName = in.RelativePath
	}
	if err := storage.ValidateFile(in.Prefix, validateName, contentType, int64(len(data))); err != nil {
		return nil, err
	}
	key, err := storage.BuildKey(storage.KeyOptions{Prefix: in.Prefix, Filename: name, DemoName: in.DemoName, DemoVersion: in.DemoVersion, RelativePath: in.RelativePath})
	if err != nil {
		return nil, err
	}
	if err := s.Storage.Put(ctx, key, contentType, bytes.NewReader(data), int64(len(data))); err != nil {
		return nil, fmt.Errorf("store %s: %w", key, err)
	}
	sum := sha256.Sum256(data)
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO assets (key, prefix, original_name, size, mime, sha256, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET size = excluded.size, mime = excluded.mime, sha256 = excluded.sha256`,
		key, storage.PrefixOf(key), filepath.Base(in.Path), len(data), contentType, hex.EncodeToString(sum[:]), db.Now()); err != nil {
		return nil, err
	}
	return map[string]any{
		"key": key, "assetPath": "/assets/" + key, "publicUrl": s.Storage.PublicURL(key),
		"size": len(data), "contentType": contentType, "converted": converted, "width": width, "height": height,
	}, nil
}

func (s *server) listAssets(ctx context.Context, in listAssetsIn) (any, error) {
	if in.Limit <= 0 || in.Limit > 500 {
		in.Limit = 50
	}
	where, args := "WHERE 1=1", []any{}
	if in.Prefix != "" {
		where += " AND prefix = ?"
		args = append(args, in.Prefix)
	}
	if q := strings.TrimSpace(in.Q); q != "" {
		where += " AND (key LIKE ? OR original_name LIKE ?)"
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT key, prefix, original_name, size, mime, created_at FROM assets `+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, append(args, in.Limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type asset struct {
		Key          string `json:"key"`
		AssetPath    string `json:"assetPath"`
		PublicURL    string `json:"publicUrl"`
		Prefix       string `json:"prefix"`
		OriginalName string `json:"originalName"`
		Size         int64  `json:"size"`
		ContentType  string `json:"contentType"`
		CreatedAt    string `json:"createdAt"`
	}
	items := []asset{}
	for rows.Next() {
		var a asset
		if err := rows.Scan(&a.Key, &a.Prefix, &a.OriginalName, &a.Size, &a.ContentType, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.AssetPath = "/assets/" + a.Key
		a.PublicURL = s.Storage.PublicURL(a.Key)
		items = append(items, a)
	}
	return map[string]any{"count": len(items), "assets": items}, rows.Err()
}

/* ---------- validation, builds, stats ---------- */

type stepResult struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Duration string `json:"duration"`
	Output   string `json:"output"`
}

func (s *server) validateContent(ctx context.Context, _ emptyIn) (any, error) {
	steps := [][]string{
		{"astro sync", "pnpm", "-C", s.SiteDir, "exec", "astro", "sync"},
		{"astro check", "pnpm", "-C", s.SiteDir, "check"},
	}
	var results []stepResult
	ok := true
	for _, st := range steps {
		start := time.Now()
		cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		cmd := exec.CommandContext(cctx, st[1], st[2:]...)
		cmd.Dir = s.repoRoot
		out, err := cmd.CombinedOutput()
		cancel()
		r := stepResult{Name: st[0], OK: err == nil, Duration: time.Since(start).Round(time.Millisecond).String(), Output: tail(stripANSI(string(out)), 4000)}
		if err != nil {
			ok = false
			if r.Output == "" {
				r.Output = err.Error()
			}
		}
		results = append(results, r)
		if !ok {
			break
		}
	}
	return map[string]any{"ok": ok, "steps": results}, nil
}

func (s *server) triggerBuild(ctx context.Context, _ emptyIn) (any, error) {
	job, err := s.Builds.Trigger(ctx, "mcp")
	if err != nil {
		return nil, err
	}
	return map[string]any{"job": job}, nil
}

func (s *server) buildStatus(ctx context.Context, in buildStatusIn) (any, error) {
	var job *build.Job
	var err error
	if in.ID != nil {
		job, err = s.Builds.Get(ctx, *in.ID)
	} else {
		var recent []*build.Job
		recent, err = s.Builds.Recent(ctx, 1)
		if err == nil && len(recent) == 0 {
			return map[string]any{"job": nil, "log": ""}, nil
		}
		if err == nil {
			job = recent[0]
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("build job not found")
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"job": job, "log": stripANSI(s.Builds.LogTail(job, 8<<10))}, nil
}

func (s *server) getStats(ctx context.Context, _ emptyIn) (any, error) {
	type counter struct {
		Slug  string `json:"slug"`
		Views int64  `json:"views"`
		Likes int64  `json:"likes"`
	}
	top := func(order string) ([]counter, error) {
		rows, err := s.DB.QueryContext(ctx, `SELECT slug, views, likes FROM counters ORDER BY `+order+` DESC, slug LIMIT 20`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []counter{}
		for rows.Next() {
			var c counter
			if err := rows.Scan(&c.Slug, &c.Views, &c.Likes); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	views, err := top("views")
	if err != nil {
		return nil, err
	}
	likes, err := top("likes")
	if err != nil {
		return nil, err
	}
	var totalViews, totalLikes int64
	_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(views),0), COALESCE(SUM(likes),0) FROM counters`).Scan(&totalViews, &totalLikes)
	return map[string]any{"totalViews": totalViews, "totalLikes": totalLikes, "topViews": views, "topLikes": likes}, nil
}

/* ---------- helpers ---------- */

// toMap converts a typed input to a front matter map via JSON, dropping the listed keys.
func toMap(v any, drop ...string) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for _, k := range drop {
		delete(m, k)
	}
	return m, nil
}

func containsStr(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
