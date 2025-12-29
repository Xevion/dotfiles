/**
 * Chezmoi Safety Plugin for OpenCode
 *
 * Blocks unsafe chezmoi commands:
 * - Commands with --force/-f flags
 * - Commands without specific file arguments (chezmoi apply with no files)
 */

interface PluginInput {
  project: unknown;
  client: unknown;
  $: unknown;
  directory: string;
  worktree: string;
}

interface ToolExecuteInput {
  tool: string;
  sessionID: string;
  callID: string;
}

interface ToolExecuteOutput {
  args: {
    command?: string;
    [key: string]: unknown;
  };
}

export const ChezmoiSafety = async (_ctx: PluginInput) => {
  return {
    "tool.execute.before": async (
      input: ToolExecuteInput,
      output: ToolExecuteOutput
    ): Promise<void> => {
      // Only validate Bash commands
      if (input.tool.toLowerCase() !== "bash") {
        return;
      }

      const command = output.args.command ?? "";

      // Only validate chezmoi commands
      if (!/\bchezmoi\b/.test(command)) {
        return;
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
        throw new Error(errors.join("\n"));
      }
    },
  };
};
