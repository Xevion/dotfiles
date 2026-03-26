#!/usr/bin/env bun

/**
 * reminder - Manage ~/reminders/ files
 *
 * Usage:
 *   reminder                   List all reminders
 *   reminder list              List all reminders
 *   reminder add <name>        Create a new reminder and open in editor
 *   reminder rm <name>         Remove a reminder
 *   reminder show <name>       View a reminder (glow > bat > cat)
 *   reminder edit <name>       Edit a reminder in $EDITOR
 */

import { existsSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { homedir } from "node:os";
import { join, basename } from "node:path";
import { $ } from "bun";

const REMINDERS_DIR = join(homedir(), "reminders");

// Catppuccin Mocha palette — 24-bit truecolor (matches recall's theme.ts)
function rgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `\x1b[38;2;${r};${g};${b}m`;
}

const theme = {
  lavender: rgb("#b4befe"), // accent — brand, headers, icons
  peach: rgb("#fab387"),    // highlight — counts, values
  subtext0: rgb("#a6adc8"), // muted — labels, secondary text
  overlay0: rgb("#6c7086"), // dim — separators, empty states
  green: rgb("#a6e3a1"),    // success
  red: rgb("#f38ba8"),      // error
  sky: rgb("#89dceb"),      // links
};

const RESET = "\x1b[0m";
const BOLD = "\x1b[1m";
const UNDERLINE = "\x1b[4m";

function stripExtension(filename: string): string {
  return filename.replace(/\.(md|txt)$/, "");
}

function getReminderFiles(): string[] {
  if (!existsSync(REMINDERS_DIR)) return [];
  return readdirSync(REMINDERS_DIR)
    .filter((f) => !f.startsWith("."))
    .sort();
}

/** Find a reminder file by name, trying exact match then with extensions */
function resolveFile(name: string): string | null {
  const candidates = [
    join(REMINDERS_DIR, name),
    join(REMINDERS_DIR, `${name}.md`),
    join(REMINDERS_DIR, `${name}.txt`),
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate)) return candidate;
  }
  return null;
}

/** OSC 8 hyperlink: makes text clickable in supporting terminals */
function hyperlink(url: string, text: string): string {
  return `\x1b]8;;${url}\x1b\\${UNDERLINE}${theme.sky}${text}${RESET}\x1b]8;;\x1b\\`;
}

function fileUrl(path: string): string {
  return `file://${encodeURI(path).replace(/%2F/g, "/")}`;
}

async function findViewer(): Promise<string | null> {
  for (const cmd of ["glow", "bat", "cat"]) {
    const result = await $`command -v ${cmd}`.quiet().nothrow();
    if (result.exitCode === 0) return cmd;
  }
  return "cat";
}

async function findEditor(): Promise<string | null> {
  if (process.env.EDITOR) return process.env.EDITOR;
  for (const cmd of ["micro", "nano", "vi"]) {
    const result = await $`command -v ${cmd}`.quiet().nothrow();
    if (result.exitCode === 0) return cmd;
  }
  return null;
}

function listReminders(): void {
  const files = getReminderFiles();
  if (files.length === 0) {
    console.log(`${theme.overlay0}No reminders.${RESET}`);
    return;
  }

  for (let i = 0; i < files.length; i++) {
    const name = stripExtension(files[i]);
    const fullPath = join(REMINDERS_DIR, files[i]);
    const link = hyperlink(fileUrl(fullPath), name);
    console.log(`  ${theme.overlay0}${i + 1}.${RESET} ${link}`);
  }
}

async function addReminder(name: string): Promise<void> {
  mkdirSync(REMINDERS_DIR, { recursive: true });

  const file = join(REMINDERS_DIR, `${name}.md`);
  if (existsSync(file)) {
    console.error(
      `${theme.red}Already exists:${RESET} ${name}\nUse ${theme.subtext0}reminder edit ${name}${RESET} to modify it.`,
    );
    process.exit(1);
  }

  await Bun.write(file, `# ${name}\n\n`);

  const editor = await findEditor();
  if (editor) {
    const proc = Bun.spawn([editor, file], {
      stdin: "inherit",
      stdout: "inherit",
      stderr: "inherit",
    });
    await proc.exited;
  } else {
    console.log(`${theme.green}Created:${RESET} ${file}`);
    console.log(`${theme.overlay0}No editor found — edit manually.${RESET}`);
  }
}

async function removeReminder(name: string): Promise<void> {
  const file = resolveFile(name);
  if (!file) {
    console.error(`${theme.red}Not found:${RESET} ${name}`);
    listReminders();
    process.exit(1);
  }

  rmSync(file);
  console.log(`${theme.green}Removed:${RESET} ${stripExtension(basename(file))}`);
}

async function showReminder(name: string): Promise<void> {
  const file = resolveFile(name);
  if (!file) {
    console.error(`${theme.red}Not found:${RESET} ${name}`);
    listReminders();
    process.exit(1);
  }

  const viewer = await findViewer();
  if (viewer === "bat") {
    await $`bat --style=plain ${file}`;
  } else if (viewer === "glow") {
    await $`glow ${file}`;
  } else {
    const content = await Bun.file(file).text();
    process.stdout.write(content);
  }
}

async function editReminder(name: string): Promise<void> {
  const file = resolveFile(name);
  if (!file) {
    console.error(`${theme.red}Not found:${RESET} ${name}`);
    listReminders();
    process.exit(1);
  }

  const editor = await findEditor();
  if (!editor) {
    console.error(`${theme.red}No editor found.${RESET} Set $EDITOR.`);
    process.exit(1);
  }

  const proc = Bun.spawn([editor, file], {
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
  await proc.exited;
}

/** Detect if the terminal likely supports Nerd Fonts */
function hasNerdFont(): boolean {
  return !!(
    process.env.KITTY_WINDOW_ID ||
    process.env.WEZTERM_EXECUTABLE ||
    process.env.ALACRITTY_SOCKET ||
    process.env.TERM_PROGRAM === "vscode" ||
    process.env.WT_SESSION
  );
}

/**
 * Print a one-line banner for shell startup.
 * Designed to be fast — no subprocesses, no async, just readdir + write.
 */
function printBanner(): void {
  const files = getReminderFiles();
  if (files.length === 0) return;

  const icon = hasNerdFont() ? "\uf435" : "";
  const names = files.map((f) => {
    const name = stripExtension(f);
    const url = fileUrl(join(REMINDERS_DIR, f));
    return hyperlink(url, name);
  });

  const list = names.join(`${theme.overlay0}, ${RESET}`);
  const count = files.length;

  if (count === 1) {
    process.stdout.write(`${theme.lavender} ${icon} ${RESET}${list}\n`);
  } else {
    process.stdout.write(
      `${theme.lavender} ${icon}${RESET} ${theme.peach}${count}${RESET} ${theme.subtext0}reminders:${RESET} ${list}\n`,
    );
  }
}

function printHelp(): void {
  console.log(`${BOLD}${theme.lavender}reminder${RESET} ${theme.subtext0}- Manage ~/reminders/ files${RESET}

${theme.lavender}Usage:${RESET}
  reminder                   List all reminders
  reminder list              List all reminders
  reminder banner            One-line summary for shell startup
  reminder add <name>        Create a new reminder and open in editor
  reminder rm <name>         Remove a reminder
  reminder show <name>       View a reminder (glow > bat > cat)
  reminder edit <name>       Edit a reminder in $EDITOR
  reminder help              Show this help

${theme.lavender}Examples:${RESET}
  reminder add "fish keybindings"
  reminder show fish keybindings
  reminder rm fish keybindings`);
}

async function main(): Promise<void> {
  const args = process.argv.slice(2);
  const subcmd = args[0] ?? "list";
  const rest = args.slice(1).join(" ");

  switch (subcmd) {
    case "list":
    case "ls":
      listReminders();
      break;
    case "banner":
      printBanner();
      break;
    case "add":
    case "new":
      if (!rest) {
        console.error(`${theme.red}Usage:${RESET} reminder add <name>`);
        process.exit(1);
      }
      await addReminder(rest);
      break;
    case "rm":
    case "remove":
    case "delete":
      if (!rest) {
        console.error(`${theme.red}Usage:${RESET} reminder rm <name>`);
        process.exit(1);
      }
      await removeReminder(rest);
      break;
    case "show":
    case "view":
    case "cat":
      if (!rest) {
        console.error(`${theme.red}Usage:${RESET} reminder show <name>`);
        process.exit(1);
      }
      await showReminder(rest);
      break;
    case "edit":
      if (!rest) {
        console.error(`${theme.red}Usage:${RESET} reminder edit <name>`);
        process.exit(1);
      }
      await editReminder(rest);
      break;
    case "help":
    case "--help":
    case "-h":
      printHelp();
      break;
    default:
      // If not a known subcommand, treat entire argv as a show query
      // e.g., `reminder fish commands` → show "fish commands"
      const fullName = args.join(" ");
      const file = resolveFile(fullName);
      if (file) {
        await showReminder(fullName);
      } else {
        console.error(`${theme.red}Unknown command:${RESET} ${subcmd}`);
        printHelp();
        process.exit(1);
      }
  }
}

main().catch((e) => {
  console.error(`${theme.red}Error:${RESET} ${e.message}`);
  process.exit(1);
});
