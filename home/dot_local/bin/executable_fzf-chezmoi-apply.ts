#!/usr/bin/env bun

/**
 * fzf-chezmoi-apply - Interactive chezmoi apply with status display
 * Output format: target\tstatus\tsource\tdisplay
 */

import { $ } from "bun";
import {
  colors,
  parseSourceFile,
  formatError,
} from "./fzf-utils.ts";

interface StatusEntry {
  target: string;
  statusCode: string;
  action: "add" | "modify" | "delete" | "run";
  source?: string;
}

const statusLabels: Record<string, { label: string; color: string; action: StatusEntry["action"] }> = {
  A: { label: "ADD", color: colors.add, action: "add" },
  M: { label: "MOD", color: colors.modify, action: "modify" },
  D: { label: "DEL", color: colors.delete, action: "delete" },
  R: { label: "RUN", color: colors.script, action: "run" },
};

async function getStatus(): Promise<StatusEntry[]> {
  const result = await $`chezmoi status`.quiet().nothrow();

  if (result.exitCode !== 0) {
    console.error(formatError("chezmoi command failed"));
    process.exit(1);
  }

  // Don't trim() - it removes leading spaces which are part of the status format
  const lines = result.text().split("\n").filter(line => line.length > 0);
  
  if (lines.length === 0) {
    // No changes - this is a success, not an error
    console.error("✅ No changes to apply - target is in sync with source");
    process.exit(0);
  }

  const entries: StatusEntry[] = [];

  for (const line of lines) {
    // Format: "XY path" where X is last state, Y is target state (what will happen)
    const statusCode = line.substring(0, 2);
    const target = line.substring(3);
    
    // We care about the second character (what will happen on apply)
    const actionChar = statusCode[1];
    const statusInfo = statusLabels[actionChar];
    
    if (statusInfo) {
      entries.push({
        target,
        statusCode,
        action: statusInfo.action,
      });
    }
  }

  return entries;
}

async function getSourcePaths(targets: string[]): Promise<Map<string, string>> {
  if (targets.length === 0) return new Map();

  const homeDir = process.env.HOME || "";
  
  // Get source paths for specific targets (preserves input order)
  const targetPaths = targets.map(t => `${homeDir}/${t}`);
  const result = await $`chezmoi source-path ${targetPaths}`.quiet().nothrow();

  if (result.exitCode !== 0) {
    return new Map(); // Fallback to no source paths
  }

  const sourcePaths = result.text().trim().split("\n").filter(Boolean);
  
  const sourceMap = new Map<string, string>();
  targets.forEach((t, i) => {
    // Extract just the basename from the full source path
    const source = sourcePaths[i]?.split("/").pop() || "";
    sourceMap.set(t, source);
  });

  return sourceMap;
}

function formatDisplay(entry: StatusEntry, source?: string): string {
  const statusInfo = statusLabels[entry.statusCode[1]] || { label: "CHG", color: colors.type };
  
  const sourceInfo = source 
    ? ` ${colors.arrow}(${colors.type}${source}${colors.arrow})${colors.reset}`
    : "";

  return (
    `${statusInfo.color}[${statusInfo.label}]${colors.reset} ` +
    `${colors.name}${entry.target}${colors.reset}` +
    sourceInfo
  );
}

async function main() {
  const entries = await getStatus();
  
  // Get source paths for all targets
  const sourceMap = await getSourcePaths(entries.map(e => e.target));

  // Output: target\tstatus\tsource\tdisplay
  for (const entry of entries) {
    const source = sourceMap.get(entry.target) || "";
    const display = formatDisplay(entry, source);
    console.log(`${entry.target}\t${entry.action}\t${source}\t${display}`);
  }
}

main();
