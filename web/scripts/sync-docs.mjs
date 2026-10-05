// Refresh the served reference docs from their canonical source before every
// build, so web/public/docs/reference/*.md can't silently drift from
// docs/reference/*.md again. (It just did: the token benchmark was rewritten
// in the repo but the served copy kept the old, contradicted claims — which
// the landing page links to.)
//
// Conservative by design: it only overwrites files that ALREADY exist in BOTH
// places. It never adds or deletes — so public-only docs (getting-started,
// how-it-works, why, …) and intentionally-divergent files (README, FOR_AGENTS)
// are left untouched. To publish a new reference doc, copy it in once; from
// then on it stays in sync automatically.
import { readdirSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const repoRoot = join(webRoot, '..');
const servedRefDir = join(webRoot, 'public', 'docs', 'reference');
const canonicalRefDir = join(repoRoot, 'docs', 'reference');

if (!existsSync(servedRefDir) || !existsSync(canonicalRefDir)) {
  console.log('[sync-docs] reference dirs not found — skipping');
  process.exit(0);
}

let synced = 0;
for (const name of readdirSync(servedRefDir)) {
  if (!name.endsWith('.md')) continue;
  const canonical = join(canonicalRefDir, name);
  const served = join(servedRefDir, name);
  if (!existsSync(canonical)) continue; // public-only doc — leave it
  const src = readFileSync(canonical);
  if (!readFileSync(served).equals(src)) {
    writeFileSync(served, src);
    console.log(`[sync-docs] refreshed reference/${name}`);
    synced++;
  }
}
console.log(`[sync-docs] ${synced} reference doc(s) refreshed from docs/reference/`);
