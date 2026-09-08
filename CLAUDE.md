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
| 前台 | `site/` | Astro 5，构建期生成静态 HTML，Nginx 直出。交互用 Vue 3 岛屿 | 不做服务端渲染，不在运行时读数据库 |
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
- Astro 5 最新稳定版，TypeScript strict。
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
mustenaka-site/
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
