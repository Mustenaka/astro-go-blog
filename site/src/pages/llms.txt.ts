import type { APIRoute } from 'astro';
import { getCategoryIndex, getPosts, getWorks, postUrl, workUrl } from '@lib/content';
import { absoluteUrl, site } from '../site.config';

/** llms.txt: site summary, author, and every post grouped by category. */
export const GET: APIRoute = async () => {
  const posts = await getPosts();
  const works = await getWorks();
  const categories = getCategoryIndex(posts);

  const lines: string[] = [
    `# ${site.name} (${site.englishName})`,
    '',
    `> ${site.description}`,
    '',
    `站点地址：${absoluteUrl('/')}`,
    `全文版本：${absoluteUrl('/llms-full.txt')}`,
    `每篇文章都有 Markdown 版本：把文章地址末尾的 \`/\` 换成 \`.md\`，例如 ${absoluteUrl('/posts/<slug>.md')}`,
    `作品集机器可读版本：${absoluteUrl('/works.json')}`,
    `RSS（全文）：${absoluteUrl('/rss.xml')}`,
    '',
    '## 作者',
    '',
    `- 姓名：${site.author.name}（${site.author.alias}）`,
    `- 简介：${site.author.bio}`,
    `- GitHub：${site.author.url}`,
    `- 关于页：${absoluteUrl('/about/')}`,
    '',
    `## 文章（共 ${posts.length} 篇）`,
  ];

  for (const category of categories) {
    lines.push('', `### ${category.name}`, '');
    for (const post of category.posts) {
      lines.push(`- [${post.data.title}](${absoluteUrl(postUrl(post.id))}): ${post.data.description}`);
    }
  }

  if (works.length) {
    lines.push('', '## 作品', '');
    for (const work of works) {
      lines.push(`- [${work.data.title}](${absoluteUrl(workUrl(work.id))}): ${work.data.summary}`);
    }
  }

  return new Response(lines.join('\n') + '\n', { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
};
