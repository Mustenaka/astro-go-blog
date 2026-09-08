/**
 * Content polish applied to rendered Markdown/MDX:
 * - images: lazy loading, async decoding, `data-lightbox` marker (unless wrapped in a link)
 * - tables: wrapped in `<div class="table-wrap">` so they scroll inside their own box
 * - external links: `target="_blank"` and `rel="noopener noreferrer"`
 */

interface HastNode {
  type: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: HastNode[];
}

export interface RehypeContentEnhanceOptions {
  /** Site origin; links to other origins are treated as external. */
  siteUrl: string;
}

export function rehypeContentEnhance(options: RehypeContentEnhanceOptions) {
  const origin = options.siteUrl.replace(/\/+$/, '');

  const visit = (node: HastNode, parent: HastNode | null, insideLink: boolean): void => {
    if (node.type === 'element' && node.properties) {
      const props = node.properties;
      if (node.tagName === 'img') {
        props['loading'] ??= 'lazy';
        props['decoding'] ??= 'async';
        if (!insideLink) props['dataLightbox'] = '';
      } else if (node.tagName === 'a' && typeof props['href'] === 'string') {
        const href = props['href'];
        if (/^https?:\/\//.test(href) && !href.startsWith(origin + '/') && href !== origin) {
          props['target'] ??= '_blank';
          props['rel'] ??= 'noopener noreferrer';
        }
      } else if (node.tagName === 'table' && parent?.children) {
        const index = parent.children.indexOf(node);
        if (index !== -1 && !(parent.tagName === 'div' && (parent.properties?.['className'] as string[] | undefined)?.includes('table-wrap'))) {
          parent.children[index] = {
            type: 'element',
            tagName: 'div',
            properties: { className: ['table-wrap'] },
            children: [node],
          };
        }
      }
    }
    const link = insideLink || (node.type === 'element' && node.tagName === 'a');
    if (node.children) for (const child of [...node.children]) visit(child, node, link);
  };

  return (tree: HastNode) => {
    visit(tree, null, false);
  };
}
