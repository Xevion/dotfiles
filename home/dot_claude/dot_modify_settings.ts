// Worker for chezmoi modify_settings.json — deep-merges managed keys into
// Claude Code settings. Receives current target on stdin, outputs merged JSON.
// Preserves keys Claude Code adds at runtime (e.g. skipDangerousModePermissionPrompt).

import { resolve } from "path";

const sourceDir = process.argv[2];
if (!sourceDir) {
  console.error("Usage: .modify_settings.ts <chezmoi-source-dir>");
  process.exit(1);
}
const permissionsScript = resolve(sourceDir, "../meta/permissions.ts");

// Read current target file from stdin
const stdin = await Bun.stdin.text();
const current = stdin.trim() ? JSON.parse(stdin) : {};

// Generate permissions block
const permProc = Bun.spawnSync(["bun", permissionsScript, "claude"]);
if (permProc.exitCode !== 0) {
  console.error("Failed to generate permissions:", permProc.stderr.toString());
  process.exit(1);
}
const permissions = JSON.parse(permProc.stdout.toString());

// Desired state — keys chezmoi manages. Anything not here is left as-is.
const desired: Record<string, unknown> = {
  includeCoAuthoredBy: false,
  permissions,
  hooks: {
    PreToolUse: [
      {
        matcher: "Bash",
        hooks: [
          {
            type: "command",
            command: "~/.claude/hooks/guard hook",
            timeout: 3000,
          },
        ],
      },
    ],
    PostToolUse: [
      {
        matcher: "Write|Edit",
        hooks: [
          {
            type: "command",
            command: "~/.claude/hooks/banner-comment-lint",
          },
        ],
      },
      {
        matcher: "Skill",
        hooks: [
          {
            type: "command",
            command: "bunx -y ccstatusline@latest --hook",
          },
        ],
      },
    ],
    UserPromptSubmit: [
      {
        hooks: [
          {
            type: "command",
            command: "bunx -y ccstatusline@latest --hook",
          },
        ],
      },
    ],
  },
  statusLine: {
    type: "command",
    command: "bunx -y ccstatusline@latest",
    padding: 0,
  },
  enabledPlugins: {
    "rust-analyzer-lsp@claude-plugins-official": false,
    "gopls-lsp@claude-plugins-official": false,
    "code-simplifier@claude-plugins-official": true,
    "ralph-loop@claude-plugins-official": true,
    "claude-md-management@claude-plugins-official": true,
  },
  alwaysThinkingEnabled: true,
  voiceEnabled: true,
  disabledMcpjsonServers: [
    "claude.ai Linear",
    "claude.ai Cloudflare Developer Platform",
    "claude.ai Google Calendar",
    "claude.ai Gmail",
  ],
};

// Merge: desired keys overwrite current, current-only keys preserved
const merged = { ...current, ...desired };

console.log(JSON.stringify(merged, null, 2));
