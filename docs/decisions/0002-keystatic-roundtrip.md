# 0002 Keystatic 的 round-trip 差异与使用边界

日期：2026-09-08。状态：已采纳。

Keystatic（`site/keystatic.config.ts`）只在 `astro dev` 下启用，存储 local，根目录是仓库。用它打开手写文章并保存时，文件会被它的序列化器重写。本文记录实测差异与由此定下的使用边界。

## 实测 1：不改任何内容直接保存

Keystatic 在表单数据与文件数据相同时不发出保存请求（Save 可点但无写入），`git diff` 为空。

## 实测 2：改一个字段再改回，触发真正的重写

`content/posts/2026/hello-astro.mdx`（手写）经 Keystatic 重写后的完整 diff：

```diff
-tags: [astro, 博客]
-categories: [站点建设]
+tags:
+  - astro
+  - 博客
+categories:
+  - 站点建设
+draft: false
 cover: /assets/img/2026/09/sample-802972.webp
+math: false
 legacyUrls:
   - /index.php/2026/09/01/hello-astro/
+toc: true
 ---
-
 这是一篇普通文章…
-- 第一项
-- 第二项
-  - 嵌套项
-- 第三项
+* 第一项
+* 第二项
+  * 嵌套项
+* 第三项
-| 字段 | 类型 | 必填 |
-| --- | --- | --- |
-| title | string | 是 |
+| 字段    | 类型      | 必填 |
+| ----- | ------- | -- |
+| title | string  | 是  |
-![示例图片](/assets/img/2026/09/sample-802972.webp)
+!\[示例图片]\(/assets/img/2026/09/sample-802972.webp)
```

`content/posts/2026/demo-showcase.mdx`（含 `<Demo>` 与 `<Video>`）：

```diff
-tags: [three.js, demo, 视频]
+tags:
+  - three.js
+  - demo
+  - 视频
+draft: false
+math: false
+legacyUrls: []
+toc: true
-<Video
-  src="/assets/video/2026/09/particles-demo-a4ae0b.mp4"
-  webm="/assets/video/2026/09/particles-demo-7d069b.webm"
-  poster="/assets/img/2026/09/particles-demo-poster-e5af08.webp"
-  caption="ffmpeg 生成的六秒测试片段。"
-/>
+<Video src="…mp4" webm="…webm" poster="…webp" caption="ffmpeg 生成的六秒测试片段。" />
```

`<Demo name="particles" caption="…" />` 一字未变。

## 差异分类

| 差异 | 性质 | 处理 |
|---|---|---|
| 数组从流式 `[a, b]` 变块式 | 无害 | 接受 |
| 补写 `draft: false`、`math: false`、`toc: true`、`legacyUrls: []` | 无害（与 schema 默认值一致） | 接受 |
| `---` 后的空行被删 | 无害 | 接受 |
| 列表 `-` 变 `*`、表格重新对齐 | 无害 | 接受 |
| 多行 JSX 组件折成一行 | 无害 | 接受 |
| **Markdown 图片 `![alt](/assets/…)` 被转义成纯文本 `!\[alt]\(…)`** | **破坏性**：图片不再渲染 | 不接受。原因是编辑器只认仓库内的本地图片文件，`/assets/` 在 CDN 上。试过 `image: false` 与 `image: { directory: 'api/data/assets', publicPath: '/assets/' }`，都一样。最终配置 `image: false`（最保守）。 |
| **含 LaTeX 花括号的文章打不开**（`Could not parse expression with acorn`） | 无法编辑 | Keystatic 的 MDX 解析器没有 remark-math，把 `\mathbf{r}` 的 `{}` 当 JSX 表达式。无解。 |
| 手写 HTML `<img>` | 需在 `mdxComponents` 里声明 `img` 才能打开 | 已声明 |
| 空的可选文本字段（`cover`、`series`、`lang`） | 不写入（好） | 接受 |

## 由此定下的边界

- **Keystatic 适合**：没有 Markdown 图片、没有公式的文章与页面；作品的标量字段；友链；新建草稿。
- **Keystatic 不适合**：含 `![]()` 图片的文章（保存会破坏图片）、`math: true` 的文章（打不开）、作品的 `links`/`gallery`/`demo`（在 Keystatic 里是 `ignored`，只读保留）。这些用 MCP 工具或直接写文件。
- 要在 Keystatic 里维护的文章，图片用 `<img src="/assets/…" alt="…" />` 写法（编辑器把它当 block 组件，原样保留）。
- posts 的路径规则 `content/posts/**`：slug 必须写成 `2026/my-post`（含年份目录），Keystatic 不会按日期自动分目录。
- 不要用编辑器的图片按钮上传（它会把文件写进仓库）；上传走后台或 MCP。
- 生产构建不含 Keystatic：集成只在 `command === 'dev'`（或 `KEYSTATIC=1`）时注入路由与 React。
