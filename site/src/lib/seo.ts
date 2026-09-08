import type { Post, Work } from './content';
import { postUrl, workUrl } from './content';
import { absoluteUrl, site } from '../site.config';
import { assetUrl } from './assets';

const person = () => ({
  '@type': 'Person',
  name: site.author.name,
  alternateName: site.author.alias,
  url: absoluteUrl('/about/'),
  sameAs: [site.author.url],
});

export function personJsonLd() {
  return { '@context': 'https://schema.org', ...person(), description: site.author.bio };
}

export function websiteJsonLd() {
  return {
    '@context': 'https://schema.org',
    '@type': 'WebSite',
    name: site.name,
    alternateName: site.englishName,
    url: absoluteUrl('/'),
    description: site.description,
    inLanguage: site.lang,
    author: person(),
    potentialAction: {
      '@type': 'SearchAction',
      target: { '@type': 'EntryPoint', urlTemplate: absoluteUrl('/search/?q={search_term_string}') },
      'query-input': 'required name=search_term_string',
    },
  };
}

export function blogPostingJsonLd(post: Post) {
  const url = absoluteUrl(postUrl(post.id));
  return {
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: post.data.title,
    description: post.data.description,
    datePublished: post.data.date.toISOString(),
    dateModified: (post.data.updated ?? post.data.date).toISOString(),
    author: person(),
    publisher: person(),
    image: absoluteUrl(post.data.cover ? assetUrl(post.data.cover) : site.defaultOgImage),
    keywords: post.data.tags.join(', '),
    articleSection: post.data.categories,
    inLanguage: post.data.lang,
    url,
    mainEntityOfPage: { '@type': 'WebPage', '@id': url },
    isPartOf: { '@type': 'Blog', name: site.name, url: absoluteUrl('/') },
  };
}

export function softwareSourceCodeJsonLd(work: Work) {
  const url = absoluteUrl(workUrl(work.id));
  return {
    '@context': 'https://schema.org',
    '@type': 'SoftwareSourceCode',
    name: work.data.title,
    description: work.data.summary,
    url,
    codeRepository: work.data.links?.repo,
    programmingLanguage: work.data.stack,
    author: person(),
    image: absoluteUrl(assetUrl(work.data.cover)),
    dateCreated: `${work.data.period.from}-01`,
    inLanguage: site.lang,
  };
}
