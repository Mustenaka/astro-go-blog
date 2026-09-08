/**
 * Keystatic API route (development only, injected by integrations/keystatic-dev.ts).
 * `localBaseDirectory` points at the repository root so collection paths like
 * `content/posts/**` resolve to <repo>/content/... instead of <repo>/site/content/...
 */
import { fileURLToPath } from 'node:url';
import { makeHandler } from '@keystatic/astro/api';
import keystaticConfig from '../../keystatic.config';

const repoRoot = fileURLToPath(new URL('../../../', import.meta.url));

export const ALL = makeHandler({ config: keystaticConfig, localBaseDirectory: repoRoot });
export const prerender = false;
