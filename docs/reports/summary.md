# 阶段 2–6 汇总

日期：2026-09-08。仓库 https://github.com/Mustenaka/astro-go-blog ，每个阶段一个提交（`git log --grep "stage N"`），各阶段详细报告在 `docs/reports/stage-N.md`。

## 各阶段要点

| 阶段 | 做了什么 | 关键验证 |
|---|---|---|
| 2 前台 | `tokens.css` 单文件设计变量、深浅色无闪烁、全部页面（分页、标签、分类、归档、about/friends、Pagefind 搜索、404）、自托管 KaTeX、Shiki 双主题 + 复制、灯箱、表格滚动、RSS 全文、robots 配置文件、llms.txt/llms-full.txt、`/posts/<slug>.md`、JSON-LD/OG、`redirects.map`、`PUBLIC_NOINDEX` | Lighthouse 移动端文章页 100/100/100/100；中文搜索命中；移动端无横向滚动；截图在 `docs/reports/stage-2-screenshots/` |
| 3 后端 | Go 1.25 + SQLite（WAL、embed 迁移）、argon2id、会话、限流、安全头、`Storage` 接口（local 完整模拟 + COS 预签名）、公开计数/点赞、管理接口、GitHub webhook、构建互斥排队、Vue 后台 embed、前台 Counters 岛屿 | `go test` 11 个 httptest 用例；无头浏览器走通登录→上传→文章可见→点赞每天一次→后台构建成功；webhook 401/202 |
| 4 MCP | zod schema 单点定义并导出 JSON Schema 到 `content/schema/`，Go 侧同一份校验；front matter round-trip 无 diff；拼音 slug；WebP 转换；12 个 MCP 工具（stdio），`.mcp.json` | Go SDK 客户端经 stdio 端到端：建文章→传图→插图→校验→构建→状态；未知字段按名拒绝；每个工具的参数校验有测试 |
| 5 作品与 demo | `/works/`、`/works/<slug>/`、`/works.json`；`<Demo>`/`<UnityEmbed>`/`<Video>`/`<Gallery>`；three.js 粒子岛屿（释放资源、离屏暂停）；Brotli 模拟 Unity 页；ffmpeg 视频 | three.js 只在 demo chunk，首页 0 JS；WebGL 上下文滚出滚回仍为 1；未注册 demo 构建失败并列出名字；`.br` 与 COOP/COEP 响应头正确 |
| 6 Keystatic | dev-only 集成（自写，`localBaseDirectory` 指仓库根）、四个集合映射、`remark-auto-import` 让 MDX 不写 import、round-trip diff 记录 | UI 新建文章与作品→构建通过；生产产物 0 处 keystatic/react；构建时间无变化 |

## 所有新增依赖（宪法第 3 节之外，或需要说明的）

前端 `site/`：
- 生产依赖：`@astrojs/markdown-remark`（Astro 7 把 remark 管线拆成可选包，公式插件需要它）、`pagefind`、`three`（宪法指定）。
- 开发依赖：`vite`、`@types/node`（`astro.config.ts` 用 `loadEnv` 与 `node:url`）、`@types/three`、`@keystatic/core`、`@keystatic/astro`（宪法指定）、`@astrojs/react`、`react`、`react-dom`、`@types/react`、`@types/react-dom`（Keystatic UI 需要，仅 dev）、`tslib`（Keystatic 依赖在 pnpm 隔离模式下的解析补丁）。

后台 `admin/`：`vue`、`vue-router`、`vite`、`@vitejs/plugin-vue`、`vue-tsc`、`typescript`。

Go `api/`：`modernc.org/sqlite`、`golang.org/x/crypto`（固定 v0.55.0 以留在 go 1.25）、`cos-go-sdk-v5`、`godotenv`、`modelcontextprotocol/go-sdk`（以上宪法指定）；`santhosh-tekuri/jsonschema/v6`（按导出的 JSON Schema 校验）、`gopkg.in/yaml.v3`（保序读写 front matter）、`mozillazg/go-pinyin`（中文 slug）、`golang.org/x/image`（缩放）、`gen2brain/webp`（纯 Go WebP 编码，wazero 跑 libwebp）、`golang.org/x/text`（jsonschema 错误信息打印）。

仓库外、只在临时目录用于验证：`puppeteer-core`、`lighthouse`。

## 待你处理的事项

1. **视觉审阅**（阶段 2）：截图在 `docs/reports/stage-2-screenshots/`；所有设计变量在 `site/src/styles/tokens.css`。特别是正文宽度取了字面 72 个汉字（`--measure: 72em`），桌面偏宽，改一个变量即可。
2. **`/mcp` 人工验证**（阶段 4）：本会话无法重载自己的 MCP 配置。下次会话打开仓库后在 `/mcp` 里调 `list_posts`，再 `create_post` 一篇草稿。需要 `api/.env` 存在（`ADMIN_PASSWORD_HASH`、`SESSION_SECRET` 非空）。
3. **Unity 真实构建验证**（阶段 5）：提供 Unity WebGL 构建目录后，按 `docs/demos.md` 用后台 `demos/` 前缀上传，把 `content/works/unity-webgl-mock.mdx`（或新作品）的 `demo.src` 指向它。多线程构建需要给 `/works/` 页面加 COOP/COEP（见 `docs/demos.md`）。
4. **Astro 版本**：宪法写 Astro 5，实际 7.3.1（阶段 1 已由你拍板），AGENTS.md 已同步。
5. **Keystatic 的边界**（阶段 6）：含 Markdown 图片或公式的文章不要用 Keystatic 编辑（前者保存会破坏图片，后者打不开），详见 `docs/decisions/0002-keystatic-roundtrip.md`。若接受不了，可考虑弃用 Keystatic 只保留 MCP 与后台。
6. **COS 真实上传**（阶段 3）：`cos` 适配器只有离线签名测试，真实凭据验证留阶段 7。
7. **阶段 3 的实现取舍**：浏览计数的十分钟去重在内存（重启清零）；MCP 与 HTTP 各有一个构建 runner，不互斥。
8. **示例资产**：封面、视频、模拟页只在本地 `api/data/`，不在仓库；生产要重新上传并改路径。封面是 ffmpeg 合成图，待换真实截图。
9. **本机残留**：`api/.env`（含开发密码哈希 `dev-password-1234` 与随机 `SESSION_SECRET`）、`api/data/`、`api/bin/`、`site/.env` 均已 gitignore；临时目录里的 puppeteer/lighthouse 可删。
10. **阶段 7 起的事**：Nginx（`redirects.map` 的 `410` 约定、`/assets/` 302 兜底、demos 响应头）、systemd、发布脚本、Artalk、WordPress 迁移脚本（`scripts/wp-migrate/` 尚未开始）。
