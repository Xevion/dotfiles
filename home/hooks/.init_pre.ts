#!/usr/bin/env -S deno run -A

import { resolve } from "https://deno.land/std/path/mod.ts";
import { exists } from "jsr:@std/fs";
import { join } from "node:path";
import { $, os } from "npm:zx@8.3.2";

const { exit } = Deno;
const filePath = join(os.homedir(), "key.txt");


if (await exists(resolve(filePath))) {
  console.log("key.txt already exists");
  Deno.exit(0);
}

// Acquire the secret from Doppler
const result = await $`doppler secrets get KEY_TXT --plain`;

// Check if the command was successful
if (result.exitCode !== 0) {
  console.error("Failed to get secret KEY_TXT");
  exit(1);
}

// Write the secret to a file
await Deno.writeTextFile(resolve(filePath), result.stdout);
console.log("key.txt bootstrapped");
