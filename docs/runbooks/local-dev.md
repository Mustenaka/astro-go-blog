# 本地开发运行手册

两个终端：一个跑 Go 后端（8080），一个跑 Astro（4321）。开发机是 Windows 11 + PowerShell，命令同样适用于 bash。

## 0. 第一次准备

```powershell
pnpm install                       # 前台、后台依赖；同时把 KaTeX 复制到 site/public/katex/
Copy-Item site/.env.example site/.env
Copy-Item api/.env.example api/.env
```

生成管理员密码哈希（argon2id），填进 `api/.env` 的 `ADMIN_PASSWORD_HASH`：

```powershell
cd api
go run ./cmd/server hash-password            # 交互输入
# 或 go run ./cmd/server hash-password '你的密码'
```

再生成 `SESSION_SECRET`（32 字节以上随机串）：

```powershell
node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"
```

`api/.env` 的其他值保持示例默认即可：`STORAGE_KIND=local`，`DATA_DIR=./data`，`RELEASE_SCRIPT=../deploy/scripts/release.dev.ps1`，`CORS_ORIGINS=http://localhost:4321`。

后台 SPA 是 embed 进 Go 二进制的，第一次（以及改了 `admin/` 之后）要构建一次：

```powershell
pnpm build:admin                   # vue-tsc + vite build，产物复制到 api/internal/adminui/dist/
```

## 1. 终端 A：Go 后端

```powershell
cd api
go run ./cmd/server
```

看到 `server listening addr=127.0.0.1:8080 storage=local` 即可。数据库、上传文件、构建日志都在 `api/data/`（已 gitignore）。

- 后台：http://localhost:8080/admin/ （用户名 `ADMIN_USER`，密码就是你哈希前的密码）
- 健康检查：http://localhost:8080/healthz
- 本地资产：http://localhost:8080/assets/<key>

## 2. 终端 B：Astro

```powershell
pnpm -C site dev
```

http://localhost:4321 。`/assets/` 会代理到 8080，所以上传后的图片在 dev 里直接可见。文章页底部的浏览数与点赞按钮读 `PUBLIC_API_BASE=http://localhost:8080`。

Astro 7 的 `astro dev` 会转成守护进程；关闭用：

```powershell
pnpm -C site exec astro dev stop
```

## 2b. Keystatic 可视化编辑（只在 dev）

`pnpm -C site dev` 启动后打开 http://localhost:4321/keystatic 。不需要额外进程：集成在 `astro dev` 时自动注入 `/keystatic` 与 `/api/keystatic`（并临时把 `trailingSlash` 放宽为 `ignore`），`pnpm -C site build` 完全不包含它。要在非 dev 命令下强制启用：`KEYSTATIC=1`；要在 dev 下关闭：`KEYSTATIC=0`。

- 适合：改文章/页面的标题、摘要、日期、标签、分类、草稿、封面路径、正文；作品的标量字段；友链；新建草稿（slug 写成 `2026/my-post`）。
- 不适合（用 MCP 或直接写文件）：含 `![]()` 图片的文章（保存会把图片转义成文本）、`math: true` 的文章（打不开）、作品的 `links`/`gallery`/`demo`。
- 不要用编辑器里的图片按钮，上传一律走后台或 MCP。

差异记录在 `docs/decisions/0002-keystatic-roundtrip.md`。

## 3. 上传一张图并在文章里引用

1. 打开 http://localhost:8080/admin/ ，登录，进入"媒体库"。
2. 前缀选 `img/`，拖入或选择 `.webp`/`.png`，等进度到 100%。
3. 复制显示的 `/assets/img/YYYY/MM/<name>-<rand>.webp`。
4. 在 MDX 里写 `![说明](/assets/img/...)`，保存，dev 页面即时刷新。

上传是浏览器直接 PUT 到签发的地址（local 模式下是 Go 的 `/_local/upload/<token>`，生产是 COS 预签名 URL），文件不经过 Go 进程；sha256 在浏览器里算好后随 `complete` 回传入库。

## 4. 触发构建

后台"构建"页点"触发构建"，或：

```powershell
Invoke-RestMethod -Method Post http://localhost:8080/admin/api/build -WebSession $s
```

开发环境的 `RELEASE_SCRIPT` 只做 `pnpm -C site build`，日志在 `api/data/builds/<id>.log`，页面每 1.5 秒轮询尾部。

## 5. 测试

```powershell
cd api
go vet ./...
go test ./...
```

测试用临时目录与临时数据库，不读 `api/.env`。

## 6. 常见问题

- **登录 429**：同一用户名 + IP 连续失败五次锁 15 分钟，重启后端即清零。
- **后台 404 "admin UI not embedded"**：没跑 `pnpm build:admin`，或跑了但没重新 `go run`。
- **Cookie 不生效**：会话 cookie 带 `Secure`，Chrome/Firefox 对 `localhost` 允许，用 `127.0.0.1` 也可以；其他主机名需要 HTTPS。
- **前台看不到图片**：确认后端在跑、`site/.env` 的 `PUBLIC_ASSET_BASE=http://localhost:8080/assets`。
