# 阶段 4 报告：MCP 服务器与 AI 操作

日期：2026-09-08。提交：见 `git log --grep "stage 4"`。

## 做了什么

**schema 只定义一次**：zod 定义从 `content.config.ts` 抽到 `site/src/content/schemas.ts`（不含 Astro 虚拟模块）。`site/scripts/export-schema.mjs` 用 `z.toJSONSchema` 导出 draft 2020-12 到 `content/schema/{posts,works,pages,friends}.schema.json`（`z.coerce.date()` 覆写为 `type: string, format: date`；`strictObject` 生成 `additionalProperties: false`）。`pnpm -C site schema` 单独跑，`pnpm -C site build` 开头自动跑；Node 22 用 `--experimental-strip-types` 直接加载 `.ts`。生成文件已提交。

**Go 内容层** `api/internal/content`：
- `frontmatter.go`：解析 `---` 块（yaml.v3 Node 保留键顺序与序列的 flow/block 样式），`time.Time` 归一为 `YYYY-MM-DD`，写回时 `date`/`updated` 输出裸时间戳，其余按 AGENTS.md 表格顺序。手写示例文件 round-trip 无 diff（有测试）。
- `schema.go`：`santhosh-tekuri/jsonschema/v6` 加载 `content/schema/*.json`，`AssertFormat` 打开，错误带字段路径（如 `/tags: got string, want array`、`(root): additional properties 'bogusField' not allowed`）。
- `slug.go`：标题转 slug，汉字用 go-pinyin（`SPH 公式总结` → `sph-gong-shi-zong-jie`），可显式指定。
- `store.go`：posts 与 works 的 list/get/create/update；create 校验后写 `content/posts/<年>/<slug>.mdx`，已存在拒绝；update 合并字段、`unset` 删字段、可整体替换正文，校验失败不落盘。

**图片处理** `api/internal/media`：PNG/JPEG/GIF/WebP 解码，最长边限 2000（`x/image/draw` CatmullRom），`gen2brain/webp`（纯 Go，wazero 跑 libwebp）编码质量 82；`keepOriginal` 或非图片直通，SVG 永不转换。

**存储**：`Storage` 接口加 `Put(ctx, key, contentType, reader, size)`，local 直接落盘，COS 走预签名 PUT。浏览器上传路径不变。

**MCP 服务器** `api/cmd/mcp` + `api/internal/mcpserver`：`modelcontextprotocol/go-sdk` v1.7.0，stdio 传输，复用 config/db/storage/build/content，读同一份 `api/.env`，与 HTTP 服务器共享 SQLite（WAL）和存储目录。日志走 stderr。工具清单（最终版）：

| 工具 | 输入 | 输出 |
|---|---|---|
| `list_posts` | `tag?` `category?` `draft?`(all/only/exclude) `q?` | `count`, `posts[]{slug,path,title,date,description,tags,categories,draft}` |
| `get_post` | `slug` | `slug,path,frontmatter,body` |
| `create_post` | `title` `description` `tags` `categories` `body` 必填；`date?` `updated?` `draft?` `cover?` `math?` `lang?` `series?` `legacyUrls?` `toc?` `slug?` | `slug,path,url` |
| `update_post` | `slug`；`frontmatter?`(对象) `unset?`(字段名) `body?` | `slug,path` |
| `create_work` | `title` `summary` `period{from,to?}` `stack` `cover` `status` `body` 必填；`role?` `links?` `gallery?` `demo?` `featured?` `order?` `slug?` | `slug,path,url` |
| `update_work` | 同 `update_post` | `slug,path` |
| `upload_asset` | `path`；`prefix?`(默认 img) `keepOriginal?` `maxEdge?` `demoName?` `demoVersion?` `relativePath?` | `key,assetPath,publicUrl,size,contentType,converted,width,height` |
| `list_assets` | `prefix?` `q?` `limit?` | `count,assets[]` |
| `validate_content` | 无 | `ok, steps[]{name,ok,duration,output}`：`astro sync` + `astro check` |
| `trigger_build` | 无 | `job` |
| `build_status` | `id?`（省略取最近） | `job, log`（尾部 8KB） |
| `get_stats` | 无 | `totalViews,totalLikes,topViews[],topLikes[]` |

每个工具的输入 schema 手写（含中文描述，`additionalProperties: false`），处理端用 `DisallowUnknownFields` 严格解码，未知参数返回 `isError` 并点名字段；内容类工具再过一遍 JSON Schema。

**配置与文档**：`.mcp.json`（`go run -C api ./cmd/mcp`，cwd 为 `api/` 所以能读 `.env`）、`AGENTS.md` 第 11 节"AI 操作指南"（三种方式、写文章检查清单、禁止事项）、`docs/ai-operations.md`（工具与后端接口对应表、排错）、`content/README.md`。

## 怎么验证的

| 验收项 | 结果 |
|---|---|
| `go vet ./...` | 通过 |
| `go test ./...` | auth、content、httpapi、mcpserver、media、storage 全部 ok |
| 端到端（`internal/mcpserver/e2e_test.go`，`go build` 出 `cmd/mcp` 二进制，Go SDK 客户端经 stdio 连接） | 31 秒通过：ListTools 12 个 → `create_post` 传 `bogusField` 被拒且错误含字段名 → `create_post`（含公式与 Go 代码块，`math: true`，`draft: true`）写到 `content/posts/2026/mcp-e2e-<ts>.mdx` → 同 slug 再建被拒 → `upload_asset` 2400×1200 PNG 变成 2000×1000 WebP 存到 `img/2026/09/e2e-shot-<rand>.webp`（磁盘上解码验证）→ `README.md` 传 img/ 被拒 → `update_post` 把封面与图片插进文章（文件内容验证）→ `update_post` 未知字段 `nope` 被拒 → `get_post`/`list_posts`/`list_assets` 看到新内容 → `validate_content` ok（astro sync + check）→ `trigger_build` → `build_status` 轮询到 `succeeded`，日志含 `page(s) built` → `get_stats` 正常。测试结束删除临时文章，仓库 `content/` 无残留 |
| 单元（`server_test.go`，内存传输） | 12 个工具逐个传未知参数全部被拒并点名；每个工具至少一条成功路径或明确的失败路径：缺 `description`、坏日期 `/date`、拼音 slug、`get_post` 未找到、`list_posts` 非法 draft、`update_post` 空操作/类型错误/成功、`create_work` 成功与非法 status、`update_work` 成功与非法 demo、`upload_asset` 文件不存在/非法前缀、无 `RELEASE_SCRIPT` 时 `trigger_build` 明确拒绝、`build_status` 空与未知 id、`get_stats` |
| `content` 包测试 | 手写示例 round-trip 无 diff；Slugify 四例；创建/更新/校验（未知字段、类型、必填、日期、`/period/from`、枚举，失败不落盘） |
| `media` 包测试 | 3000×1500 PNG → 2000×1000 WebP 并可解码；小图不放大；`keepOriginal` 与 SVG 直通；坏文件报错 |
| `pnpm -C site check` / `build` | 43 files 0 errors；23 页面，schema 导出 0 变化 |
| `/mcp` 人工验证 | **留到下一次会话**：本会话无法重新加载自己的 MCP 配置。 |

## 没做或有疑问的

- **`/mcp` 人工验证留到下一次会话**（见上）。建议第一步跑 `list_posts`，再 `create_post` 一篇草稿。
- **两个构建 runner**：MCP 进程与 HTTP 进程各有一个 runner，互不知晓，同时触发会并行跑两个构建。已写进 `docs/ai-operations.md`；生产只用一条路径触发。
- **update 会规范化 front matter 写法**：键顺序按 AGENTS.md 表、短标量数组写成 flow 样式（`[a, b]`）。手写文件的原有样式会尽量保留（顺序、flow/block），但注释会丢失。
- **`validate_content` 的耗时**：astro sync + astro check 约 20 秒，是内容与类型的完整校验；不做真正的 `astro build`。
- **图片转换用 wazero 跑 libwebp**：首次调用有约 1 秒 JIT 开销，纯 Go 无 cgo。
- **`.mcp.json` 需要 `api/.env`**：配置校验要求 `ADMIN_PASSWORD_HASH` 与 `SESSION_SECRET` 非空，MCP 本身用不到。
- `tsconfig.json` 现在 exclude `scripts/`，因为 `.mjs` 脚本用 `.ts` 扩展名导入，不进 `astro check`。

## 新增依赖及理由

Go：
- `github.com/modelcontextprotocol/go-sdk` v1.7.0：宪法指定。
- `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3：Go 侧按导出的 JSON Schema 校验 front matter，避免两边漂移；完整 2020-12 实现，带 `format: date` 断言。
- `gopkg.in/yaml.v3` v3.0.1：解析与写回 front matter，Node API 能保留键顺序与样式。
- `github.com/mozillazg/go-pinyin` v0.21.0：中文标题生成拼音 slug（宪法要求"拼音或英文"）。
- `golang.org/x/image` v0.45.0：高质量缩放（CatmullRom）。
- `github.com/gen2brain/webp` v0.6.4：Go 标准库没有 WebP 编码器；它用 wazero 跑 libwebp，纯 Go 无 cgo（间接引入 `wazero`、`purego`）。
- `golang.org/x/text`：jsonschema 的错误本地化打印需要 `message.Printer`，本来就是其间接依赖，现在直接引用。

前端：无新增（`astro/zod` 已随 Astro 存在）。
