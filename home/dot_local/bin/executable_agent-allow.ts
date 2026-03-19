#!/usr/bin/env bun

/**
 * agent-allow - Add a bash pattern to the shared permissions allowlist.
 *
 * Edits meta/permissions.ts in the chezmoi source directory, then optionally
 * runs chezmoi apply to deploy the updated configs to both OpenCode and Claude Code.
 *
 * Usage:
 *   agent-allow 'go test'              # add as "allow" (default)
 *   agent-allow --ask 'git push'       # add as "ask"
 *   agent-allow --deny 'rm -rf /'      # add as "deny"
 *   agent-allow --apply 'cargo run'    # add and immediately deploy
 */

import { readFileSync, writeFileSync } from "fs";
import { join } from "path";
import { $ } from "bun";

const CHEZMOI_SOURCE = join(
  process.env.HOME!,
  ".local",
  "share",
  "chezmoi",
);
const PERMISSIONS_FILE = join(CHEZMOI_SOURCE, "meta", "permissions.ts");

// Marker comment where new entries get appended
const CUSTOM_SECTION_MARKER = "// == Custom (added via agent-allow) ==";

function parseArgs() {
  const args = process.argv.slice(2);
  let level: "allow" | "ask" | "deny" = "allow";
  let apply = false;
  const patterns: string[] = [];

  for (const arg of args) {
    if (arg === "--ask") level = "ask";
    else if (arg === "--deny") level = "deny";
    else if (arg === "--allow") level = "allow";
    else if (arg === "--apply") apply = true;
    else if (arg === "--help" || arg === "-h") {
      console.log(`Usage: agent-allow [--allow|--ask|--deny] [--apply] <pattern> [pattern...]

Examples:
  agent-allow 'go test'              # allow go test
  agent-allow --ask 'git push'       # prompt for git push
  agent-allow --deny 'rm -rf /'      # deny rm -rf /
  agent-allow --apply 'cargo run'    # allow and deploy immediately
  agent-allow 'touch' 'mv' 'cp'     # allow multiple patterns`);
      process.exit(0);
    } else {
      patterns.push(arg);
    }
  }

  if (patterns.length === 0) {
    console.error("Error: at least one pattern is required");
    process.exit(1);
  }

  return { level, patterns, apply };
}

function ensureCustomSection(content: string): string {
  if (content.includes(CUSTOM_SECTION_MARKER)) return content;

  // Insert the custom section before the OUTPUT FORMATTING section
  const outputMarker = "// =============================================================================\n// OUTPUT FORMATTING";
  const idx = content.indexOf(outputMarker);
  if (idx === -1) {
    console.error("Error: could not find OUTPUT FORMATTING section in permissions.ts");
    process.exit(1);
  }

  const insertion = `${CUSTOM_SECTION_MARKER}\n\n${outputMarker}`;
  return content.slice(0, idx) + insertion + content.slice(idx + outputMarker.length);
}

function addPatterns(content: string, level: string, patterns: string[]): string {
  const markerIdx = content.indexOf(CUSTOM_SECTION_MARKER);
  if (markerIdx === -1) {
    console.error("Error: custom section marker not found");
    process.exit(1);
  }

  // Build the new lines
  const calls = patterns.map((p) => `${level}("${p}");`).join("\n");
  const insertPoint = markerIdx + CUSTOM_SECTION_MARKER.length;

  return content.slice(0, insertPoint) + "\n" + calls + content.slice(insertPoint);
}

async function main() {
  const { level, patterns, apply } = parseArgs();

  let content = readFileSync(PERMISSIONS_FILE, "utf8");

  // Check for duplicates
  const existing = patterns.filter((p) => content.includes(`"${p}"`));
  if (existing.length > 0) {
    console.warn(`Warning: already present: ${existing.join(", ")}`);
  }
  const newPatterns = patterns.filter((p) => !content.includes(`"${p}"`));
  if (newPatterns.length === 0) {
    console.log("Nothing to add (all patterns already exist).");
    return;
  }

  content = ensureCustomSection(content);
  content = addPatterns(content, level, newPatterns);
  writeFileSync(PERMISSIONS_FILE, content);

  for (const p of newPatterns) {
    console.log(`+ ${level}("${p}")`);
  }
  console.log(`\nUpdated ${PERMISSIONS_FILE}`);

  if (apply) {
    console.log("\nDeploying via chezmoi apply...");
    await $`chezmoi apply ~/.config/opencode/opencode.jsonc ~/claude-settings.json`.quiet();
    console.log("Done. Both configs updated.");
  } else {
    console.log("\nRun 'chezmoi apply' to deploy, or use --apply next time.");
  }
}

main();
