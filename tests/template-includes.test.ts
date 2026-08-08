import { describe, test, expect } from "bun:test";
import { Glob } from "bun";
import { existsSync } from "node:fs";
import { basename, dirname, resolve } from "node:path";

/**
 * Chezmoi's `include` resolves relative to the source directory and takes the
 * literal source filename, attribute prefixes and all (`executable_foo.ts`, not
 * `foo.ts`). A stale path is not a local failure: template rendering happens up
 * front, so one bad include makes every chezmoi command exit non-zero.
 */

const sourceDir = resolve(import.meta.dir, "..", "home");
const includeRe = /\{\{-?\s*include\s+"([^"]+)"/g;
const attributePrefixes = ["executable_", "private_", "dot_", "symlink_", "encrypted_"];

/** Guess the attribute-prefixed source name a bare target was meant to reference. */
function attributeGuess(resolved: string): string | null {
  const base = basename(resolved);
  const dir = dirname(resolved);
  for (const prefix of attributePrefixes) {
    if (existsSync(resolve(dir, prefix + base))) return prefix + base;
  }
  return null;
}

const includes: { template: string; target: string }[] = [];
for await (const rel of new Glob("**/*.tmpl").scan({ cwd: sourceDir, dot: true })) {
  const text = await Bun.file(resolve(sourceDir, rel)).text();
  for (const match of text.matchAll(includeRe)) {
    includes.push({ template: rel, target: match[1] });
  }
}
includes.sort((a, b) => a.template.localeCompare(b.template) || a.target.localeCompare(b.target));

describe("chezmoi include paths resolve", () => {
  test("templates were scanned", () => {
    expect(includes.length).toBeGreaterThan(0);
  });

  for (const { template, target } of includes) {
    test(`${template} -> ${target}`, () => {
      const resolved = resolve(sourceDir, target);
      if (existsSync(resolved)) return;

      const guess = attributeGuess(resolved);
      const hint = guess ? ` Did you mean "${dirname(target)}/${guess}"?` : "";
      throw new Error(`${template}: include "${target}" does not exist.${hint}`);
    });
  }
});
