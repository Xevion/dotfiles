#!/usr/bin/env bun
console.log("init_pre.ts");

import { exists } from "node:fs/promises";
import { join } from "node:path";
import { homedir } from "node:os";
import { $ } from "bun";

const filePath = join(homedir(), "key.txt");

if (await exists(filePath)) {
  console.log("key.txt already exists");
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
console.log("key.txt bootstrapped");
