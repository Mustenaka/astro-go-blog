import rss from '@astrojs/rss';
import type { APIRoute } from 'astro';
import { render } from 'astro:content';
import { experimental_AstroContainer as AstroContainer } from 'astro/container';
// Static imports so Vite bundles the renderers (and Vue's virtual app module) at build time.
// `loadRenderers()` imports them dynamically at runtime, which fails inside the prerender worker.
import mdxServer from '@astrojs/mdx/server.js';
import vueServer from '@astrojs/vue/server.js';
import { getPosts, postUrl } from '@lib/content';
import { absolutizeAssets } from '@lib/markdown';
import { site, siteUrl } from '../site.config';

/** Full-content RSS feed. MDX bodies are rendered to HTML with the container API. */
export const GET: APIRoute = async () => {
  const container = await AstroContainer.create();
  container.addServerRenderer({ name: '@astrojs/mdx', renderer: mdxServer });
  container.addServerRenderer({ name: '@astrojs/vue', renderer: vueServer });
  container.addClientRenderer({ name: '@astrojs/vue', entrypoint: '@astrojs/vue/client.js' });
  const posts = await getPosts();

  const items = await Promise.all(
    posts.map(async (post) => {
      const { Content } = await render(post);
      // Islands render their hydration runtime inline; feeds only want the static HTML.
      const html = (await container.renderToString(Content))
        .replace(/<script[\s\S]*?<\/script>/g, '')
        .replace(/<style[\s\S]*?<\/style>/g, '');
      return {
        title: post.data.title,
        description: post.data.description,
        pubDate: post.data.date,
        link: postUrl(post.id),
        categories: [...post.data.categories, ...post.data.tags],
        content: absolutizeAssets(html),
      };
    }),
  );

  return rss({
    title: site.name,
    description: site.description,
    site: siteUrl,
    trailingSlash: true,
    items,
    customData: `<language>zh-cn</language><managingEditor>${site.author.alias}</managingEditor>`,
  });
};
