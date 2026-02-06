#!/usr/bin/env bun
/**
 * setup-ai-configs - Configure AI tool symlinks for Cursor and Antigravity
 *
 * Creates symlinks so Cursor and Antigravity read from the project's AGENTS.md file.
 *
 * Usage:
 *   setup-ai-configs --cursor        # Create .cursorrules → AGENTS.md
 *   setup-ai-configs --antigravity   # Create .antigravity/rules.md → AGENTS.md
 *   setup-ai-configs --all           # Create both
 */

import { existsSync, mkdirSync, symlinkSync, lstatSync, readlinkSync } from "fs";
import { join, resolve } from "path";
import { parseArgs } from "util";

const MINIMAL_AGENTS_TEMPLATE = `# Project Guidelines

## Overview

[Brief project description]

## Tech Stack

- [List main technologies]

## Development

- Run \`just check\` for linting
- Run \`just test\` for tests

## Key Patterns

[Document important patterns, conventions, or gotchas]
`;

interface Config {
  cursor: boolean;
  antigravity: boolean;
  help: boolean;
}

function parseArguments(): Config {
  const { values } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      cursor: { type: "boolean", default: false },
      antigravity: { type: "boolean", default: false },
      all: { type: "boolean", default: false },
      help: { type: "boolean", short: "h", default: false },
    },
    allowPositionals: false,
  });

  return {
    cursor: values.cursor || values.all || false,
    antigravity: values.antigravity || values.all || false,
    help: values.help || false,
  };
}

function printHelp(): void {
  console.log(`setup-ai-configs - Configure AI tool symlinks for Cursor and Antigravity

Usage:
  setup-ai-configs [options]

Options:
  --cursor        Create .cursorrules symlink pointing to AGENTS.md
  --antigravity   Create .antigravity/rules.md symlink pointing to AGENTS.md
  --all           Create both Cursor and Antigravity configs
  -h, --help      Show this help message

Examples:
  setup-ai-configs --all           # Set up both tools
  setup-ai-configs --cursor        # Set up Cursor only
  setup-ai-configs --antigravity   # Set up Antigravity only

The script will:
  1. Check if AGENTS.md exists in the current directory
  2. Offer to create a minimal template if missing
  3. Create the appropriate symlinks for the selected tools
`);
}

async function promptYesNo(question: string): Promise<boolean> {
  process.stdout.write(`${question} [y/N] `);

  for await (const line of console) {
    const answer = line.trim().toLowerCase();
    return answer === "y" || answer === "yes";
  }
  return false;
}

function checkAgentsMd(cwd: string): boolean {
  return existsSync(join(cwd, "AGENTS.md"));
}

async function offerCreateAgentsMd(cwd: string): Promise<boolean> {
  console.log("No AGENTS.md found in current directory.");
  const create = await promptYesNo("Would you like to create a minimal template?");

  if (create) {
    const path = join(cwd, "AGENTS.md");
    await Bun.write(path, MINIMAL_AGENTS_TEMPLATE);
    console.log(`Created ${path}`);
    return true;
  }

  return false;
}

function isSymlink(path: string): boolean {
  try {
    return lstatSync(path).isSymbolicLink();
  } catch {
    return false;
  }
}

function createSymlink(target: string, linkPath: string, description: string): boolean {
  if (existsSync(linkPath)) {
    if (isSymlink(linkPath)) {
      const existingTarget = readlinkSync(linkPath);
      if (existingTarget === target) {
        console.log(`✓ ${description} already exists and points to correct target`);
        return true;
      }
      console.log(`⚠ ${description} exists but points to: ${existingTarget}`);
      console.log(`  Expected: ${target}`);
      console.log(`  Skipping - remove manually if you want to update it`);
      return false;
    }
    console.log(`⚠ ${linkPath} exists but is not a symlink - skipping`);
    return false;
  }

  try {
    symlinkSync(target, linkPath);
    console.log(`✓ Created ${description}: ${linkPath} → ${target}`);
    return true;
  } catch (err) {
    console.error(`✗ Failed to create ${description}: ${err}`);
    return false;
  }
}

function setupCursor(cwd: string): boolean {
  const linkPath = join(cwd, ".cursorrules");
  return createSymlink("AGENTS.md", linkPath, ".cursorrules");
}

function setupAntigravity(cwd: string): boolean {
  const antigravityDir = join(cwd, ".antigravity");

  // Create .antigravity directory if needed
  if (!existsSync(antigravityDir)) {
    try {
      mkdirSync(antigravityDir);
      console.log(`✓ Created .antigravity/ directory`);
    } catch (err) {
      console.error(`✗ Failed to create .antigravity/ directory: ${err}`);
      return false;
    }
  }

  const linkPath = join(antigravityDir, "rules.md");
  // Symlink needs to go up one directory to reach AGENTS.md
  return createSymlink("../AGENTS.md", linkPath, ".antigravity/rules.md");
}

async function main(): Promise<void> {
  const config = parseArguments();

  if (config.help) {
    printHelp();
    process.exit(0);
  }

  if (!config.cursor && !config.antigravity) {
    console.error("Error: No tool specified. Use --cursor, --antigravity, or --all");
    console.error("Run with --help for usage information.");
    process.exit(1);
  }

  const cwd = process.cwd();
  console.log(`Working directory: ${cwd}\n`);

  // Check for AGENTS.md
  if (!checkAgentsMd(cwd)) {
    const created = await offerCreateAgentsMd(cwd);
    if (!created) {
      console.log("\nCannot create symlinks without AGENTS.md. Exiting.");
      process.exit(1);
    }
    console.log();
  }

  let success = true;

  if (config.cursor) {
    console.log("Setting up Cursor...");
    if (!setupCursor(cwd)) {
      success = false;
    }
  }

  if (config.antigravity) {
    console.log("Setting up Antigravity...");
    if (!setupAntigravity(cwd)) {
      success = false;
    }
  }

  console.log();
  if (success) {
    console.log("Done! AI config symlinks are set up.");
  } else {
    console.log("Completed with some warnings - check output above.");
    process.exit(1);
  }
}

main();
