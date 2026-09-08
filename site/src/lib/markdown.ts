import type { Post } from './content';
import { formatDate, postUrl } from './content';
import { absoluteUrl } from '../site.config';

/** Asset base as an absolute URL, whatever form PUBLIC_ASSET_BASE takes. */
export function absoluteAssetBase(): string {
  const base = (import.meta.env.PUBLIC_ASSET_BASE ?? '/assets').replace(/\/+$/, '');
  return absoluteUrl(base);
}

/** Rewrite `/assets/...` occurrences in Markdown or HTML text to absolute URLs. */
export function absolutizeAssets(text: string): string {
  const base = absoluteAssetBase();
  return text.replace(/(\(|"|'|\s|^)\/assets\//g, `$1${base}/`);
}

/** Drop top-level MDX `import` lines; they mean nothing to a Markdown reader. */
export function stripMdxImports(body: string): string {
  return body.replace(/^import\s[^\n]*\n?/gm, '').replace(/^\n+/, '');
}

function yamlString(value: string): string {
  return JSON.stringify(value);
}

/** Front matter as a YAML metadata block, followed by the Markdown body. */
export function postToMarkdown(post: Post): string {
  const { data } = post;
  const lines = [
    '---',
    `title: ${yamlString(data.title)}`,
    `description: ${yamlString(data.description)}`,
    `date: ${formatDate(data.date)}`,
  ];
  if (data.updated) lines.push(`updated: ${formatDate(data.updated)}`);
  lines.push(`tags: [${data.tags.map(yamlString).join(', ')}]`);
  lines.push(`categories: [${data.categories.map(yamlString).join(', ')}]`);
  if (data.series) lines.push(`series: ${yamlString(data.series)}`);
  lines.push(`lang: ${data.lang}`);
  if (data.cover) lines.push(`cover: ${absolutizeAssets(data.cover)}`);
  lines.push(`url: ${absoluteUrl(postUrl(post.id))}`);
  lines.push(`author: ${yamlString('木十 / Mustenaka')}`);
  lines.push('---', '');
  const body = absolutizeAssets(stripMdxImports(post.body ?? ''));
  return `${lines.join('\n')}\n${body.trimEnd()}\n`;
}
