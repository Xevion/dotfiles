#!/usr/bin/env bun

// Masks the volatile settings declared in reconcile-policy.ts, so app churn
// stays out of diffs while every other setting still surfaces.
//
//   clean <path>   git clean filter: volatile keys read as their recorded value
//   modify <path>  chezmoi modify_ body: stdin is the deployed file; prints the
//                  source with the deployed file's volatile values kept
//   pull <path>    write the deployed file back into the source, cleaned
//
// <path> is a repo-relative source path, as keyed in POLICIES.

import { homedir } from "node:os";
import { join, resolve } from "node:path";
import { applyEdits, findNodeAtLocation, getNodeValue, modify, parseTree } from "jsonc-parser";
import { POLICIES, type Policy, type Value } from "./reconcile-policy";

interface Adapter {
  get(text: string, key: string): Value | undefined;
  /** Replaces or inserts the key; null removes it. An insert mirrors its position in `reference`. */
  set(text: string, key: string, value: Value | null, reference?: string): string;
}

const jsonc: Adapter = {
  get(text, key) {
    const tree = parseTree(text);
    const node = tree && findNodeAtLocation(tree, key.split("/"));
    return node ? getNodeValue(node) : undefined;
  },
  set(text, key, value) {
    const options = { formattingOptions: { insertSpaces: true, tabSize: 2 } };
    return applyEdits(text, modify(text, key.split("/"), value ?? undefined, options));
  },
};

const SECTION = /^\s*\[(.+)\]\s*$/;
const ENTRY = /^(\s*([^=#;[\s][^=]*?)\s*=\s*)(.*)$/;

/** A key with no "/" lives before the first section header (btop has none). */
function splitKey(key: string): [string | undefined, string] {
  const sep = key.indexOf("/");
  return sep < 0 ? [undefined, key] : [key.slice(0, sep), key.slice(sep + 1)];
}

/** Line index of the entry (-1 if absent), and where a new one would go (-1: no section). */
function locate(lines: string[], key: string): { entry: number; insertAt: number } {
  const [section, name] = splitKey(key);
  const firstHeader = lines.findIndex((l) => SECTION.test(l));
  const tail = lines.at(-1) === "" ? lines.length - 1 : lines.length;
  let insertAt = section === undefined ? (firstHeader < 0 ? tail : firstHeader) : -1;
  let entry = -1;
  let current: string | undefined;

  lines.forEach((line, i) => {
    const header = line.match(SECTION);
    if (header) {
      current = header[1];
      if (current === section) insertAt = i + 1;
      return;
    }
    const match = current === section ? line.match(ENTRY) : null;
    if (!match) return;
    if (match[2] === name) entry = i;
    if (section !== undefined) insertAt = i + 1;
  });
  return { entry, insertAt };
}

/** Insert position right after the key's nearest predecessor in `reference` that `lines` also has. */
function anchor(lines: string[], reference: string[], key: string): number | undefined {
  const [section] = splitKey(key);
  const qualify = (name: string) => (section === undefined ? name : `${section}/${name}`);
  for (let i = locate(reference, key).entry - 1; i >= 0; i--) {
    if (SECTION.test(reference[i])) {
      const header = lines.findIndex((l) => l.match(SECTION)?.[1] === section);
      return header < 0 ? undefined : header + 1;
    }
    const prev = reference[i].match(ENTRY);
    const found = prev ? locate(lines, qualify(prev[2])).entry : -1;
    if (found >= 0) return found + 1;
  }
  return undefined;
}

const ini: Adapter = {
  get(text, key) {
    const lines = text.split("\n");
    const { entry } = locate(lines, key);
    return entry < 0 ? undefined : lines[entry].match(ENTRY)![3];
  },
  set(text, key, value, reference) {
    const lines = text.split("\n");
    const { entry, insertAt: fallback } = locate(lines, key);
    const insertAt = (reference && anchor(lines, reference.split("\n"), key)) ?? fallback;
    if (value === null) {
      if (entry >= 0) lines.splice(entry, 1);
    } else if (entry >= 0) {
      lines[entry] = lines[entry].match(ENTRY)![1] + value;
    } else {
      const [section, name] = splitKey(key);
      const sep = lines.some((l) => ENTRY.test(l) && / = /.test(l)) ? " = " : "=";
      const line = `${name}${sep}${value}`;
      if (insertAt >= 0) lines.splice(insertAt, 0, line);
      else lines.splice(lines.at(-1) === "" ? lines.length - 1 : lines.length, 0, "", `[${section}]`, line);
    }
    return lines.join("\n");
  },
};

const ADAPTERS: Record<Policy["format"], Adapter> = { jsonc, ini };

function clean(policy: Policy, text: string): string {
  const format = ADAPTERS[policy.format];
  return Object.entries(policy.volatile).reduce((out, [key, recorded]) => {
    const current = format.get(out, key);
    return current === undefined || current === recorded ? out : format.set(out, key, recorded);
  }, text);
}

function adopt(policy: Policy, source: string, deployed: string): string {
  const format = ADAPTERS[policy.format];
  return Object.keys(policy.volatile).reduce((out, key) => {
    const value = format.get(deployed, key);
    return value === undefined ? out : format.set(out, key, value, deployed);
  }, source);
}

const [command, path] = process.argv.slice(2);
const root = resolve(import.meta.dir, "..");
const policy = path ? POLICIES[path] : undefined;

function requirePolicy(withTarget = false): Policy {
  if (!policy || (withTarget && !policy.target)) {
    console.error(`reconcile: no ${withTarget ? "modify_ " : ""}policy for ${path ?? "(missing path)"}`);
    process.exit(1);
  }
  return policy;
}

switch (command) {
  case "clean": {
    // Unknown paths and unreadable content pass through byte for byte.
    const input = await Bun.stdin.bytes();
    let output: string | Uint8Array = input;
    if (policy) {
      try {
        output = clean(policy, new TextDecoder().decode(input));
      } catch (err) {
        console.error(`reconcile: passing ${path} through unchanged: ${err}`);
      }
    }
    process.stdout.write(output);
    break;
  }
  case "modify": {
    const p = requirePolicy(true);
    const deployed = await Bun.stdin.text();
    const source = await Bun.file(join(root, path)).text();
    process.stdout.write(deployed ? adopt(p, source, deployed) : source);
    break;
  }
  case "pull": {
    const p = requirePolicy(true);
    const deployed = await Bun.file(join(homedir(), p.target!)).text();
    await Bun.write(join(root, path), clean(p, deployed));
    console.error(`reconcile: ~/${p.target} -> ${path}`);
    break;
  }
  default:
    console.error("Usage: reconcile.ts <clean|modify|pull> <repo-relative source path>");
    process.exit(1);
}
