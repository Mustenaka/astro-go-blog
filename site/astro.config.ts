import { fileURLToPath } from 'node:url';
import { defineConfig } from 'astro/config';
import { loadEnv } from 'vite';
import mdx from '@astrojs/mdx';
import sitemap from '@astrojs/sitemap';
import vue from '@astrojs/vue';
import { unified } from '@astrojs/markdown-remark';
import tailwindcss from '@tailwindcss/vite';
import rehypeKatex from 'rehype-katex';
import remarkMath from 'remark-math';
import { rehypeAssetBase } from './src/plugins/rehype-asset-base';

const siteDir = fileURLToPath(new URL('.', import.meta.url));
const repoRoot = fileURLToPath(new URL('..', import.meta.url));

// Astro loads `.env` only after this config is evaluated, so read it here.
// Only `.env` and `.env.local` are used; the mode argument does not select a file.
const env = loadEnv(process.env.NODE_ENV ?? 'development', siteDir, 'PUBLIC_');
const siteUrl = env.PUBLIC_SITE_URL || 'http://localhost:4321';
const assetBase = env.PUBLIC_ASSET_BASE || 'http://localhost:8080/assets';
const devAssetOrigin = 'http://localhost:8080';

export default defineConfig({
  site: siteUrl,
  trailingSlash: 'always',
  integrations: [vue(), mdx(), sitemap()],
  markdown: {
    processor: unified({
      remarkPlugins: [remarkMath],
      rehypePlugins: [rehypeKatex, [rehypeAssetBase, { base: assetBase }]],
    }),
    shikiConfig: {
      themes: { light: 'github-light', dark: 'github-dark' },
    },
  },
  vite: {
    plugins: [tailwindcss()],
    server: {
      // Content lives at the repo root, outside site/, so Vite must be allowed to serve it.
      fs: { allow: [repoRoot] },
      // In development the Go server (phase 3) serves local assets. Until then this proxy
      // simply returns an error for /assets/ requests; it does not affect `astro build`.
      proxy: {
        '/assets/': { target: devAssetOrigin, changeOrigin: true },
      },
    },
  },
});
