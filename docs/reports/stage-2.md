# 阶段 2 报告：前台完整功能与视觉设计

日期：2026-09-08。提交：见 `git log --grep "stage 2"`。

## 做了什么

**设计系统**
- `site/src/styles/tokens.css`：全部颜色、字体栈、字号、行高、宽度、间距、圆角、阴影。颜色用 `light-dark()`，由 `<html>` 上的 `color-scheme` 决定深浅色：无 `data-theme` 跟随系统，`data-theme="light|dark"` 为手动切换，存 localStorage，`Base.astro` 里 `<head>` 最前面的内联脚本在首屏前应用，无闪烁。
- `site/src/styles/global.css`：所有样式只引用 tokens；Tailwind v4 通过 `@theme inline` 映射同一套变量。
- 中文字体栈按要求顺序；代码字体走系统等宽字体，不引入字体文件。单一强调色（青绿），其余灰阶，无动效。
- 身份元素：站名"木十的博客"、英文名 Mustenaka、页脚备案号链接到 beian.miit.gov.cn、莎士比亚引言（`site.config.ts` 里可改）。

**页面**：首页（简介、精选作品卡片、最新十篇）、`/posts/` 分页（每页 15，分类入口、标签与归档链接）、文章页（日期、更新日期、分类、标签、阅读时长、右侧悬浮目录、窄屏折叠到顶部、上一篇下一篇、系列列表、Artalk 岛屿壳）、`/tags/`、`/tags/<tag>/`、`/categories/`、`/categories/<cat>/`、`/archive/`、`/about/`、`/friends/`（读 `friends.yaml`）、`/search/`（Pagefind）、`/works/`（简版，阶段 5 定稿）、`/works/<slug>/`、404。

**内容渲染**：KaTeX 样式与 woff2 字体由 `site/scripts/copy-katex.mjs` 在 install/dev/build 时复制到 `site/public/katex/`（已 gitignore），只在 `math: true` 页面引入；Shiki 双主题跟随深浅色，语言标签用 CSS `attr(data-language)`，复制按钮；图片懒加载 + `<dialog>` 灯箱；表格由 rehype 插件包进 `.table-wrap` 横向滚动；外链自动 `target=_blank rel=noopener`。

**机器入口**：`/rss.xml`（全文，用 Astro Container API 渲染 MDX）、`/sitemap-index.xml`、`/robots.txt`（策略在 `robots.config.ts`，文件头注释说明如何禁 AI 爬虫）、`/llms.txt`、`/llms-full.txt`、`/posts/<slug>.md`（YAML 元数据头 + Markdown 正文，`/assets/` 改写为绝对 URL）、JSON-LD（BlogPosting / SoftwareSourceCode / Person + WebSite）、OG 与 Twitter meta、默认 OG 图 `site/public/og-default.png`。`PUBLIC_NOINDEX=1` 或 `true` 时全站 noindex 且 robots 全部 Disallow。

**301 映射**：`site/scripts/redirects.mjs` 在 `pnpm -C site build` 末尾生成 `dist/redirects.map`，来源是非草稿文章的 `legacyUrls` 加 `site/src/legacy-redirects.json`（即 AGENTS.md 5.5 的固定表）。

**工程**：`site/src/site.config.ts` 集中站点常量；跳转正文链接、`:focus-visible` 样式、色彩对比达标；`AGENTS.md` 追加"新增一篇文章的完整步骤""新增一个页面的步骤""站点常量与样式"。

## 怎么验证的

| 验收项 | 结果 |
|---|---|
| `pnpm -C site check` | 44 files, 0 errors, 0 warnings, 0 hints |
| `pnpm -C site build` | 23 页面 + Pagefind 索引 6 页 + `redirects.map` 8 条 |
| 三篇示例文章渲染 | 公式（`.katex` 4 处）、三种语言高亮（`data-language` csharp/go/ts）、图片改写、岛屿计数 10→12 均正常（headless Chrome 实测） |
| 深浅色切换 | 点击后 `data-theme=dark` 写入 localStorage，刷新后保持；`DOMContentLoaded` 时已是 dark，无闪烁 |
| `/search/` 中文 | Pagefind 搜"公式"命中《公式与代码块》，搜"岛屿"命中《岛屿示例》 |
| `curl` 五个入口 | `rss.xml` 3 条 item 含 `content:encoded` 全文；`llms.txt` 按分类分组；`llms-full.txt` 3 篇按日期倒序、80 个等号分隔；`hello-astro.md` 有 YAML 头且 `/assets/` 已改成绝对 URL；`robots.txt` 允许全部并带 Sitemap |
| `legacyUrls` → `redirects.map` | 给 `math-and-code.mdx` 加一条后，map 里出现 `/index.php/2026/09/02/math-and-code/ /posts/math-and-code/;`，固定五条（含 `/feed/`）与 410 都在 |
| `PUBLIC_NOINDEX=1` | 首页与文章页均有 `<meta name="robots" content="noindex, nofollow">`，robots 为 `Disallow: /`（验证后已还原 `.env`） |
| 移动端视口 375px | 首页、两篇文章、作品页、列表页 `scrollWidth == innerWidth == 375`，无横向滚动 |
| Lighthouse（移动端，文章页，预览服务器） | 性能 100，可访问性 100，最佳实践 100，SEO 100；FCP 0.9s，LCP 1.1s，TBT 50ms，CLS 0.002 |
| 文章页 JS | 无外部 `<script src>`；只有内联脚本（主题初始化、导航、复制/灯箱/目录高亮）与岛屿运行时 |
| 截图 | `docs/reports/stage-2-screenshots/{home,post,work}-{light,dark}.png` |

Lighthouse 第一次跑出可访问性 95，原因是 headless Chrome 跟随系统进入深色，而 Shiki 内联的浅色 token 颜色压过了样式表，深色下代码是浅色配色。已改为 `!important` 覆盖，并把 `--color-text-faint` 加深、去掉复制按钮的半透明，第二次 100。

## 没做或有疑问的

- **正文宽度**取字面 72 个汉字（`--measure: 72em`，约 1224px）。桌面上偏宽，改 `tokens.css` 这一个变量即可。
- **"零 JS"** 的理解：文章页没有任何外部 JS bundle，但主题、菜单、复制按钮、灯箱、目录高亮需要约 3KB 内联脚本，无法用纯 CSS 完成且不影响性能预算。
- **Artalk** 只做了壳：`PUBLIC_ARTALK_SERVER` 为空时整个岛屿不渲染；非空时动态加载服务器上的 `Artalk.js`。真实联调留到部署阶段。
- **RSS 中的岛屿**：Container API 渲染 Vue 岛屿会带上 hydration 脚本，已在 feed 里剥掉 `<script>`/`<style>`，只留静态 HTML。`loadRenderers()` 在预渲染 worker 里加载不了 Vue 的虚拟模块，改为静态导入 `@astrojs/vue/server.js` 与 `@astrojs/mdx/server.js`。
- **410 的表示**：`redirects.map` 里值为 `410`，Nginx 端需 `if ($legacy_redirect = 410) { return 410; }`，文件头有注释，阶段 7 落地。
- **标签/分类 URL** 直接用中文（`/tags/博客/`），产物目录名是中文，Nginx 与浏览器都按 UTF-8 处理；如需拉丁 slug，改 `tagUrl`/`categoryUrl` 与 `getStaticPaths` 即可。
- **Astro 7 的 `astro dev` / `astro preview` 会转成守护进程**，用 `pnpm exec astro preview stop` 停，本阶段验证完已停，端口 4321 已释放。
- Lighthouse 只测了文章页（要求如此）；首页未测。
- 阶段 1 已知：`/assets/` 在没有后端时 502，作品封面在截图里是灰色占位，阶段 3 上传真实图片后消失。
- 视觉待你审阅。

## 新增依赖及理由

- `pagefind` 1.5.2（site devDependency）：宪法第 3 节已列，用于构建后生成搜索索引与前端 UI。
- 仓库之外、只在临时目录用于验证：`puppeteer-core` 25.10.0（驱动本机 Chrome 截图、生成 OG 图、做交互测试）、`lighthouse` 13.4.1（通过 `npx` 运行）。它们不在任何 `package.json` 里。
