import { defineCollection } from 'astro:content';
import { file, glob } from 'astro/loaders';
import { parseFrontmatter } from '@astrojs/markdown-remark';
import { friendSchema, pageSchema, postSchema, workSchema } from './content/schemas';

// Content lives at the repository root, outside site/. Paths are relative to site/.
const CONTENT = '../content';

/** Slug is the file name without extension; directories (e.g. the year) are not part of it. */
const slugFromFileName = ({ entry }: { entry: string }) =>
  entry.replace(/\\/g, '/').split('/').pop()!.replace(/\.mdx?$/, '');

const posts = defineCollection({
  loader: glob({ pattern: '**/*.mdx', base: `${CONTENT}/posts`, generateId: slugFromFileName }),
  schema: postSchema,
});

const works = defineCollection({
  loader: glob({ pattern: '*.mdx', base: `${CONTENT}/works`, generateId: slugFromFileName }),
  schema: workSchema,
});

const pages = defineCollection({
  loader: glob({ pattern: '*.mdx', base: `${CONTENT}/pages`, generateId: slugFromFileName }),
  schema: pageSchema,
});

const friends = defineCollection({
  // friends.yaml keeps its entries under a top-level `friends` key (Keystatic singleton); parse the
  // YAML through the front matter parser (no extra dependency) and hand Astro the array.
  loader: file(`${CONTENT}/data/friends.yaml`, {
    parser: (text) => (parseFrontmatter(`---
${text}
---`).frontmatter.friends ?? []) as Record<string, unknown>[],
  }),
  schema: friendSchema,
});

export const collections = { posts, works, pages, friends };
