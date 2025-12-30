#!/usr/bin/env bun

/**
 * fzf-chezmoi-show - Browse chezmoi managed files
 * Output format: target\tsource\ttype_flags\tdisplay
 */

import { $ } from "bun";
import {
  colors,
  parseSourceFile,
  getTypeIndicators,
  getFileColor,
  padRight,
  formatError,
} from "./fzf-utils.ts";

interface ManagedFile {
  target: string;
  source: string;
  flags: ReturnType<typeof parseSourceFile>;
}

async function getManagedFiles(): Promise<ManagedFile[]> {
  // Get source-absolute paths (sorted by source path)
  const sourcesResult = await $`chezmoi managed --include=files --path-style=source-absolute`.quiet().nothrow();

  if (sourcesResult.exitCode !== 0) {
    console.error(formatError("chezmoi not found or not initialized"));
    process.exit(1);
  }

  const sourceAbsolutes = sourcesResult.text().trim().split("\n").filter(Boolean);

  if (sourceAbsolutes.length === 0) {
    return [];
  }

  // Get target paths for all sources in one call (preserves input order)
  const targetResult = await $`chezmoi target-path ${sourceAbsolutes}`.quiet().nothrow();

  if (targetResult.exitCode !== 0) {
    console.error(formatError("Failed to resolve target paths"));
    process.exit(1);
  }

  const targetAbsolutes = targetResult.text().trim().split("\n");
  const homeDir = process.env.HOME || "";

  return sourceAbsolutes.map((sourceAbs, i) => {
    const source = sourceAbs.split("/").pop() || "";
    // Remove home directory prefix to get relative target
    const target = targetAbsolutes[i].replace(homeDir + "/", "").replace(/^\//, "");
    return {
      target,
      source,
      flags: parseSourceFile(source),
    };
  });
}

function formatDisplay(file: ManagedFile): string {
  const indicators = getTypeIndicators(file.flags);
  const color = getFileColor(file.flags);
  
  // Pad indicators to 4 chars for alignment (most are 2 emojis = 4 visual chars)
  const paddedIndicators = padRight(indicators || "  ", 4);
  
  return (
    `${paddedIndicators} ` +
    `${color}${file.target}${colors.reset} ` +
    `${colors.arrow}<-${colors.reset} ` +
    `${colors.type}${file.source}${colors.reset}`
  );
}

async function main() {
  const files = await getManagedFiles();

  if (files.length === 0) {
    console.error(formatError("No managed files found"));
    process.exit(1);
  }

  // Output: target\tsource\tflags\tdisplay
  for (const file of files) {
    const flagStr = Object.entries(file.flags)
      .filter(([_, v]) => v)
      .map(([k]) => k.replace("is", "").toLowerCase())
      .join(",");
    
    const display = formatDisplay(file);
    console.log(`${file.target}\t${file.source}\t${flagStr}\t${display}`);
  }
}

main();
