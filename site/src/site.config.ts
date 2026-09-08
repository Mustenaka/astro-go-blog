/**
 * Site-wide constants. Everything that is "about this site" rather than "about the code"
 * lives here: names, author, navigation, pagination sizes, footer identity elements.
 */
export const site = {
  name: '木十的博客',
  englishName: 'Mustenaka',
  tagline: 'Unity、实时物理仿真与 AI 应用的技术笔记',
  description:
    '木十（Mustenaka）的个人博客：Unity/C#、实时物理仿真、AI 应用方向的技术文章、作品集与简历。',
  lang: 'zh-CN',
  author: {
    name: '木十',
    alias: 'Mustenaka',
    url: 'https://github.com/Mustenaka',
    bio: '软件工程师，方向是 Unity/C#、实时物理仿真、AI 应用。',
  },
  social: [
    { label: 'GitHub', url: 'https://github.com/Mustenaka' },
    { label: 'RSS', url: '/rss.xml' },
  ],
  nav: [
    { label: '文章', href: '/posts/' },
    { label: '作品', href: '/works/' },
    { label: '归档', href: '/archive/' },
    { label: '标签', href: '/tags/' },
    { label: '关于', href: '/about/' },
    { label: '友链', href: '/friends/' },
    { label: '搜索', href: '/search/' },
  ],
  postsPerPage: 15,
  homeLatestPosts: 10,
  homeFeaturedWorks: 4,
  icp: { number: '桂ICP备19006307号-1', url: 'https://beian.miit.gov.cn/' },
  quote: { text: 'We know what we are, but know not what we may be.', source: 'William Shakespeare, Hamlet' },
  defaultOgImage: '/og-default.png',
  readingSpeed: { cjkCharsPerMinute: 400, wordsPerMinute: 200 },
} as const;

/** Canonical site origin without trailing slash, from `astro.config` `site`. */
export const siteUrl = (import.meta.env.SITE ?? 'http://localhost:4321').replace(/\/+$/, '');

export function absoluteUrl(path: string): string {
  if (/^https?:\/\//.test(path)) return path;
  return siteUrl + (path.startsWith('/') ? path : `/${path}`);
}

export const noindex = ['1', 'true'].includes(String(import.meta.env.PUBLIC_NOINDEX ?? '').toLowerCase());
