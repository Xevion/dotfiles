#!/usr/bin/env bun

import { exists } from "node:fs/promises";
import { join } from "node:path";
import { homedir } from "node:os";
import { $ } from "bun";

// Type-safe wrapper around console.log that prepends [init_pre]
const log = (...args: any[]): void => {
  console.log("[init_pre]", ...args);
};

const filePath = join(homedir(), "key.txt");
const chezmoiSourceDir = join(homedir(), ".local", "share", "chezmoi");

// Configure git to include repo-local .gitconfig (for age diff support)
const gitIncludeResult = await $`git -C ${chezmoiSourceDir} config --local include.path ../.gitconfig`.quiet();
if (gitIncludeResult.exitCode === 0) {
  log("git include.path configured");
} else {
  log("warning: failed to configure git include.path");
}

if (await exists(filePath)) {
  log("key.txt already exists");
  process.exit(0);
}

// Acquire the secret from Doppler
const result = await $`doppler secrets get KEY_TXT --plain`.quiet();

// Check if the command was successful
if (result.exitCode !== 0) {
  console.error("Failed to get secret KEY_TXT");
  process.exit(1);
}

// Write the secret to a file
await Bun.write(filePath, result.stdout);
log("key.txt bootstrapped");
