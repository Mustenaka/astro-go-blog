import { rewriteAssetPath } from '../plugins/rehype-asset-base';

const base = (import.meta.env.PUBLIC_ASSET_BASE ?? '/assets').replace(/\/+$/, '');

/** Resolve a `/assets/...` path from front matter (cover, gallery) to its public URL. */
export function assetUrl(path: string): string {
  return rewriteAssetPath(path, base);
}
