# 阶段 5 报告：作品集与交互 demo

日期：2026-09-08。提交：见 `git log --grep "stage 5"`。

## 做了什么

**页面**
- `/works/`：卡片网格（封面、标题、摘要、技术栈标签、状态角标），按 `order` 再按 `period.from` 倒序（阶段 2 的 `WorkCard` 与 `getWorks` 已满足，本阶段封面换成真实上传的图片）。
- `/works/<slug>/`：头部信息区（时间、角色、状态、技术栈、仓库/演示/相关文章链接）、demo 区（`island` 走 `<Demo>`，`iframe` 走 `<UnityEmbed>`，无 demo 时显示封面）、正文、`<Gallery>`。
- 首页精选作品卡片用同一个 `WorkCard` 组件，封面现在是真实图片。
- `/works.json`：全部作品的 front matter（`/assets/` 已改成绝对 URL）加正文纯文本摘要（去 MDX 语法，400 字）。

**demo 组件体系**（`site/src/components/`）
- `demos/registry.ts`：名字 → 异步 Vue 组件加载器；`demos/DemoIsland.vue` 是唯一的宿主岛屿，用 `defineAsyncComponent` 按名字动态 import，因此每个 demo 及其依赖是独立 chunk。
- `<Demo name caption />`（Astro）：构建期查注册表，未注册的名字直接抛错，错误列出已注册名字；以 `client:visible` 挂载岛屿。MDX 与作品页都能用。
- `<UnityEmbed src aspect sizeHint title caption />`：`UnityFrame.vue` 岛屿，点击前不创建 iframe，按钮显示体积提示；iframe `allow="fullscreen; autoplay; xr-spatial-tracking"` + `allowfullscreen`，右下角全屏按钮（`requestFullscreen`）。`src` 必须是 `/assets/demos/...`。
- `<Video src webm poster caption loop muted autoplay />`：`preload="none"`，海报图，WebM 与 MP4 双源，可在 MDX 使用。
- `<Gallery items />`：渲染 works 的 `gallery`，图片进现有灯箱，视频复用 `<Video>`。

**样板**
- three.js 岛屿 `site/src/demos/particles/index.vue`：600 个（100–2000 可调）粒子在盒子里受重力（0–20 可调）下落、碰壁反弹、带涡流；`onBeforeUnmount` 释放几何体、材质、渲染器、canvas、ResizeObserver、IntersectionObserver、RAF；离开视口暂停渲染，回来继续，不重建上下文。`adaptor-physx.mdx` 的 `demo` 指向它。
- Unity 嵌入样板：`demos/unity-mock/1.0.0/index.html` + `Build/mock.framework.js.br`（Node `brotliCompressSync` 压缩，1141 → 522 字节），通过后台接口的 demos/ 路径上传（`demoName`、`demoVersion`、`relativePath`），新作品 `unity-webgl-mock.mdx` 用 `demo: { kind: iframe }` 嵌入。
- 视频样板：ffmpeg 生成 6 秒 1280×720 H.264 MP4（1.2 MB）与 VP9 WebM（345 KB）、海报 WebP，上传到 `video/2026/09/` 与 `img/2026/09/`，在作品 `adaptor-physx` 的图集和新文章《在文章里嵌入可交互 demo 与视频》里各嵌一次。该文章同时用了 `<Demo>` 与 `<Video>`。

**文档**：`docs/demos.md`（新增 three.js demo 的步骤、Unity WebGL 导出设置与上传步骤、响应头验证、`crossOriginIsolated` 说明、视频转码命令）；`AGENTS.md` 的"新增一个作品"改写、追加"新增一个 demo"。

## 怎么验证的

| 验收项 | 结果 |
|---|---|
| `pnpm -C site check` / `build` | 49 files 0 errors；28 页面，Pagefind 8 页 |
| `/works/` 与 `/works/adaptor-physx/` | 正常渲染；粒子 demo 可交互：滑块把粒子数改到 1500 后 `output` 同步；`data-testid=running` 显示"运行中" |
| 滚出视口再滚回 | 拦截 `HTMLCanvasElement.getContext`：可见时 1 个 canvas、1 个 WebGL 上下文；滚到页底后状态变"已暂停"；滚回后"运行中"，上下文仍是 1，canvas 仍是 1 |
| 文章同时用 `<Demo>` 与 `<Video>` | `/posts/demo-showcase/` 构建通过；页面有 1 个 demo 岛屿，`<video preload="none">` 带海报，两个 `<source>`（webm、mp4） |
| `<Demo name="not-exist" />` | 临时文章构建失败：`Error: <Demo name="not-exist" />: unknown demo. Registered demos: particles (see site/src/demos/registry.ts)`，退出码 1；删除后重建通过，dist 无残留 |
| three.js 不进文章页与首页 | `WebGLRenderer` 只出现在 `particles.CS1LS20s.js`；没有任何 HTML 直接引用该 chunk，只有 `DemoIsland.*.js` 通过动态 import 引用它；无头浏览器实测首页 0 个 JS 请求，文章页只请求 Vue 运行时与 Counters 岛屿 |
| 模拟页在 iframe 中运行 | 点击前 0 个 iframe；点击后 iframe `src` 为 `…/assets/demos/unity-mock/1.0.0/index.html`，页内状态文本 "mock framework loaded via .br"，说明 `.br` 脚本被浏览器解压执行 |
| 响应头 | `index.html`：`text/html; charset=utf-8` + COOP `same-origin` + COEP `require-corp` + CORP `cross-origin`；`mock.framework.js.br`：`Content-Encoding: br`、`Content-Type: text/javascript`、`Vary: Accept-Encoding`、COOP/COEP；视频 `video/mp4` 且 `Accept-Ranges: bytes` |
| `/works.json` | 2 个作品，字段齐全（slug、url、全部 front matter、excerpt），`cover`/`gallery.src`/`demo.src` 为绝对 URL |
| 进程清理 | Go 后端与 preview 均已停止，8080 与 4321 释放 |

**各页面 JS 体积**（构建产物，未压缩；"首屏"指页面 HTML 直接引用的脚本及其静态 import 闭包）：

| 页面 | 首屏 JS | 说明 |
|---|---|---|
| `/` | 0 KB | 无岛屿 |
| `/works/` | 0 KB | 无岛屿 |
| `/posts/hello-astro/`、`/posts/math-and-code/` | 75.1 KB（5 个文件） | Vue 运行时 + Counters 岛屿 |
| `/posts/island-demo/` | 75.7 KB | 多一个 Counter 岛屿 |
| `/posts/demo-showcase/` | 77.5 KB | 多 DemoIsland 宿主；three.js 不在首屏 |
| `/works/adaptor-physx/` | 75.5 KB | 同上 |
| `/works/unity-webgl-mock/` | 74.5 KB | UnityFrame 岛屿 |
| 按需加载 | `particles.CS1LS20s.js` 510 KB（含 three.js，gzip 后约 130 KB） | 仅在粒子 demo 进入视口后下载 |

## 没做或有疑问的

- **Unity 真实构建验证待补**：没有 Unity WebGL 构建目录，用模拟页走通了上传、嵌入与响应头。拿到真实构建后按 `docs/demos.md` 上传并把 `demo.src` 指向它即可。
- **`crossOriginIsolated=false`**：iframe 内是否隔离取决于顶层页面。默认单线程 Unity 构建不需要；多线程需要给 `/works/` 页面加 COOP/COEP，见 `docs/demos.md`。
- **视频 gallery 只有单源**：`gallery` 的 schema 只有 `src`（MP4）与 `poster`，没有 `webm` 字段；文章里用 `<Video>` 可给双源。如需图集双源，加字段要改 `schemas.ts`。
- **Vue 运行时是文章页 75 KB 的主体**：来自 Counters 岛屿（阶段 3 需求）。如果要把文章页拉回接近零 JS，可把 Counters 改成不依赖 Vue 的内联脚本，留待需要时再做。
- 上传的样板资产（封面、视频、模拟页）在本地 `api/data/`，不在仓库；生产要重新上传后改路径。封面是 ffmpeg 合成图，待换真实截图。
- 三个 AI 生成的 MDX 里 `<Demo>`/`<Video>` 需要显式 import；没有做全局注入（Astro MDX 支持 `components` 导出但会把组件塞进每篇文章的 chunk，违背按需原则）。

## 新增依赖及理由

- `three` 0.185.1：宪法指定。
- `@types/three` 0.185.4（devDependency）：three 不自带类型定义，`astro check` 需要它才能检查 `index.vue` 里的 three 调用。
