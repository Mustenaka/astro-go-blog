# content/

站点的唯一真实来源。所有文章、作品、固定页面、友链都是这里的文件；后端、后台、MCP 工具改内容的方式只有一种：写这里的文件。

```
content/
  posts/<发布年份>/<slug>.mdx   文章。slug 取自文件名，全站唯一
  works/<slug>.mdx             作品集
  pages/<slug>.mdx             固定页面：about（简历）、friends（友链）
  data/friends.yaml            友链数据，数组，每项要有唯一 id
  schema/*.schema.json         front matter 的 JSON Schema（生成文件，勿手改）
```

## front matter 规则

字段表在仓库根 `AGENTS.md` 第 5 节。规则只定义一次，在 `site/src/content/schemas.ts`（zod）：

- Astro 构建时用它校验，字段错误直接构建失败。
- `pnpm -C site schema`（`pnpm -C site build` 会自动跑）把它导出为 `content/schema/*.schema.json`。
- Go 后端与 MCP 工具读这些 JSON Schema 做同样的校验。

改字段：只改 `schemas.ts`，然后跑 `pnpm -C site schema` 并把生成的 JSON 一起提交。

## 约定

- 日期写 `YYYY-MM-DD`，不加引号。
- 资产一律写 `/assets/<key>`，key 由后端在上传时生成（见 `AGENTS.md` 5.4 与"上传一个资产的步骤"）。
- 草稿设 `draft: true`：开发服务器可见，生产构建排除。
- 从 WordPress 迁来的文章填 `legacyUrls`，构建会生成 301 映射。
- 不改已发布文章的 slug，不删旧文章，不删 `legacyUrls`。
