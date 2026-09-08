import { defineCollection } from 'astro:content';
import { file, glob } from 'astro/loaders';
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
  loader: file(`${CONTENT}/data/friends.yaml`),
  schema: friendSchema,
});

export const collections = { posts, works, pages, friends };
