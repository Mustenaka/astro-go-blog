import { getCollection, type CollectionEntry } from 'astro:content';

export type Post = CollectionEntry<'posts'>;
export type Work = CollectionEntry<'works'>;
export type Page = CollectionEntry<'pages'>;
export type Friend = CollectionEntry<'friends'>;

/** Published posts, newest first. Drafts are only included outside production builds. */
export async function getPosts(): Promise<Post[]> {
  const posts = await getCollection('posts', (entry) => (import.meta.env.PROD ? !entry.data.draft : true));
  return posts.sort((a, b) => b.data.date.getTime() - a.data.date.getTime());
}

/** Works ordered by `order` (ascending), then by period start (newest first). */
export async function getWorks(): Promise<Work[]> {
  const works = await getCollection('works');
  return works.sort((a, b) => {
    const ao = a.data.order ?? Number.MAX_SAFE_INTEGER;
    const bo = b.data.order ?? Number.MAX_SAFE_INTEGER;
    if (ao !== bo) return ao - bo;
    return b.data.period.from.localeCompare(a.data.period.from);
  });
}

export async function getPages(): Promise<Page[]> {
  return getCollection('pages');
}

export async function getFriends(): Promise<Friend[]> {
  return getCollection('friends');
}

/* ---------- URLs ---------- */

export const postUrl = (slug: string) => `/posts/${slug}/`;
export const workUrl = (slug: string) => `/works/${slug}/`;
export const tagUrl = (tag: string) => `/tags/${encodeURIComponent(tag)}/`;
export const categoryUrl = (category: string) => `/categories/${encodeURIComponent(category)}/`;
export const postsPageUrl = (page: number) => (page <= 1 ? '/posts/' : `/posts/page/${page}/`);

/* ---------- Grouping ---------- */

export interface TermIndexEntry {
  name: string;
  count: number;
  posts: Post[];
}

function indexBy(posts: Post[], pick: (post: Post) => readonly string[]): TermIndexEntry[] {
  const map = new Map<string, Post[]>();
  for (const post of posts) {
    for (const term of pick(post)) {
      const list = map.get(term) ?? [];
      list.push(post);
      map.set(term, list);
    }
  }
  return [...map.entries()]
    .map(([name, list]) => ({ name, count: list.length, posts: list }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name, 'zh-CN'));
}

export const getTagIndex = (posts: Post[]) => indexBy(posts, (p) => p.data.tags);
export const getCategoryIndex = (posts: Post[]) => indexBy(posts, (p) => p.data.categories);

export function groupByYear(posts: Post[]): { year: number; posts: Post[] }[] {
  const map = new Map<number, Post[]>();
  for (const post of posts) {
    const year = post.data.date.getUTCFullYear();
    const list = map.get(year) ?? [];
    list.push(post);
    map.set(year, list);
  }
  return [...map.entries()].sort((a, b) => b[0] - a[0]).map(([year, list]) => ({ year, posts: list }));
}

/** Previous (older) and next (newer) posts around `slug` in a newest-first list. */
export function getAdjacent(posts: Post[], slug: string): { prev?: Post; next?: Post } {
  const index = posts.findIndex((p) => p.id === slug);
  if (index === -1) return {};
  return { next: posts[index - 1], prev: posts[index + 1] };
}

/** Posts of a series, oldest first. */
export function getSeriesPosts(posts: Post[], series: string): Post[] {
  return posts.filter((p) => p.data.series === series).sort((a, b) => a.data.date.getTime() - b.data.date.getTime());
}

export function paginate<T>(items: T[], perPage: number): T[][] {
  const pages: T[][] = [];
  for (let i = 0; i < items.length; i += perPage) pages.push(items.slice(i, i + perPage));
  return pages.length ? pages : [[]];
}

export function formatDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}
