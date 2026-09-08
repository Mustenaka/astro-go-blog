# demo 体系

作品页与文章里有三类演示方式，共用一套组件（都在 `site/src/components/`）：

| 组件 | 用途 | 加载策略 |
|---|---|---|
| `<Demo name="..." />` | three.js 等交互岛屿 | `client:visible`，demo 代码与 three.js 是独立 chunk，只在该 demo 可见时下载 |
| `<UnityEmbed src="/assets/demos/<name>/<version>/index.html" aspect="16/9" sizeHint="25 MB" />` | Unity WebGL 或任何托管的 HTML | 点击后才创建 iframe；有全屏按钮 |
| `<Video src webm poster caption />` | 一两分钟内的短片 | `preload="none"`，海报图，MP4 + WebM 双源 |
| `<Gallery items={...} />` | 作品的 `gallery` 字段 | 图片进灯箱，视频走 `<Video>` |

作品的 `demo` 字段决定作品页头部显示哪种：`{ kind: island, name }` 或 `{ kind: iframe, src, aspect? }`。MDX 里显式 `import Demo from '@components/Demo.astro'` 后使用。

## 新增一个 three.js demo

1. 新建 `site/src/demos/<name>/index.vue`。在 `onMounted` 里创建渲染器、场景、几何体、材质；在 `onBeforeUnmount` 里全部 `dispose()`、移除 canvas、断开监听、取消 RAF。参考 `site/src/demos/particles/index.vue`，它还用 `IntersectionObserver` 在离开视口时暂停渲染。
2. 在 `site/src/demos/registry.ts` 加一行：`<name>: () => import('./<name>/index.vue')`。
3. 用 `<Demo name="<name>" />`。名字写错构建会失败并列出已注册的名字。
4. 跑 `pnpm -C site build`，在 `site/dist/_astro/` 里确认 three 只出现在 demo 的 chunk：`grep -l "three" dist/_astro/*.js`。

规则：three.js 只在作品页与用到它的文章里出现，不进首页；不要在 `DemoIsland.vue` 或任何公共组件里静态 import three。

## Unity WebGL 构建

导出设置（Player Settings → Publishing Settings）：

- Compression Format：**Brotli**。
- Decompression Fallback：**关闭**（服务端已正确返回 `Content-Encoding: br`，开启会绕过压缩且更大）。
- Data Caching：**开启**。
- 如需多线程或 SharedArrayBuffer，服务端 `demos/` 前缀返回 COOP `same-origin` 与 COEP `require-corp`（本地 Go 与 Nginx 规则都已包含）。

上传：

1. 在后台媒体库选前缀 `demos/`，填 demo 名（小写字母数字连字符，如 `fluid-demo`）与版本（如 `1.0.0`），把导出目录整个拖入（浏览器会带相对路径）；或用 MCP `upload_asset`，每个文件传 `prefix: demos`、`demoName`、`demoVersion`、`relativePath`。
2. 结果是 `/assets/demos/<name>/<version>/index.html` 与 `Build/*.br`。
3. 作品 front matter 写：

```yaml
demo:
  kind: iframe
  src: /assets/demos/<name>/<version>/index.html
  aspect: 16/9
```

4. 换版本时上传到新的 `<version>` 目录再改 `src`，旧版本保留一段时间后在后台删除。

响应头验证（本地）：

```bash
curl -sI http://localhost:8080/assets/demos/<name>/<version>/Build/xxx.wasm.br
```

应看到 `Content-Type: application/wasm`、`Content-Encoding: br`、`Cross-Origin-Opener-Policy: same-origin`、`Cross-Origin-Embedder-Policy: require-corp`。

关于 `crossOriginIsolated`：iframe 里的页面只有在顶层页面（作品页本身）也带 COOP/COEP 时才会进入隔离状态。默认的 Unity WebGL 构建是单线程，不需要隔离，现状可用；如果启用了多线程（SharedArrayBuffer），需要在 Nginx 给 `/works/` 页面也加这两个头，而且页面里所有跨域资源（评论、图片、CDN）都必须带 `Cross-Origin-Resource-Policy: cross-origin`（本站资产已带）。阶段 5 的模拟页验证时 `crossOriginIsolated=false`，属预期。

## 视频转码

```bash
# H.264 MP4，faststart 便于边下边播
ffmpeg -i in.mov -c:v libx264 -preset slow -crf 24 -pix_fmt yuv420p -movflags +faststart -c:a aac -b:a 96k out.mp4
# WebM（VP9 + Opus），体积更小
ffmpeg -i in.mov -c:v libvpx-vp9 -b:v 0 -crf 33 -c:a libopus -b:a 64k out.webm
# 海报图
ffmpeg -i out.mp4 -ss 1 -frames:v 1 -c:v libwebp -quality 80 poster.webp
```

上传到 `video/`（视频）与 `img/`（海报），然后 `<Video src="…mp4" webm="…webm" poster="…webp" />`。超过两分钟的视频用 B 站嵌入。
