/**
 * Keystatic: a visual editor for content/ in development only. Storage is local; the content
 * root is the repository (see integrations/keystatic-dev.ts, which passes localBaseDirectory).
 *
 * Every field mirrors AGENTS.md section 5 / src/content/schemas.ts. Nested structures that
 * Keystatic cannot serialise in the exact shape our schema expects are declared `ignored`, which
 * keeps their existing value on save; edit those through the MCP tools or by hand.
 */
import { collection, config, fields, singleton } from '@keystatic/core';
import { block } from '@keystatic/core/content-components';

const assetPattern = { regex: /^(\/assets\/.+)?$/, message: '必须以 /assets/ 开头' };
const yearMonth = { regex: /^\d{4}-(0[1-9]|1[0-2])$/, message: '格式 YYYY-MM' };

const assetText = (label: string, description?: string) =>
  fields.text({ label, description, validation: { pattern: assetPattern } });

/** Shared MDX components. The names must match site/src/plugins/remark-auto-import.ts. */
const mdxComponents = {
  Demo: block({
    label: 'Demo（three.js 岛屿）',
    schema: {
      name: fields.text({ label: '注册名', description: 'site/src/demos/registry.ts 里的名字', validation: { isRequired: true } }),
      caption: fields.text({ label: '说明' }),
    },
  }),
  Video: block({
    label: 'Video',
    schema: {
      src: assetText('MP4', '/assets/video/...'),
      webm: assetText('WebM'),
      poster: assetText('海报图'),
      caption: fields.text({ label: '说明' }),
    },
  }),
  UnityEmbed: block({
    label: 'UnityEmbed',
    schema: {
      src: assetText('index.html', '/assets/demos/<name>/<version>/index.html'),
      aspect: fields.text({ label: '宽高比', defaultValue: '16/9' }),
      sizeHint: fields.text({ label: '体积提示', description: '如 25 MB' }),
      caption: fields.text({ label: '说明' }),
    },
  }),
  Counter: block({
    label: 'Counter（示例岛屿）',
    schema: { start: fields.integer({ label: '起始值', defaultValue: 0 }) },
  }),
  // Raw <img> tags in MDX (hand-written or migrated content) need a definition or the editor refuses
  // the file. Markdown images (![alt](src)) do not go through this.
  img: block({
    label: 'HTML 图片 <img>',
    schema: {
      src: assetText('src', '/assets/img/...'),
      alt: fields.text({ label: 'alt' }),
    },
  }),
};

const body = fields.mdx({
  label: '正文',
  extension: 'mdx',
  components: mdxComponents,
  options: {
    // The editor cannot represent /assets/ images (they live on the CDN, not in the repository):
    // with image handling on it escapes Markdown images into plain text. Keep it off; posts that
    // contain Markdown images are edited through the MCP tools or by hand (see docs/decisions/0002).
    image: false,
  },
});

const stringList = (label: string) => fields.array(fields.text({ label }), { label, itemLabel: (p) => p.value });

export default config({
  storage: { kind: 'local' },
  ui: { brand: { name: '木十的博客 · 内容' } },
  collections: {
    posts: collection({
      label: '文章',
      path: 'content/posts/**',
      slugField: 'title',
      entryLayout: 'content',
      format: { contentField: 'body' },
      columns: ['date', 'draft'],
      schema: {
        title: fields.slug({
          name: { label: '标题', validation: { isRequired: true } },
          slug: {
            label: 'slug（含年份目录）',
            description: '形如 2026/my-post：年份目录与发布日期一致，slug 只用小写字母、数字、连字符',
            validation: { pattern: { regex: /^\d{4}\/[a-z0-9][a-z0-9-]*$/, message: '形如 2026/my-post' } },
          },
        }),
        description: fields.text({ label: '摘要', multiline: true, validation: { isRequired: true } }),
        date: fields.date({ label: '发布日期', defaultValue: { kind: 'today' }, validation: { isRequired: true } }),
        updated: fields.date({ label: '更新日期' }),
        tags: stringList('标签'),
        categories: stringList('分类'),
        draft: fields.checkbox({ label: '草稿', defaultValue: false }),
        cover: assetText('封面', '/assets/img/...'),
        math: fields.checkbox({ label: '含公式（加载 KaTeX）', defaultValue: false }),
        lang: fields.text({ label: '语言', defaultValue: 'zh-CN' }),
        series: fields.text({ label: '系列' }),
        legacyUrls: fields.array(fields.text({ label: '旧站路径' }), { label: '旧站路径（301）', itemLabel: (p) => p.value }),
        toc: fields.checkbox({ label: '显示目录', defaultValue: true }),
        body,
      },
    }),
    works: collection({
      label: '作品',
      path: 'content/works/*',
      slugField: 'title',
      entryLayout: 'content',
      format: { contentField: 'body' },
      columns: ['status', 'featured'],
      schema: {
        title: fields.slug({ name: { label: '作品名', validation: { isRequired: true } } }),
        summary: fields.text({ label: '摘要', multiline: true, validation: { isRequired: true } }),
        period: fields.object(
          {
            from: fields.text({ label: '开始', validation: { isRequired: true, pattern: yearMonth } }),
            to: fields.text({ label: '结束', description: '进行中留空', validation: { pattern: { regex: /^(\d{4}-(0[1-9]|1[0-2]))?$/, message: '格式 YYYY-MM' } } }),
          },
          { label: '时间' },
        ),
        role: fields.text({ label: '角色' }),
        stack: stringList('技术栈'),
        links: fields.ignored(),
        cover: assetText('封面', '/assets/img/...'),
        gallery: fields.ignored(),
        demo: fields.ignored(),
        featured: fields.checkbox({ label: '首页精选', defaultValue: false }),
        order: fields.number({ label: '排序（小在前）' }),
        status: fields.select({
          label: '状态',
          options: [
            { label: '进行中', value: 'active' },
            { label: '开发中', value: 'wip' },
            { label: '已归档', value: 'archived' },
          ],
          defaultValue: 'wip',
        }),
        body,
      },
    }),
    pages: collection({
      label: '固定页面',
      path: 'content/pages/*',
      slugField: 'title',
      entryLayout: 'content',
      format: { contentField: 'body' },
      schema: {
        title: fields.slug({ name: { label: '标题', validation: { isRequired: true } } }),
        description: fields.text({ label: '摘要', multiline: true }),
        updated: fields.date({ label: '更新日期' }),
        body,
      },
    }),
  },
  singletons: {
    friends: singleton({
      label: '友链',
      path: 'content/data/friends',
      format: 'yaml',
      schema: {
        friends: fields.array(
          fields.object({
            id: fields.text({ label: 'id', validation: { isRequired: true, pattern: { regex: /^[a-z0-9-]+$/, message: '小写字母、数字、连字符' } } }),
            name: fields.text({ label: '名称', validation: { isRequired: true } }),
            url: fields.url({ label: '链接', validation: { isRequired: true } }),
            description: fields.text({ label: '说明' }),
            avatar: fields.text({ label: '头像', validation: { pattern: assetPattern } }),
          }),
          { label: '友链', itemLabel: (p) => p.fields.name.value || p.fields.id.value },
        ),
      },
    }),
  },
});
