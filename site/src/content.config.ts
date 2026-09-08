import { defineCollection } from 'astro:content';
import { file, glob } from 'astro/loaders';
import { z } from 'astro/zod';

// Content lives at the repository root, outside site/. Paths are relative to site/.
const CONTENT = '../content';

/** Slug is the file name without extension; directories (e.g. the year) are not part of it. */
const slugFromFileName = ({ entry }: { entry: string }) =>
  entry.replace(/\\/g, '/').split('/').pop()!.replace(/\.mdx?$/, '');

const assetPath = z.string().startsWith('/assets/', 'must start with /assets/');
const urlOrPath = z.union([z.url(), z.string().startsWith('/')]);
const yearMonth = z.string().regex(/^\d{4}-(0[1-9]|1[0-2])$/, 'expected YYYY-MM');

const posts = defineCollection({
  loader: glob({ pattern: '**/*.mdx', base: `${CONTENT}/posts`, generateId: slugFromFileName }),
  schema: z.strictObject({
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
  }),
});

const works = defineCollection({
  loader: glob({ pattern: '*.mdx', base: `${CONTENT}/works`, generateId: slugFromFileName }),
  schema: z.strictObject({
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
  }),
});

const pages = defineCollection({
  loader: glob({ pattern: '*.mdx', base: `${CONTENT}/pages`, generateId: slugFromFileName }),
  schema: z.strictObject({
    title: z.string().min(1),
    description: z.string().optional(),
    updated: z.coerce.date().optional(),
  }),
});

const friends = defineCollection({
  loader: file(`${CONTENT}/data/friends.yaml`),
  schema: z.strictObject({
    id: z.string().min(1),
    name: z.string().min(1),
    url: z.url(),
    description: z.string().optional(),
    avatar: z.string().optional(),
  }),
});

export const collections = { posts, works, pages, friends };
