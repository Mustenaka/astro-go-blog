# 木十博客重建：项目宪法

## 1. 项目是什么

- 把 https://www.mustenaka.cn 从 WordPress 重建为：**Astro 静态前台 + Go 后端 + 后台**。
- 作者：木十 / Mustenaka，软件工程师，方向是 Unity/C#、实时物理仿真、AI 应用。站点语言中文为主。
- 站点内容：技术文章（大量代码块与 LaTeX 公式）、作品集（含 three.js 交互、Unity WebGL 嵌入、演示视频）、简历、友链。
- 路线：本地开发 → 本地测试 → 部署到新服务器以临时子域名并行运行 → 迁移 WordPress 内容与资源 → 切换域名 → 旧机下线。
- 旧站数据规模：175 篇文章、6 个页面、47 条评论、143 个媒体文件、32 个分类、77 个标签。文章是经典编辑器 HTML，代码块是裸 `<pre>`，LaTeX 公式非常多。旧固定链接形如 `/index.php/YYYY/MM/DD/slug/`。

## 2. 架构与边界

| 部分 | 目录 | 职责 | 明确不做的事 |
|---|---|---|---|
| 前台 | `site/` | Astro，构建期生成静态 HTML，Nginx 直出。交互用 Vue 3 岛屿 | 不做服务端渲染，不在运行时读数据库 |
| 内容 | `content/` | Markdown/MDX 是唯一真实来源，进 Git | 内容不进数据库 |
| 后端 | `api/` | Go + SQLite。管理员认证、资产上传签发与索引、计数与点赞、构建触发、GitHub webhook、MCP 服务器、内嵌最小后台 | 不存文章，不渲染页面 |
| 后台 | `admin/` | Vue 3 SPA，构建后 embed 进 Go 二进制，路径 `/admin/` | 第一版不做文章编辑器 |
| 评论 | 外部 | Artalk 独立部署，前台以岛屿嵌入 | 不自写评论系统 |
| 资产 | 外部 | 腾讯云 COS + CDN。本地开发由 Go 的 local 存储适配器模拟 | 视频、大文件、Unity 构建不进 Git，不放服务器磁盘 |
| 部署 | `deploy/` | Nginx、systemd、发布脚本、Artalk compose、运行手册 | |
| 脚本 | `scripts/` | WordPress 迁移等一次性工具 | |

任何后台、脚本、AI 修改内容的方式只有一种：写 `content/` 下的文件。

## 3. 技术选型（已定，不重开讨论）

前台与后台：
- Node 22，pnpm 10，pnpm workspace 管理 `site/` 与 `admin/`。
- Astro 最新稳定版（当前 7.x），TypeScript strict。
- 集成：`@astrojs/vue`、`@astrojs/mdx`、`@astrojs/sitemap`、`@astrojs/rss`。
- 样式：Tailwind CSS v4，通过 `@tailwindcss/vite`。
- 公式：`remark-math` + `rehype-katex`，KaTeX 样式自托管，只在 `math: true` 的页面加载。
- 代码高亮：Astro 内置 Shiki，双主题。
- 搜索：Pagefind，构建后生成索引。
- 可视化编辑：Keystatic，仅开发模式启用，生产构建不包含。
- 三维：`three` 直接用在 Vue 组件里，组件卸载时必须释放资源。

后端：
- Go 1.25。路由用标准库 `net/http` 的方法加模式匹配，不引入 web 框架。
- 数据库 `modernc.org/sqlite`，纯 Go 无 CGO。WAL 模式。
- 日志 `log/slog`。密码哈希 `golang.org/x/crypto/argon2`。
- 对象存储 `github.com/tencentyun/cos-go-sdk-v5`。
- MCP `github.com/modelcontextprotocol/go-sdk`。
- `.env` 加载仅开发用 `github.com/joho/godotenv`。

通用：
- 开发机 Windows 11 + PowerShell。不要创建符号链接，不要假设有 bash 或 make。需要复制文件就复制。
- 行尾 LF，由 `.gitattributes` 强制。
- 新增本表以外的第三方依赖，必须在阶段报告里单独列出并说明理由。

## 4. 仓库布局

```
astro-go-blog/
  AGENTS.md  CLAUDE.md  README.md
  .gitignore  .gitattributes  .env.example  .mcp.json
  package.json  pnpm-workspace.yaml
  content/
    posts/YYYY/<slug>.mdx
    works/<slug>.mdx
    pages/<slug>.mdx
    data/friends.yaml
  site/            Astro 项目
  api/             Go 模块：cmd/server  cmd/mcp  internal/...
  admin/           Vue SPA
  deploy/          nginx/  systemd/  scripts/  artalk/
  scripts/         wp-migrate/ 等
  docs/            decisions/ 与 runbooks/
```

## 5. 内容规范

### 5.1 posts 的 front matter

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| title | string | 是 | |
| description | string | 是 | 一两句摘要，用于列表、OG、RSS、llms.txt |
| date | date | 是 | 发布日期 |
| updated | date | 否 | |
| tags | string[] | 是 | 可为空数组 |
| categories | string[] | 是 | 大类，一般一到两个 |
| draft | boolean | 否 | 默认 false。draft 为 true 的文章不出现在生产构建中 |
| cover | string | 否 | `/assets/...` 路径 |
| math | boolean | 否 | 默认 false。为 true 时页面加载 KaTeX 样式 |
| lang | string | 否 | 默认 `zh-CN` |
| series | string | 否 | 系列名，同系列文章互相链接 |
| legacyUrls | string[] | 否 | 旧站路径，如 `/index.php/2024/12/20/sph-formula-summary/`，用于生成 301 映射 |
| toc | boolean | 否 | 默认 true |

slug 取自文件名，不放在 front matter 里。文件放在 `content/posts/<发布年份>/<slug>.mdx`。

### 5.2 works 的 front matter

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| title | string | 是 | |
| summary | string | 是 | |
| period | { from: string, to?: string } | 是 | `YYYY-MM` 格式 |
| role | string | 否 | |
| stack | string[] | 是 | |
| links | { repo?, demo?, post? } | 否 | 均为 URL 或站内路径 |
| cover | string | 是 | `/assets/...` |
| gallery | { type: 'image' \| 'video', src, poster?, caption? }[] | 否 | |
| demo | { kind: 'island', name } 或 { kind: 'iframe', src, aspect? } | 否 | island 对应 `site/src/demos/<name>/`，iframe 用于 Unity WebGL |
| featured | boolean | 否 | 首页展示 |
| order | number | 否 | 排序，小在前 |
| status | 'active' \| 'wip' \| 'archived' | 是 | |

### 5.3 pages 的 front matter

title 必填，description 与 updated 可选。固定页面：`about`（简历）、`friends`（友链，数据来自 `content/data/friends.yaml`）。

### 5.4 资产引用约定

- 内容里一律写 `/assets/<key>`。key 的前缀固定为五种：`img/`、`video/`、`demos/`、`models/`、`files/`，其后按 `YYYY/MM/` 分目录，`demos/` 按 `<name>/<version>/` 分目录。
- 构建期由 rehype 插件把 `/assets/` 前缀改写为环境变量 `PUBLIC_ASSET_BASE`。开发时 Astro dev 把 `/assets/` 代理到 Go 的 local 存储。生产环境 Nginx 对残留的 `/assets/` 请求 302 到 CDN 兜底。
- 图片入库前转 WebP，最长边不超过 2000 像素，带文字的截图可保留 PNG。视频用 H.264 MP4，一两分钟的短片直传，长视频用 B 站嵌入。
- Unity WebGL 构建用 Brotli 预压缩，服务端必须对 `.br` 文件返回正确的 `Content-Encoding` 与 `Content-Type`，并对 `demos/` 前缀返回 COOP 与 COEP 头。

### 5.5 URL 方案

| 路径 | 内容 |
|---|---|
| `/` | 首页：简介、精选作品、最新文章 |
| `/posts/` 与 `/posts/page/N/` | 文章列表，分页 |
| `/posts/<slug>/` | 文章 |
| `/posts/<slug>.md` | 同一篇文章的纯 Markdown |
| `/tags/`、`/tags/<tag>/` | 标签 |
| `/categories/<cat>/` | 分类 |
| `/archive/` | 按年归档 |
| `/works/`、`/works/<slug>/` | 作品集 |
| `/works.json` | 作品集机器可读版本 |
| `/about/`、`/friends/`、`/search/` | 固定页面 |
| `/rss.xml`、`/sitemap-index.xml`、`/robots.txt`、`/llms.txt`、`/llms-full.txt` | 机器入口 |

旧站路径通过构建期生成的 Nginx map 文件做 301。固定映射：`/index.php/blog/` 到 `/posts/`，`/index.php/myprofile/` 到 `/about/`，`/index.php/myopensource/` 到 `/works/`，`/index.php/feed/` 与 `/feed/` 到 `/rss.xml`，`/index.php/download/` 返回 410。

## 6. 安全规则

这些规则来自 2026 年 7 月旧站被入侵的事故，不可放松。

- 公开站是纯静态文件，服务器上没有任何面向公网的动态渲染。
- Go 进程只监听 `127.0.0.1`。Nginx 只把 `/api/v1/` 前缀转发给它。`/admin/` 与 `/mcp` 不经 Nginx 暴露，只通过 SSH 隧道访问。
- 公开可写接口只有计数与点赞，都有基于 IP 哈希的限流与去重。没有注册，没有公开的内容写入接口。
- 上传只走预签名地址，后端校验扩展名白名单、MIME 与大小上限，文件不经过 Go 进程。
- 管理员只有一个，密码用 argon2id 存储，登录有限流，会话 cookie 是 HttpOnly、Secure、SameSite=Strict。
- 所有 SQL 参数化。所有响应带安全头，含 CSP。
- 仓库里不出现任何密钥。`.env` 在 `.gitignore` 里，只提交 `.env.example`。
- 依赖版本锁定，lockfile 提交。

## 7. AI 友好要求

被 AI 读：
- 构建期生成 `/llms.txt`（站点简介与文章目录）和 `/llms-full.txt`（全文）。
- 每篇文章有 `.md` 版本。作品集有 `/works.json`。
- 页面带 JSON-LD：文章用 BlogPosting，作品用 SoftwareSourceCode，站点用 Person 与 WebSite。
- RSS 输出全文。HTML 语义化。
- `robots.txt` 的策略集中在一个配置文件里，默认允许所有爬虫，包括 AI 爬虫。

被 AI 操作：
- `AGENTS.md` 说明如何新增文章、作品、资产，以及所有常用命令。
- 内容 schema 在构建期校验，AI 写错字段直接构建失败。
- Go 提供 MCP 服务器，工具覆盖发文、传资产、建作品、触发构建、查统计。
- 仓库根目录有 `.mcp.json`，Claude Code 打开仓库即可使用这些工具。

## 8. 环境变量

`site/.env`：`PUBLIC_SITE_URL`、`PUBLIC_ASSET_BASE`、`PUBLIC_API_BASE`、`PUBLIC_ARTALK_SERVER`、`PUBLIC_NOINDEX`。

`api/.env`：`ADDR`、`DATA_DIR`、`STORAGE_KIND`（local 或 cos）、`COS_BUCKET`、`COS_REGION`、`COS_SECRET_ID`、`COS_SECRET_KEY`、`ASSET_PUBLIC_BASE`、`ADMIN_USER`、`ADMIN_PASSWORD_HASH`、`SESSION_SECRET`、`CONTENT_DIR`、`SITE_DIR`、`RELEASE_SCRIPT`、`GITHUB_WEBHOOK_SECRET`、`CORS_ORIGINS`。

开发默认值：Astro 在 4321，Go 在 8080，`PUBLIC_API_BASE=http://localhost:8080`，`PUBLIC_ASSET_BASE=http://localhost:8080/assets`，`STORAGE_KIND=local`。生产 `PUBLIC_API_BASE` 为空字符串表示同源。

## 9. 工作方式

- 每个阶段开始先给不超过十行的计划，等我确认后再动手。
- 只改本仓库内的文件。不要读写 `D:\Work\blog`，那里有凭据。
- 阶段结束前必须实际运行并贴出结果：`pnpm -C site check`、`pnpm -C site build`、在 `api/` 下 `go vet ./...` 与 `go test ./...`。失败就如实报告，不要描述成通过。
- 对不确定的第三方 API，先看官方文档或 `node_modules` 与 Go module cache 里的类型定义，不要凭记忆猜。
- 每个阶段结束用 conventional commits 风格提交，一个阶段一个或几个提交。
- 阶段报告固定四段：做了什么、怎么验证的、没做或有疑问的、新增依赖及理由。

## 10. 常用命令

在仓库根目录执行。开发机是 PowerShell，命令同样适用于 bash。

| 目的 | 命令 | 说明 |
|---|---|---|
| 安装依赖 | `pnpm install` | 首次或 lockfile 变化后。`site/.env` 不存在时先 `Copy-Item site/.env.example site/.env` |
| 开发服务器 | `pnpm -C site dev` | http://localhost:4321 ，草稿可见，`/assets/` 代理到 http://localhost:8080 |
| 类型检查 | `pnpm -C site check` | 检查 `.astro`、`.ts`、`.vue`，含 `astro.config.ts` |
| 生产构建 | `pnpm -C site build` | 输出 `site/dist/`，`draft: true` 的文章被排除，`/assets/` 改写为 `PUBLIC_ASSET_BASE` |
| 预览构建 | `pnpm -C site preview` | 本地静态预览 `site/dist/` |

根 `package.json` 提供同名快捷方式：`pnpm dev`、`pnpm build`、`pnpm check`、`pnpm preview`。

本机按服务器方式（WSL 里的 Nginx + Go）跑一遍：`docs/runbooks/local-wsl.md`。

内容集合的定义在 `site/src/content.config.ts`。它从 `../content/` 读取，front matter 用 zod `strictObject` 校验：字段类型错误或出现未定义字段，`build` 与 `dev` 都会失败并指出文件与字段。

### 新增一篇文章的完整步骤

1. 定 slug：小写字母、数字、连字符，全站唯一（不同年份也不能重名）。文件放在 `content/posts/<发布年份>/<slug>.mdx`。
2. 写 front matter，按第 5.1 节。必填 `title`、`description`（一两句，会出现在列表、OG、RSS、llms.txt）、`date`、`tags`（尽量复用 `/tags/` 里已有的）、`categories`（一到两个）。
3. 正文用 Markdown/MDX。标题从 `##` 开始，`toc: true`（默认）时 `##` 与 `###` 进目录。
4. 图片先上传拿到 `/assets/img/YYYY/MM/<name>.webp`，再用 `![说明](/assets/...)` 引用；alt 文本就是说明。附件写 `[名字](/assets/files/...)`。
5. 有公式就设 `math: true`（行内 `$...$`，块 `$$...$$`）。代码块写语言标识，会自动带语言标签与复制按钮。
6. 正文里可以直接用 `<Demo name="…" />`、`<Video src="…" />`、`<UnityEmbed src="…" />`、`<Counter start={10} />`，不用写 import（`site/src/plugins/remark-auto-import.ts` 在构建时注入）。要加新组件，在那个映射表里加一行，并在 `site/keystatic.config.ts` 的 `mdxComponents` 里声明。
7. 从 WordPress 迁来的文章填 `legacyUrls`（旧路径，形如 `/index.php/YYYY/MM/DD/slug/`），构建会写进 `dist/redirects.map`。
8. 同系列文章填相同的 `series`，文章页会列出整个系列。
9. 未完成的文章设 `draft: true`：dev 可见，生产构建排除。
10. 跑 `pnpm -C site check` 与 `pnpm -C site build`；字段错误会直接失败并指出文件与字段。
11. 提交 `content/` 下的改动。

### 新增一个页面的步骤

固定页面来自 `content/pages/<slug>.mdx`，由 `site/src/pages/[page].astro` 渲染到 `/<slug>/`。

1. 在 `content/pages/` 新建 `<slug>.mdx`，front matter 只有 `title`（必填）、`description`、`updated`（第 5.3 节）。
2. 正文写 Markdown/MDX，规则同文章。
3. 需要出现在顶部导航时，在 `site/src/site.config.ts` 的 `nav` 里加一项。
4. 需要特殊数据（如友链读 `friends.yaml`）时，在 `[page].astro` 里按 `page.id` 分支处理，不要新建独立路由。
5. 跑 `pnpm -C site build` 确认 `dist/<slug>/index.html` 生成。

### 后端常用命令

在 `api/` 下执行。

| 目的 | 命令 | 说明 |
|---|---|---|
| 生成管理员密码哈希 | `go run ./cmd/server hash-password` | argon2id，结果填 `api/.env` 的 `ADMIN_PASSWORD_HASH` |
| 启动后端 | `go run ./cmd/server` | 读 `api/.env`，监听 `127.0.0.1:8080`，后台在 http://localhost:8080/admin/ |
| 静态检查与测试 | `go vet ./... && go test ./...` | 测试用临时目录与临时库，不读 `.env` |
| 构建后台 SPA 并嵌入 | `pnpm build:admin`（仓库根） | 产物复制到 `api/internal/adminui/dist/`，之后重新 `go run`/`go build` |
| 编译二进制 | `go build -o bin/blog-server ./cmd/server` | `api/bin/` 已 gitignore |

接口清单在 `api/openapi.yaml`，本地联调步骤在 `docs/runbooks/local-dev.md`。

### 上传一个资产的步骤

1. 后端在跑，打开 http://localhost:8080/admin/ 登录。
2. 媒体库选前缀：图片 `img/`、视频 `video/`、三维模型 `models/`、附件 `files/`、Unity WebGL 或其他整目录构建 `demos/`（需填 demo 名与版本，文件按相对路径放到 `demos/<name>/<version>/` 下）。
3. 拖入文件。后端校验扩展名白名单、MIME 与大小上限后签发上传地址，浏览器直接 PUT（本地是 Go 的 `/_local/upload/<token>`，生产是 COS 预签名 URL），完成后回传 sha256 入库。
4. 复制得到的 `/assets/<key>`，写进内容文件。key 由后端生成：`<prefix>/YYYY/MM/<slug>-<rand>.<ext>`，不要手改。
5. 图片入库前先转 WebP、最长边不超过 2000 像素（第 5.4 节）。

## 11. AI 操作指南

### 三种改内容的方式

| 方式 | 何时用 |
|---|---|
| 直接写 `content/` 下的文件 | 改正文、改字段、批量迁移、任何"看得见文件"的改动。写完跑 `pnpm -C site build` |
| MCP 工具（`.mcp.json` 的 `blog` 服务器） | 在 Claude Code 里端到端完成"写文章 → 传图 → 校验 → 触发构建"。工具：`list_posts`、`get_post`、`create_post`、`update_post`、`create_work`、`update_work`、`upload_asset`、`list_assets`、`validate_content`、`trigger_build`、`build_status`、`get_stats` |
| 后台 http://localhost:8080/admin/ | 人手上传媒体、看构建日志与统计 |

三者写出的文件格式一致，front matter 都由 `content/schema/*.schema.json`（从 `site/src/content/schemas.ts` 导出）校验。改 schema 只改 `schemas.ts`，再跑 `pnpm -C site schema`。详细对应关系与排错见 `docs/ai-operations.md`。

### Keystatic 可视化编辑（仅开发模式）

`pnpm -C site dev` 后打开 http://localhost:4321/keystatic 。配置在 `site/keystatic.config.ts`，存储是 local，根目录是仓库（不是 `site/`）。生产构建不含它。

| 用 Keystatic | 用 MCP 或直接写文件 |
|---|---|
| 改文章、页面的标题、摘要、日期、标签、分类、草稿、封面路径、正文 | 作品的 `links`、`gallery`、`demo`（在 Keystatic 里是只读的 ignored 字段） |
| 新建文章：slug 要写成 `2026/my-post`（含年份目录） | 批量修改、迁移、任何需要精确控制 front matter 写法的场合 |
| 友链 | 上传资产（Keystatic 只填 `/assets/` 路径，不上传） |

Keystatic 保存会规范化 front matter（字段顺序、把可选布尔与默认值写全、数组用块样式）；已知差异记录在 `docs/decisions/0002-keystatic-roundtrip.md`。

### 写文章的检查清单

1. `description` 必须写，一两句话，会出现在列表、OG、RSS、llms.txt。
2. `tags` 优先复用已有的（`list_posts` 或 `/tags/` 页面），`categories` 一到两个。
3. 有公式就设 `math: true`；代码块写语言标识。
4. 图片先传后引：`upload_asset`（或后台）拿到 `/assets/img/YYYY/MM/<name>-<rand>.webp`，再写进正文；不要写还不存在的路径。
5. slug 让工具从标题生成（拼音或英文），需要时显式指定；一旦发布不再改。
6. 未完成的文章 `draft: true`。
7. 发布前 `validate_content`（或 `pnpm -C site check` + `build`），通过后再 `trigger_build`。

### 禁止事项

- 不改已有文章的 `legacyUrls`，不删除它们（会断 301）。
- 不删旧文章；下线用 `draft: true`。
- 不改已发布文章的 slug（文件名）。
- 不手写 `/assets/` 下的 key，不把大文件放进 Git。
- 不碰 `content/schema/*.json`，它们是生成文件。

### 站点常量与样式

- 站名、作者、导航、每页篇数、备案号、引言都在 `site/src/site.config.ts`。
- 颜色、字号、间距、圆角等设计变量全部在 `site/src/styles/tokens.css`；改风格只动这个文件。
- `robots.txt` 策略在 `site/src/robots.config.ts`。
- 旧站固定 301 映射在 `site/src/legacy-redirects.json`，构建后与文章的 `legacyUrls` 一起写进 `site/dist/redirects.map`。

### 新增一个作品

1. 先上传封面（`img/`）、图集图片与视频（`video/`），拿到 `/assets/...` 路径。
2. 在 `content/works/` 新建 `<slug>.mdx`，front matter 按第 5.2 节：`title`、`summary`、`period.from`（`YYYY-MM`）、`stack`、`cover`、`status` 必填；`featured: true` 上首页，`order` 小在前。
3. 有演示时填 `demo`：three.js 岛屿写 `{ kind: island, name: <注册名> }`，Unity WebGL 写 `{ kind: iframe, src: /assets/demos/<name>/<version>/index.html, aspect: 16/9 }`。
4. 图集填 `gallery`（`type: image|video`，`src`，视频加 `poster`）。
5. `links.post` 指向相关文章路径，`links.repo` 指向仓库。
6. 跑 `pnpm -C site build`；`/works/<slug>/` 与 `/works.json` 里应出现它。

### 新增一个 demo

1. 新建 `site/src/demos/<name>/index.vue`，资源在 `onMounted` 创建、`onBeforeUnmount` 全部释放（参考 `particles`）。
2. `site/src/demos/registry.ts` 加一行 `<name>: () => import('./<name>/index.vue')`。
3. 作品页用 `demo: { kind: island, name: <name> }`；文章里直接写 `<Demo name="<name>" />`（不用 import）。名字未注册时构建失败。
4. three.js 只能出现在 demo 的 chunk 里，不要在公共组件静态 import。细节与 Unity 构建导出设置见 `docs/demos.md`。

### 新增资产

阶段 1 尚无上传通道。约定已经生效：内容里写 `/assets/img/YYYY/MM/<name>.webp` 这样的路径，构建期自动改写为 CDN 地址。上传工具与 MCP 在后续阶段提供。
