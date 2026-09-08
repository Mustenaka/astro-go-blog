# 阶段 3 报告：Go 后端与最小后台

日期：2026-09-08。提交：见 `git log --grep "stage 3"`。

## 做了什么

**骨架** `api/`：Go module `github.com/Mustenaka/astro-go-blog/api`，`go 1.25.4`。目录 `cmd/server`、`cmd/mcp`（占位）、`internal/{config,db,storage,auth,httpapi,build,adminui}`。配置全部来自环境变量，开发时 godotenv 读 `api/.env`；`api/.env.example` 列全第 8 节变量并加了 `MAIN_BRANCH`。`server hash-password [密码]` 输出 argon2id（m=64MiB, t=3, p=2）PHC 格式哈希。

**数据库**：`modernc.org/sqlite`，WAL，`busy_timeout=5000`，单连接。迁移是 `embed` 的编号 SQL（`internal/db/migrations/0001_init.sql`）加 `schema_migrations` 版本表。表结构：

```sql
assets(id PK, key UNIQUE, prefix, original_name, size, mime, sha256, created_at)  -- idx (prefix, created_at DESC)
counters(slug PK, views, likes, updated_at)
like_events(slug, ip_hash, day, created_at)  PRIMARY KEY (slug, ip_hash, day)     -- 每天一次去重
sessions(id PK, username, ip_hash, created_at, expires_at)                        -- idx expires_at
build_jobs(id PK, status, trigger, created_at, started_at, finished_at, exit_code, log_path)
schema_migrations(version PK, name, applied_at)
```

浏览计数的十分钟去重在内存里（`viewDedupe`），重启清零；点赞的每日去重落库。IP 只存 HMAC-SHA256(SESSION_SECRET, ip) 的前 32 位。

**存储适配器** `internal/storage`：

```go
type Storage interface {
    Presign(ctx context.Context, key, contentType string, size int64) (*PresignedUpload, error)
    PublicURL(key string) string
    Delete(ctx context.Context, key string) error
    List(ctx context.Context, prefix string) ([]ObjectInfo, error)
    Stat(ctx context.Context, key string) (*ObjectInfo, error)
}
type PresignedUpload struct { URL, Method string; Headers map[string]string; ExpiresAt time.Time }
type ObjectInfo struct { Key string; Size int64; ContentType string; ModTime time.Time }
```

- `local`：文件在 `DATA_DIR/assets/<key>`；`Presign` 返回一次性 `PUT /_local/upload/<token>`（15 分钟有效，用后即焚，校验 Content-Type 与长度）；`GET /assets/<key>` 由 Go 直接提供，`.br` 返回 `Content-Encoding: br` 并按去掉 `.br` 后的扩展名给 Content-Type（`.wasm.br` → `application/wasm`，`.js.br` → `text/javascript`，`.data.br` → `application/octet-stream`），`demos/` 前缀加 COOP `same-origin` 与 COEP `require-corp`，所有资产带 CORP `cross-origin` 与 `Access-Control-Allow-Origin: *`，不带 X-Frame-Options（Unity 页面要进 iframe）。
- `cos`：`cos-go-sdk-v5` 的 `GetPresignedURL(PUT)` 签 Content-Type，`PublicURL` 拼 `ASSET_PUBLIC_BASE`，`Delete`/`Head`/`Bucket.Get`。单元测试离线验证预签名 URL 含 bucket 域名、`q-signature`、`q-ak`。真实凭据验证留阶段 7。
- key 规则（`key.go`）：后端按 `prefix` 与原文件名生成 `<prefix>/YYYY/MM/<slug>-<6 位随机>.<ext>`，`demos/` 为 `demos/<name>/<version>/<relativePath>`，客户端不能指定 key。扩展名白名单与 MIME 对应表、按前缀的大小上限（img 20MB、video 300MB、models/files 100MB、demos 500MB）、`..` 与非法字符全部拒绝。

**接口**（全部在 `api/openapi.yaml`）：公开 `GET /api/v1/counters`、`POST /api/v1/counters/{slug}/view`、`POST /api/v1/likes/{slug}`、`GET /healthz`；管理 `/admin/api/{login,logout,me,assets/presign,assets/complete,assets,assets/{id},build,build/{id},stats}`；`POST /hooks/github`；local 模式的 `/_local/upload/{token}` 与 `/assets/{key...}`。公开写接口有每客户端令牌桶（30/分钟，突发 60），CORS 只对 `/api/v1/` 且只放行 `CORS_ORIGINS`；登录五次失败锁 15 分钟；会话 cookie `HttpOnly; Secure; SameSite=Strict`，12 小时；所有响应带 `X-Content-Type-Options`、`Referrer-Policy`、`Permissions-Policy`，非资产响应再带 `X-Frame-Options: DENY` 与 CSP `default-src 'none'`，后台 `index.html` 有自己的 CSP。所有 SQL 参数化。

**构建触发** `internal/build`：执行 `RELEASE_SCRIPT`（`.ps1` 走 PowerShell，`.sh` 走 sh），异步，日志写 `DATA_DIR/builds/<id>.log`，同一时间一个在跑、一个排队、第三个 409。进程重启时把遗留的 running/queued 标为 failed。开发脚本 `deploy/scripts/release.dev.ps1`（另附 `.sh`）只做 `pnpm -C site build`。

**内嵌后台** `admin/`：Vue 3 + Vite 8 + TS，hash 路由，页面：登录、媒体库（前缀选择、拖拽上传、进度、浏览器算 sha256、完成后显示 `/assets/<key>` 与一键复制、图片缩略、前缀/关键字筛选、删除）、构建（按钮 + 1.5 秒轮询日志尾部 + 最近十次）、统计（浏览/点赞前二十）。`pnpm build:admin` = `vue-tsc` + `vite build` + 复制到 `api/internal/adminui/dist/`（gitignore，保留 `.gitkeep`），Go 用 `embed` 在 `/admin/` 提供。

**前台接入**：`site/src/components/Counters.vue` 岛屿挂在文章页脚，读 `PUBLIC_API_BASE`（空字符串即同源），API 不可达时自动隐藏。阶段 1 示例文章 `hello-astro.mdx` 的三处图片引用改为通过后台真实上传得到的 `/assets/img/2026/09/sample-802972.webp`（ffmpeg 生成的 1200×675 测试图）。

**文档**：`api/openapi.yaml`、`docs/runbooks/local-dev.md`、`AGENTS.md` 追加"后端常用命令"与"上传一个资产的步骤"。

## 怎么验证的

| 验收项 | 结果 |
|---|---|
| `go vet ./...` | 通过 |
| `go test ./...` | `auth`、`httpapi`（11 个用例）、`storage` 全部 ok，httpapi 5.7s |
| 测试覆盖 | 登录成功/失败五次锁定/登出、未登录访问九个管理接口全 401、presign 拒绝非法扩展名/超限/MIME 不符/未知前缀/缺 demo 名、local 上传完整链路（presign → PUT → 令牌一次性 → complete 前置校验 → GET 资产与响应头 → 列表 → 删除 → 路径穿越）、`.br` 与 `demos/` 响应头、浏览十分钟去重与 CORS 白名单、点赞每天一次、公开接口限流 429、webhook 签名（错误 401、缺失 401、正确 202、非主分支忽略）、构建互斥与排队（第三个 409，两个都 succeeded，顺序正确，日志含标记）、安全头 |
| 无头浏览器走通（Go + Astro dev 同时运行） | 登录 `http://localhost:8080/admin/` → 媒体库上传 `sample.webp` → 得到 `/assets/img/2026/09/sample-802972.webp`，缩略图可显示 → 写进 `hello-astro.mdx` 三处 → `http://localhost:4321/posts/hello-astro/` 三张图全部加载（naturalWidth 1200）→ 点赞 0→1，刷新后仍为 1，再点仍为 1；浏览数刷新后仍为 1（去重） → 后台统计页出现 `hello-astro` |
| 后台构建 | 通过 admin API 触发，13 秒后 `succeeded`，日志含 `23 page(s) built`、Pagefind `Indexed 6 pages`、`redirects.map` 8 条 |
| webhook | 错误签名 401；正确签名 202 并生成 job 3，日志 `finished exit=0` |
| `pnpm -C site check` / `build` | 0 错误；23 页面 |
| `pnpm -C admin check`（vue-tsc） | 通过 |
| 进程清理 | 验证后 `taskkill blog-server.exe`、`astro dev stop`，8080 与 4321 均已释放 |

## 没做或有疑问的

- **Go 版本**：`go get` 把 `golang.org/x/crypto` 拉到 v0.56（要求 go 1.26），已降到 v0.55.0 并把 `go.mod` 固定在 `go 1.25.4`；本机 1.25.4 通过。
- **两处修过的 bug**：`http.FileServer` 会把 `/index.html` 301 到 `/`，与 `/admin/` 形成循环，改为直接写出 `index.html`；构建脚本相对路径在切换 cwd 后找不到，`build.New` 现在先转绝对路径。
- **浏览去重在内存**：重启后同一客户端十分钟内可再计一次。要严格可改成表，暂不做。
- **`client:visible` 与无头测试**：Counters 岛屿在页脚，视口外不激活；脚本里先滚动到可见。真实用户滚到底才请求，符合预期。
- **Secure cookie 在 http://localhost**：Chrome/Firefox 允许，测试里 Go 客户端手动去掉 Secure 标记入 jar。生产走 SSH 隧道仍是 localhost，无需改。
- **COS 真实上传**未验证（无凭据），阶段 7 做。`List` 分页用 Marker 循环。
- **demos/ 目录上传**：后台支持选文件夹后按 `webkitRelativePath` 定相对路径，但没有 Unity 构建可试，阶段 5 用模拟页验证。
- 第一次 E2E 上传的 `sample-454bdd.webp` 留在本地 `api/data/`（不在仓库），可在后台删除。

## 新增依赖及理由

Go（均在宪法第 3 节）：`modernc.org/sqlite` v1.58.0、`golang.org/x/crypto` v0.55.0、`github.com/tencentyun/cos-go-sdk-v5` v0.7.75、`github.com/joho/godotenv` v1.5.1。

前端（`admin/`）：`vue` 3.5.42、`vue-router` 5.3.1（宪法要求 Vue SPA，路由需要它）、`vite` 8.2.2、`@vitejs/plugin-vue` 6.0.8、`vue-tsc` 3.3.11（`pnpm -C admin check` 的类型检查器，等价于 site 的 `astro check`）、`typescript` 5.9.3。宪法未点名的只有 `vue-router`、`@vitejs/plugin-vue`、`vue-tsc`，都是 Vue + Vite 项目的标准配套。
