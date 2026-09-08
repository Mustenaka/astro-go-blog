# AI 操作说明

给未来的自己看：AI（Claude Code 等）改动这个站点的三条路径，它们与后端接口的对应关系，以及出问题时怎么查。

## 三条路径

| 路径 | 用什么 | 适合 |
|---|---|---|
| 直接写文件 | 编辑 `content/` 下的 MDX/YAML，跑 `pnpm -C site build` | 改正文、改字段、批量迁移。所有校验在构建时发生 |
| MCP 工具 | `.mcp.json` 里的 `blog` 服务器，`go run -C api ./cmd/mcp` | 让 AI 端到端完成"写文章 → 传图 → 校验 → 触发构建"，每一步都有校验与结构化返回 |
| 后台 | http://localhost:8080/admin/ | 人手上传媒体、看构建日志、看统计 |

三条路径写出的文件格式一致：front matter 由同一份 schema 校验（`site/src/content/schemas.ts` → `content/schema/*.json`）。

## MCP 工具与后端的对应关系

MCP 服务器是独立进程（stdio），复用 `api/internal/` 里的模块，读同一份 `api/.env`，与 HTTP 服务器共享同一个 SQLite 文件（WAL）和同一个存储目录。

| MCP 工具 | 做什么 | 对应的后端模块 / 接口 |
|---|---|---|
| `list_posts` / `get_post` | 读 `content/posts/**/*.mdx` | `internal/content.Store`（无 HTTP 接口） |
| `create_post` / `update_post` | 校验后写 `content/posts/<年>/<slug>.mdx` | 同上；校验 = `content/schema/posts.schema.json` |
| `create_work` / `update_work` | 写 `content/works/<slug>.mdx` | 同上；`works.schema.json` |
| `upload_asset` | 本地文件 → （图片转 WebP、限最长边）→ `Storage.Put` → `assets` 表 | 等价于后台的 `presign` + PUT + `complete`，只是在进程内完成 |
| `list_assets` | 查 `assets` 表 | `GET /admin/api/assets` |
| `validate_content` | `pnpm -C site exec astro sync` + `pnpm -C site check` | 无接口；等价于构建的 dry run |
| `trigger_build` / `build_status` | 写 `build_jobs`，跑 `RELEASE_SCRIPT` | `POST /admin/api/build`、`GET /admin/api/build/{id}` |
| `get_stats` | 查 `counters` 表 | `GET /admin/api/stats` |

注意：MCP 进程有自己的构建 runner，与 HTTP 服务器的 runner 不互斥。两边同时触发会并行跑两个 `pnpm -C site build`，通常无害但会慢；生产上只通过一条路径触发。

## 排错

- **工具列表为空 / 连接失败**：在仓库根跑 `go run -C api ./cmd/mcp`，看 stderr。常见原因：`api/.env` 缺 `ADMIN_PASSWORD_HASH` 或 `SESSION_SECRET`（配置校验会拒绝启动，即使 MCP 用不到它们）；`content/schema/` 不存在（跑 `pnpm -C site schema`）。
- **create_post 报字段错误**：错误里带路径（如 `/tags: got string, want array`、`additional properties 'foo' not allowed`）。对照 `AGENTS.md` 第 5 节。
- **upload_asset 报扩展名或大小**：白名单与上限在 `api/internal/storage/key.go`。
- **validate_content 失败**：把返回的 `output` 尾部贴出来，通常是 MDX 语法或 front matter；也可以本地跑 `pnpm -C site check`。
- **trigger_build 卡在 queued**：说明另一个构建在跑；`build_status` 不带 id 看最新一个。日志在 `api/data/builds/<id>.log`。
- **人工验证**：本会话无法重载自己的 MCP 配置，`/mcp` 里手动调用工具的验证留到下一次会话。自动化的端到端测试在 `api/internal/mcpserver/e2e_test.go`（`go test ./...` 会跑，需要 pnpm）。
