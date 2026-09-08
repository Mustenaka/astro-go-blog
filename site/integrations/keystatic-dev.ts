/**
 * Development-only Keystatic wiring. Mirrors @keystatic/astro's integration (virtual config
 * module, the UI page) but injects our own API route so the local storage root can be the
 * repository instead of site/ (`localBaseDirectory`). It only activates for `astro dev` (or with
 * KEYSTATIC=1), and adds the React integration Keystatic's UI needs at the same time, so production
 * builds contain neither Keystatic nor React.
 */
import { mkdirSync, writeFileSync } from 'node:fs';
import type { AstroIntegration } from 'astro';
import react from '@astrojs/react';

export function keystaticDev(): AstroIntegration {
  return {
    name: 'keystatic-dev',
    hooks: {
      'astro:config:setup': ({ command, injectRoute, updateConfig, config, logger }) => {
        const enabled = process.env.KEYSTATIC === '1' || (command === 'dev' && process.env.KEYSTATIC !== '0');
        if (!enabled) {
          logger.info('Keystatic disabled (command=' + command + '); set KEYSTATIC=1 to force');
          return;
        }
        updateConfig({
          integrations: [react()],
          // Keystatic's UI calls /api/keystatic/<path> without trailing slashes; the production
          // 'always' rule would 404 them. Relaxed in development only.
          trailingSlash: 'ignore',
          vite: {
            plugins: [
              {
                name: 'keystatic-config-virtual',
                resolveId(id) {
                  if (id === 'virtual:keystatic-config') return this.resolve('./keystatic.config', './a');
                  return null;
                },
              },
            ],
            optimizeDeps: { entries: ['keystatic.config.*', '.astro/keystatic-imports.js'] },
          },
        });
        const dotAstroDir = new URL('./.astro/', config.root);
        mkdirSync(dotAstroDir, { recursive: true });
        writeFileSync(new URL('keystatic-imports.js', dotAstroDir), 'import "@keystatic/astro/ui";\nimport "@keystatic/astro/api";\nimport "@keystatic/core/ui";\n');
        injectRoute({
          entrypoint: '@keystatic/astro/internal/keystatic-astro-page.astro',
          pattern: '/keystatic/[...params]',
          prerender: false,
        });
        injectRoute({
          entrypoint: new URL('./src/keystatic/api.ts', config.root).pathname,
          pattern: '/api/keystatic/[...params]',
          prerender: false,
        });
        logger.info('Keystatic enabled at /keystatic (content root: repository)');
      },
    },
  };
}
