# 阶段 6 报告：Keystatic 本地可视化后台

日期：2026-09-08。提交：见 `git log --grep "stage 6"`。

## 做了什么

**集成方式**：没有直接用 `@keystatic/astro` 的 `keystatic()` 集成，因为它注入的 API 路由不能传 `localBaseDirectory`，内容根会被固定在 `site/`。改为自写 `site/integrations/keystatic-dev.ts`：
- 只在 `command === 'dev'`（或 `KEYSTATIC=1`；`KEYSTATIC=0` 可关）时生效，其余命令直接返回，因此 `pnpm -C site build` 的产物里没有任何 Keystatic 路由与代码，React 集成也只在此时通过 `updateConfig({ integrations: [react()] })` 加入。
- 复用官方的 UI 页面入口 `@keystatic/astro/internal/keystatic-astro-page.astro`（`/keystatic/[...params]`），API 路由指向自己的 `site/src/keystatic/api.ts`：`makeHandler({ config, localBaseDirectory: <仓库根> })`（字段名从 `@keystatic/core/dist/declarations/src/api/generic.d.ts` 确认）。
- 提供 `virtual:keystatic-config` 虚拟模块与 `.astro/keystatic-imports.js`（与官方集成一致）。
- dev 下把 `trailingSlash` 放宽为 `ignore`：Keystatic 的 UI 请求 `/api/keystatic/tree` 不带斜杠，生产的 `always` 会 404。

**`site/keystatic.config.ts`**：`storage: { kind: 'local' }`；posts `content/posts/**`（slug 含年份目录，如 `2026/my-post`，有正则校验）、works `content/works/*`、pages `content/pages/*`、friends 单例 `content/data/friends`（yaml）。字段与 AGENTS.md 第 5 节一一对应；资产字段是带 `/assets/` 前缀校验的文本字段；works 的 `links`、`gallery`、`demo` 用 `fields.ignored()`（保存时原样保留，不可编辑，因为 Keystatic 的 conditional/object 序列化形状与我们的 discriminatedUnion 与可选嵌套字段不一致）。正文用 `fields.mdx`（`extension: 'mdx'`），声明 `Demo`、`Video`、`UnityEmbed`、`Counter` 与 `img` 五个 block 组件，`image: false`。

**为 Keystatic 做的两处内容层调整**：
- 新增 `site/src/plugins/remark-auto-import.ts`：MDX 里用到 `Demo`/`Video`/`UnityEmbed`/`Counter` 时构建期自动注入 import，内容文件不再写 `import` 行（Keystatic 的 MDX 字段不能表示 ESM 语句）。`Counter` 改为 `Counter.astro` 包装（`client:visible` 在包装里），MDX 写 `<Counter start={10} />`。示例内容已改，AGENTS.md 对应步骤已改。
- `content/data/friends.yaml` 改为顶层 `friends:` 键（Keystatic 单例只能写对象），Astro 的 file loader 用 `parseFrontmatter` 取出数组，无新依赖。

**文档**：`docs/decisions/0002-keystatic-roundtrip.md`（完整 diff 与使用边界）、`docs/runbooks/local-dev.md` 新增 2b 节、`AGENTS.md` 第 11 节新增"Keystatic 可视化编辑"表。

## 怎么验证的

| 验收项 | 结果 |
|---|---|
| 开发模式打开 `/keystatic` | Dashboard 列出：文章 4、作品 2、固定页面 2、友链 |
| 无头浏览器新建文章与作品 | `content/posts/2026/keystatic-created.mdx`、`content/works/keystatic-work.mdx` 生成，front matter 通过 schema（见报告下方），`pnpm -C site build` 30 页面通过，`dist/posts/keystatic-created/`、`dist/works/keystatic-work/` 含标题。验证后删除这两个测试文件 |
| 编辑示例文章并保存 | 不改内容保存：Keystatic 不发请求，diff 为空。改一个字段再改回（触发重写）：hello-astro 与 demo-showcase 的完整 diff 记录在 `docs/decisions/0002-keystatic-roundtrip.md`。`<Demo>` 原样，`<Video>` 多行折成一行，`<img>` 原样 |
| 生产构建不含 Keystatic | `grep -ri keystatic site/dist` 0 个文件，`_astro/*.js` 中无 react，无 `dist/keystatic`、`dist/api` 目录 |
| 构建时间 | 阶段 5：28 页 3.4–4.6 s；本阶段：28 页 2.8 s（整条 `pnpm -C site build` 含 Pagefind 与 redirects 约 7 s），无明显变化 |
| `pnpm -C site check` / `build` | 54 files 0 errors；28 页面 |
| 进程清理 | `astro dev stop`，4321 已释放 |

Keystatic 生成的文章文件（原样）：

```yaml
---
title: Keystatic 新建的文章
description: 通过 Keystatic 可视化后台创建的测试文章。
date: 2026-09-08
tags: []
categories: []
draft: false
math: false
lang: zh-CN
legacyUrls: []
toc: true
---
这是在 Keystatic 编辑器里输入的正文。
```

## 没做或有疑问的

- **两个不可接受的差异**（详见 decisions/0002）：Markdown 图片 `![alt](/assets/…)` 会被转义成纯文本（试了 `image: false` 与指向本地存储目录的 `image` 配置，都一样，原因是编辑器只认仓库内的本地图片文件）；含 LaTeX 花括号的文章打不开（Keystatic 的 MDX 解析器无 remark-math）。因此 Keystatic 的适用范围是：无图片、无公式的文章与页面，作品的标量字段，友链，新建草稿。含图片的文章要在 Keystatic 里维护的话用 `<img>` 写法。
- **posts 按年份分目录**：Keystatic 不支持按 `date` 自动分目录，采用 `content/posts/**` 加"slug 含年份目录"的折中（`2026/my-post`），有正则校验。Astro 侧 slug 仍是文件名，URL 不变。
- **序列化风格差异**（数组块式、补默认布尔、`-` 变 `*`、表格对齐）无害但会让 diff 变大；MCP 与手写保持流式数组风格。三者 schema 一致，构建都通过。
- **`fields.ignored()` 的字段**（`links`、`gallery`、`demo`）在 Keystatic 里看不到也改不了，只保留。
- 修过程中的三个环境问题：pnpm 对 `tslib` 只更新了 lockfile 没有链接（`pnpm install --force` 解决）；Astro 7 的 dev 守护进程在配置改动后需要显式 `astro dev stop` 再启动；`process.argv.includes('dev')` 在守护进程里不可靠，改用集成钩子的 `command`。
- Keystatic 的 UI 自动化只覆盖了标量字段与正文输入，数组字段（标签、分类）没有通过 UI 操作，新建的文章 `tags: []`。

## 新增依赖及理由

全部是 `site/` 的 devDependencies，只在 dev 生效：
- `@keystatic/core` 0.6.9、`@keystatic/astro` 6.0.0：宪法指定。
- `@astrojs/react` 6.0.5、`react` 19.2.8、`react-dom` 19.2.8、`@types/react`、`@types/react-dom`：Keystatic 的 UI 是 React，`client:only="react"` 需要 React 渲染器；生产构建不包含。
- `tslib` 2.8.1：Keystatic 的依赖（`@urql/*`、`@floating-ui/react` 等）在 Vite 预打包后 import `tslib`，pnpm 隔离模式下从 `site/` 解析不到，显式声明。
