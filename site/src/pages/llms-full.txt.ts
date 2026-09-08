import type { APIRoute } from 'astro';
import { getPosts } from '@lib/content';
import { postToMarkdown } from '@lib/markdown';
import { absoluteUrl, site } from '../site.config';

/** llms-full.txt: every post as Markdown, newest first, with a metadata header per post. */
export const GET: APIRoute = async () => {
  const posts = await getPosts();
  const header = [
    `# ${site.name} (${site.englishName}) — 全文`,
    '',
    `> ${site.description}`,
    '',
    `共 ${posts.length} 篇文章，按发布日期倒序。每篇以一行 80 个等号分隔，随后是 YAML 元数据块与 Markdown 正文。`,
    `目录版本：${absoluteUrl('/llms.txt')}`,
    '',
  ].join('\n');

  const separator = '\n' + '='.repeat(80) + '\n\n';
  const body = posts.map((post) => postToMarkdown(post)).join(separator);

  return new Response(header + separator + body, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
};
