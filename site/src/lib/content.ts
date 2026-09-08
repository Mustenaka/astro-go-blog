import { getCollection, type CollectionEntry } from 'astro:content';

export type Post = CollectionEntry<'posts'>;
export type Work = CollectionEntry<'works'>;

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

export function formatDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}
