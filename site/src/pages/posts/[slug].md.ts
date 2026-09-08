import type { APIRoute } from 'astro';
import { getPosts, type Post } from '@lib/content';
import { postToMarkdown } from '@lib/markdown';

export async function getStaticPaths() {
  const posts = await getPosts();
  return posts.map((post) => ({ params: { slug: post.id }, props: { post } }));
}

export const GET: APIRoute<{ post: Post }> = ({ props }) =>
  new Response(postToMarkdown(props.post), {
    headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
  });
