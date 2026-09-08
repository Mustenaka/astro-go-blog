/**
 * robots.txt policy lives here and only here. `src/pages/robots.txt.ts` renders it.
 *
 * Default: every user agent, including AI crawlers, may crawl everything.
 *
 * To block a specific AI crawler, add its user-agent token to `disallowAgents`, e.g.
 *   disallowAgents: ['GPTBot', 'ClaudeBot', 'CCBot', 'Google-Extended']
 * Each listed agent gets its own `User-agent` block with `Disallow: /`.
 *
 * To hide specific paths from everyone, add them to `disallowPaths`.
 *
 * When `PUBLIC_NOINDEX` is `1` or `true` (staging on the temporary subdomain), this file is
 * ignored and robots.txt disallows everything for every agent.
 */
export const robotsConfig = {
  disallowAgents: [] as string[],
  disallowPaths: [] as string[],
  sitemap: '/sitemap-index.xml',
};
