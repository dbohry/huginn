// Fails when the built frontend is larger than the budget.
// Counts gzip-compressed JavaScript, CSS and HTML, which is what a browser downloads.
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, extname, resolve, relative } from 'node:path';
import { gzipSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)), '../internal/webui/dist');
const budgetKB = Number(process.env.BUNDLE_BUDGET_KB ?? 100);
const counted = new Set(['.js', '.css', '.html']);

function* walk(dir) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) yield* walk(path);
    else yield path;
  }
}

let total = 0;
for (const file of walk(root)) {
  if (!counted.has(extname(file))) continue;
  const size = gzipSync(readFileSync(file), { level: 9 }).length;
  total += size;
  console.log(`${(size / 1024).toFixed(1).padStart(7)} KB  ${relative(root, file)}`);
}

const totalKB = total / 1024;
console.log(`${totalKB.toFixed(1).padStart(7)} KB  total (budget ${budgetKB} KB)`);
if (totalKB > budgetKB) {
  console.error(`Bundle is over budget by ${(totalKB - budgetKB).toFixed(1)} KB`);
  process.exit(1);
}
