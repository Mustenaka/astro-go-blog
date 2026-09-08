/**
 * Front matter schemas for every content collection (AGENTS.md section 5). This file is the
 * single source of truth: `content.config.ts` uses it for Astro's build-time validation and
 * `scripts/export-schema.mjs` turns it into JSON Schema under `content/schema/` for the Go side
 * (MCP tools) to validate the same rules.
 *
 * Keep this file free of Astro virtual imports and non-erasable TypeScript syntax so that Node
 * can load it with `--experimental-strip-types`.
 */
import { z } from 'astro/zod';

export const assetPath = z.string().startsWith('/assets/', 'must start with /assets/');
export const urlOrPath = z.union([z.url(), z.string().startsWith('/')]);
export const yearMonth = z.string().regex(/^\d{4}-(0[1-9]|1[0-2])$/, 'expected YYYY-MM');
export const slug = z.string().regex(/^[a-z0-9][a-z0-9-]{0,99}$/, 'lowercase letters, digits and hyphens');

export const postSchema = z.strictObject({
  title: z.string().min(1),
  description: z.string().min(1),
  date: z.coerce.date(),
  updated: z.coerce.date().optional(),
  tags: z.array(z.string()),
  categories: z.array(z.string()),
  draft: z.boolean().default(false),
  cover: assetPath.optional(),
  math: z.boolean().default(false),
  lang: z.string().default('zh-CN'),
  series: z.string().optional(),
  legacyUrls: z.array(z.string().startsWith('/')).optional(),
  toc: z.boolean().default(true),
});

export const workSchema = z.strictObject({
  title: z.string().min(1),
  summary: z.string().min(1),
  period: z.strictObject({ from: yearMonth, to: yearMonth.optional() }),
  role: z.string().optional(),
  stack: z.array(z.string()),
  links: z
    .strictObject({ repo: urlOrPath.optional(), demo: urlOrPath.optional(), post: urlOrPath.optional() })
    .optional(),
  cover: assetPath,
  gallery: z
    .array(
      z.strictObject({
        type: z.enum(['image', 'video']),
        src: assetPath,
        poster: assetPath.optional(),
        caption: z.string().optional(),
      }),
    )
    .optional(),
  demo: z
    .discriminatedUnion('kind', [
      z.strictObject({ kind: z.literal('island'), name: z.string().min(1) }),
      z.strictObject({ kind: z.literal('iframe'), src: z.string().min(1), aspect: z.string().optional() }),
    ])
    .optional(),
  featured: z.boolean().default(false),
  order: z.number().optional(),
  status: z.enum(['active', 'wip', 'archived']),
});

export const pageSchema = z.strictObject({
  title: z.string().min(1),
  description: z.string().optional(),
  updated: z.coerce.date().optional(),
});

export const friendSchema = z.strictObject({
  id: z.string().min(1),
  name: z.string().min(1),
  url: z.url(),
  description: z.string().optional(),
  avatar: z.string().optional(),
});

export const schemas = {
  posts: postSchema,
  works: workSchema,
  pages: pageSchema,
  friends: friendSchema,
};
