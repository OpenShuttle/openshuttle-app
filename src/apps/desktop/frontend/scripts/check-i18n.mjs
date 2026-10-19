// Fails when the translation files do not define exactly the same keys.
// Transloco does not check keys at build time, so CI runs this instead.
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const dir = join(import.meta.dirname, '..', 'src', 'i18n');

function flatten(obj, prefix = '') {
  return Object.entries(obj).flatMap(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key;
    return value !== null && typeof value === 'object' ? flatten(value, path) : [path];
  });
}

const files = readdirSync(dir).filter((f) => f.endsWith('.json'));
const keys = new Map(
  files.map((f) => [f, new Set(flatten(JSON.parse(readFileSync(join(dir, f), 'utf8'))))]),
);
const all = new Set([...keys.values()].flatMap((set) => [...set]));

let failed = false;
for (const [file, set] of keys) {
  const missing = [...all].filter((k) => !set.has(k)).sort((a, b) => a.localeCompare(b));
  if (missing.length > 0) {
    failed = true;
    console.error(`${file} is missing ${missing.length} key(s):\n  ${missing.join('\n  ')}`);
  }
}

if (failed) {
  process.exit(1);
}
console.log(`i18n OK: ${files.join(', ')} share ${all.size} keys.`);
