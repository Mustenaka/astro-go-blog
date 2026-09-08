// Exports the zod front matter schemas as JSON Schema (draft 2020-12) into content/schema/.
// The Go side (MCP tools) validates against these files so the rules live in one place.
// Run with: node --experimental-strip-types --no-warnings scripts/export-schema.mjs
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { z } from 'astro/zod';
import { schemas } from '../src/content/schemas.ts';

const outDir = fileURLToPath(new URL('../../content/schema/', import.meta.url));
mkdirSync(outDir, { recursive: true });

const descriptions = {
  posts: 'Front matter of content/posts/<year>/<slug>.mdx (AGENTS.md 5.1). Dates are YYYY-MM-DD.',
  works: 'Front matter of content/works/<slug>.mdx (AGENTS.md 5.2).',
  pages: 'Front matter of content/pages/<slug>.mdx (AGENTS.md 5.3).',
  friends: 'One entry of content/data/friends.yaml.',
};

let changed = 0;
for (const [name, schema] of Object.entries(schemas)) {
  const json = z.toJSONSchema(schema, {
    target: 'draft-2020-12',
    io: 'input',
    unrepresentable: 'any',
    override: (ctx) => {
      // z.coerce.date() has no JSON representation; front matter writes ISO dates.
      if (ctx.zodSchema._zod?.def?.type === 'date') {
        delete ctx.jsonSchema.anyOf;
        ctx.jsonSchema.type = 'string';
        ctx.jsonSchema.format = 'date';
        ctx.jsonSchema.description = 'YYYY-MM-DD';
      }
    },
  });
  json.$id = `https://mustenaka.cn/schema/${name}.schema.json`;
  json.title = name;
  json.description = descriptions[name];
  const text = JSON.stringify(json, null, 2) + '\n';
  const file = `${outDir}${name}.schema.json`;
  let previous = '';
  try {
    previous = readFileSync(file, 'utf8');
  } catch {}
  if (previous !== text) {
    writeFileSync(file, text);
    changed++;
  }
}
console.log(`[schema] ${Object.keys(schemas).length} schemas exported to content/schema/ (${changed} changed)`);
