import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import MessageFormat from '@messageformat/core';
import { parse, Token } from '@messageformat/parser';

/**
 * The translation files as written, checked before the app ever loads them:
 * a broken file (a comma lost in a merge) leaves every window blank, with no
 * error anywhere. Tests run from the frontend folder.
 */
const dir = join(process.cwd(), 'src', 'i18n');
const files = readdirSync(dir).filter((f) => f.endsWith('.json'));

type Messages = Map<string, string>;

/** The messages of a file, by dotted key. */
function flatten(obj: object, prefix = '', into: Messages = new Map()): Messages {
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value !== null && typeof value === 'object') {
      flatten(value, path, into);
    } else {
      into.set(path, value);
    }
  }
  return into;
}

/** Parses a file, failing with its line and column. */
function read(file: string): Messages {
  const text = readFileSync(join(dir, file), 'utf8');
  try {
    return flatten(JSON.parse(text));
  } catch (err) {
    const at = /position (\d+)/.exec(String(err))?.[1];
    const before = at === undefined ? '' : text.slice(0, Number(at));
    const where = at === undefined ? '' : ` at line ${before.split('\n').length}, column ${before.length - before.lastIndexOf('\n')}`;
    throw new Error(`${file} is not valid JSON${where}: ${(err as Error).message}`);
  }
}

/** The parameters a message uses ({name}, {n, plural, …}, {n, selectordinal, …}), nested ones included. */
function params(tokens: Token[], into = new Set<string>()): Set<string> {
  for (const t of tokens) {
    if (t.type === 'argument' || t.type === 'function' || t.type === 'plural' || t.type === 'selectordinal' || t.type === 'select') {
      into.add(t.arg);
    }
    if (t.type === 'plural' || t.type === 'selectordinal' || t.type === 'select') {
      t.cases.forEach((c) => params(c.tokens, into));
    }
  }
  return into;
}

describe('translations', () => {
  it('are the en and fr files', () => {
    expect(files.sort()).toEqual(['en.json', 'fr.json']);
  });

  for (const file of files) {
    describe(file, () => {
      const lang = file.replace('.json', '');

      it('is valid JSON', () => {
        expect(() => read(file)).not.toThrow();
      });

      it('has only non-empty messages that compile', () => {
        const mf = new MessageFormat(lang);
        const errors: string[] = [];
        for (const [key, message] of read(file)) {
          if (typeof message !== 'string' || message.trim() === '') {
            errors.push(`${key}: empty or not a text`);
            continue;
          }
          try {
            mf.compile(message);
          } catch (err) {
            errors.push(`${key}: ${(err as Error).message}`);
          }
        }
        expect(errors).toEqual([]);
      });
    });
  }

  it('have the same keys and parameters in every language', () => {
    const [first, ...others] = files.map((f) => ({ file: f, messages: read(f) }));
    const errors: string[] = [];
    for (const other of others) {
      const keys = new Set([...first.messages.keys(), ...other.messages.keys()]);
      for (const key of keys) {
        const a = first.messages.get(key);
        const b = other.messages.get(key);
        if (a === undefined || b === undefined) {
          errors.push(`${key}: missing in ${a === undefined ? first.file : other.file}`);
          continue;
        }
        const pa = [...params(parse(a))].sort().join(', ');
        const pb = [...params(parse(b))].sort().join(', ');
        if (pa !== pb) {
          errors.push(`${key}: {${pa}} in ${first.file}, {${pb}} in ${other.file}`);
        }
      }
    }
    expect(errors).toEqual([]);
  });
});
