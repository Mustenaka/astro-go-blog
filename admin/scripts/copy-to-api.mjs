// Copies admin/dist into api/internal/adminui/dist so `go build` embeds it.
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const src = fileURLToPath(new URL('../dist/', import.meta.url));
const dest = fileURLToPath(new URL('../../api/internal/adminui/dist/', import.meta.url));

if (!existsSync(`${src}index.html`)) {
  console.error('[admin] dist/index.html missing; run vite build first');
  process.exit(1);
}
mkdirSync(dest, { recursive: true });
for (const entry of readdirSync(dest)) {
  if (entry !== '.gitkeep') rmSync(`${dest}${entry}`, { recursive: true, force: true });
}
cpSync(src, dest, { recursive: true });
console.log(`[admin] copied ${readdirSync(dest).length - 1} entries to api/internal/adminui/dist/`);
