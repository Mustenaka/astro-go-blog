import type { APIRoute } from 'astro';
import { getWorks, workUrl } from '@lib/content';
import { absolutizeAssets, stripMdxImports } from '@lib/markdown';
import { absoluteUrl, site } from '../site.config';

/** Strip the most common Markdown/MDX syntax to get a plain-text excerpt. */
function plainText(body: string, max = 400): string {
  const text = stripMdxImports(body)
    .replace(/<[^>\n]+>/g, ' ')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/\$\$[\s\S]*?\$\$/g, ' ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/[*_`>#-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  return text.length > max ? text.slice(0, max) + '…' : text;
}

/** /works.json: every work's front matter plus a plain-text excerpt, for AI and other readers. */
export const GET: APIRoute = async () => {
  const works = await getWorks();
  const payload = {
    site: site.name,
    generatedAt: new Date().toISOString(),
    works: works.map((work) => ({
      slug: work.id,
      url: absoluteUrl(workUrl(work.id)),
      ...work.data,
      cover: absolutizeAssets(work.data.cover),
      gallery: work.data.gallery?.map((g) => ({ ...g, src: absolutizeAssets(g.src), poster: g.poster ? absolutizeAssets(g.poster) : undefined })),
      demo: work.data.demo?.kind === 'iframe' ? { ...work.data.demo, src: absolutizeAssets(work.data.demo.src) } : work.data.demo,
      excerpt: plainText(work.body ?? ''),
    })),
  };
  return new Response(JSON.stringify(payload, null, 2), { headers: { 'Content-Type': 'application/json; charset=utf-8' } });
};
