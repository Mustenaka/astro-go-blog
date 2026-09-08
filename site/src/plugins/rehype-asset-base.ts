/**
 * Rewrites every `src`, `href` and `poster` attribute that starts with `/assets/`
 * so that it points at the configured asset base (the CDN in production, the Go
 * local storage adapter in development). Handles both plain HTML elements and
 * MDX JSX elements such as `<img src="/assets/..." />` written directly in MDX.
 */

const ASSET_PREFIX = '/assets/';
const ATTRIBUTES = ['src', 'href', 'poster'] as const;

interface HastNode {
  type: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: HastNode[];
  // MDX JSX nodes
  name?: string | null;
  attributes?: Array<{ type: string; name?: string; value?: unknown }>;
}

export interface RehypeAssetBaseOptions {
  /** Prefix that replaces `/assets/`, for example `https://cdn.example.com/assets`. */
  base: string;
}

export function rewriteAssetPath(value: string, base: string): string {
  if (!value.startsWith(ASSET_PREFIX)) return value;
  return base + value.slice(ASSET_PREFIX.length - 1);
}

export function rehypeAssetBase(options: RehypeAssetBaseOptions) {
  const base = options.base.replace(/\/+$/, '');
  const isAttribute = (name: string): name is (typeof ATTRIBUTES)[number] =>
    (ATTRIBUTES as readonly string[]).includes(name);

  const visit = (node: HastNode): void => {
    if (node.type === 'element' && node.properties) {
      for (const attr of ATTRIBUTES) {
        const value = node.properties[attr];
        if (typeof value === 'string') node.properties[attr] = rewriteAssetPath(value, base);
      }
    } else if (
      (node.type === 'mdxJsxFlowElement' || node.type === 'mdxJsxTextElement') &&
      Array.isArray(node.attributes)
    ) {
      for (const attribute of node.attributes) {
        if (
          attribute.type === 'mdxJsxAttribute' &&
          typeof attribute.name === 'string' &&
          isAttribute(attribute.name) &&
          typeof attribute.value === 'string'
        ) {
          attribute.value = rewriteAssetPath(attribute.value, base);
        }
      }
    }
    if (node.children) for (const child of node.children) visit(child);
  };

  return (tree: HastNode) => {
    visit(tree);
  };
}
