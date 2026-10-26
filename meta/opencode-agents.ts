#!/usr/bin/env bun

// Converts Claude Code subagent definitions into OpenCode v2 agent files.
// Usage: bun meta/opencode-agents.ts <claude-agents-dir> <opencode-agents-dir>

import { mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const [, , srcDir, outDir] = process.argv;
if (!srcDir || !outDir) {
  console.error("Usage: opencode-agents.ts <claude-agents-dir> <opencode-agents-dir>");
  process.exit(1);
}

const PROVIDER = "anthropic";
const MODELS: Record<string, string> = {
  haiku: "claude-haiku-5-5",
  sonnet: "claude-sonnet-5-5",
  opus: "claude-opus-5-5",
};

type Rule = { action: string; resource: string; effect: "deny" };

function permissionsFor(tools: string[] | undefined, disallowed: string[]): Rule[] {
  const denied = (action: string): Rule => ({ action, resource: "*", effect: "deny" });
  const can = (tool: string, ...also: string[]) =>
    !disallowed.some((t) => [tool, ...also].includes(t)) &&
    (tools === undefined || tools.some((t) => [tool, ...also].includes(t)));
  const rules: Rule[] = [];
  if (!can("Edit", "Write")) rules.push(denied("edit"));
  if (!can("Bash")) rules.push(denied("shell"));
  return rules;
}

function convert(source: string): string {
  const match = source.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) throw new Error("missing frontmatter");
  const [, head, body] = match;

  const fields = new Map<string, string>();
  for (const line of head.split("\n")) {
    const sep = line.indexOf(":");
    if (sep > 0) fields.set(line.slice(0, sep).trim(), line.slice(sep + 1).trim());
  }
  const list = (key: string) => fields.get(key)?.split(",").map((t) => t.trim());

  const out = ["---", `description: ${fields.get("description") ?? '""'}`, "mode: subagent"];
  const model = MODELS[fields.get("model") ?? ""];
  if (model) out.push(`model: ${PROVIDER}/${model}`);

  const rules = permissionsFor(list("tools"), list("disallowedTools") ?? []);
  if (rules.length > 0) {
    out.push("permissions:");
    for (const r of rules) {
      out.push(`  - action: ${r.action}`, `    resource: "${r.resource}"`, `    effect: ${r.effect}`);
    }
  }
  out.push("---", body);
  return out.join("\n");
}

mkdirSync(outDir, { recursive: true });
for (const file of readdirSync(srcDir).filter((f) => f.endsWith(".md"))) {
  writeFileSync(join(outDir, file), convert(readFileSync(join(srcDir, file), "utf8")));
}
