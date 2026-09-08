// Copies the KaTeX stylesheet and its woff2 fonts from node_modules into public/katex/
// so that math pages can self-host them (no CDN). Runs on install, dev and build.
import { copyFileSync, existsSync, mkdirSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const katexCss = require.resolve('katex/dist/katex.min.css');
const srcDir = fileURLToPath(new URL('.', `file://${katexCss.replace(/\\/g, '/')}`));
const destDir = fileURLToPath(new URL('../public/katex/', import.meta.url));

mkdirSync(`${destDir}fonts`, { recursive: true });
copyFileSync(katexCss, `${destDir}katex.min.css`);
let count = 0;
for (const file of readdirSync(`${srcDir}fonts`)) {
  if (!file.endsWith('.woff2')) continue;
  const target = `${destDir}fonts/${file}`;
  if (!existsSync(target)) copyFileSync(`${srcDir}fonts/${file}`, target);
  count++;
}
console.log(`[katex] copied katex.min.css and ${count} woff2 fonts to public/katex/`);
