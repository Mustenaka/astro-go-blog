import type { APIRoute } from 'astro';
import { robotsConfig } from '../robots.config';
import { absoluteUrl, noindex } from '../site.config';

export const GET: APIRoute = () => {
  const lines: string[] = ['# Policy is defined in site/src/robots.config.ts'];

  if (noindex) {
    lines.push('# PUBLIC_NOINDEX is set: this deployment must not be indexed.', 'User-agent: *', 'Disallow: /');
  } else {
    lines.push('User-agent: *');
    if (robotsConfig.disallowPaths.length === 0) lines.push('Disallow:');
    for (const path of robotsConfig.disallowPaths) lines.push(`Disallow: ${path}`);
    for (const agent of robotsConfig.disallowAgents) lines.push('', `User-agent: ${agent}`, 'Disallow: /');
    lines.push('', `Sitemap: ${absoluteUrl(robotsConfig.sitemap)}`);
  }

  return new Response(lines.join('\n') + '\n', { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
};
