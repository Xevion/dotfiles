#!/usr/bin/env bun
/**
 * Chezmoi Safety Hook - Prevents unsafe chezmoi commands in Claude Code
 *
 * Blocks:
 * - Commands with --force/-f flags
 * - Commands without specific file arguments (chezmoi apply with no files)
 */

interface ToolInput {
  command?: string;
  description?: string;
}

interface HookInput {
  tool_name: string;
  tool_input: ToolInput;
}

async function readStdin(): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of Bun.stdin.stream()) {
    chunks.push(Buffer.from(chunk));
  }
  return Buffer.concat(chunks).toString("utf-8");
}

async function main() {
  let inputData: HookInput;

  try {
    const input = await readStdin();
    inputData = JSON.parse(input);
  } catch {
    // Invalid JSON, allow command to proceed
    process.exit(0);
  }

  const toolName = inputData.tool_name ?? "";
  const toolInput = inputData.tool_input ?? {};
  const command = toolInput.command ?? "";

  // Only validate Bash commands
  if (toolName !== "Bash") {
    process.exit(0);
  }

  // Only validate chezmoi commands
  if (!/\bchezmoi\b/.test(command)) {
    process.exit(0);
  }

  const errors: string[] = [];

  // Check for force flags
  if (/\b(?:--force|-f)\b/.test(command)) {
    errors.push(
      "Force flag detected: chezmoi commands with --force or -f are not allowed"
    );
    errors.push(
      "   Reason: Force flag bypasses safety checks and can overwrite changes"
    );
  }

  // Check for apply/update without specific files
  if (/\bchezmoi\s+(?:apply|update)\b/.test(command)) {
    const match = command.match(/\bchezmoi\s+(?:apply|update)\s+(.*)/);
    if (match) {
      let args = match[1].trim();
      // Remove known flags to see if file arguments remain
      args = args.replace(/(?:--[\w-]+|-[a-z])\b/g, "").trim();

      if (!args) {
        errors.push(
          "No specific files specified: 'chezmoi apply' must target specific files"
        );
        errors.push("   Allowed: chezmoi apply ~/.bashrc");
        errors.push(
          "   Allowed: chezmoi apply --dry-run ~/.config/fish/config.fish"
        );
        errors.push("   Blocked: chezmoi apply");
        errors.push("   Blocked: chezmoi apply --dry-run");
        errors.push("");
        errors.push(
          "   Use 'chezmoi diff' to preview changes before asking me to apply"
        );
      }
    }
  }

  // If errors found, block the command
  if (errors.length > 0) {
    process.stderr.write(errors.join("\n") + "\n");
    process.exit(2); // Exit code 2 blocks the command
  }

  // No issues, allow command
  process.exit(0);
}

main();
