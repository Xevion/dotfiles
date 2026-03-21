#!/usr/bin/env bun

// Permission builder for OpenCode and Claude Code
// Called by chezmoi templates via {{ output "bun" "meta/permissions.ts" "opencode"|"claude" }}
// Outputs JSON fragments to be inserted into each tool's config file.

type Level = "allow" | "ask" | "deny";
type Entry = { pattern: string; level: Level };

const entries: Entry[] = [];

// --- Builder helpers ---

/** Add patterns at a given permission level */
function add(level: Level, ...patterns: string[]) {
  for (const p of patterns) entries.push({ pattern: p, level });
}

const allow = (...p: string[]) => add("allow", ...p);
const ask = (...p: string[]) => add("ask", ...p);
const deny = (...p: string[]) => add("deny", ...p);

/** Expand "base" + ["sub1", "sub2"] into ["base sub1", "base sub2"] */
function subs(base: string, subcommands: string[]): string[] {
  return subcommands.map((s) => `${base} ${s}`);
}

/** Generate "--version" patterns for a list of tool names */
function versions(...tools: string[]): string[] {
  return tools.map((t) => `${t} --version`);
}

/**
 * Define mixed permissions for a tool with subcommands.
 * tool("git", {
 *   allow: ["status", "log", "diff"],
 *   ask:   ["push", "checkout"],
 *   deny:  ["push --force"],
 * })
 */
function tool(
  base: string,
  levels: Partial<Record<Level, string[]>>,
) {
  for (const [level, cmds] of Object.entries(levels)) {
    for (const cmd of cmds!) {
      entries.push({ pattern: `${base} ${cmd}`, level: level as Level });
    }
  }
}

// =============================================================================
// PERMISSIONS
// =============================================================================

// -- Version / info (read-only, always safe) --
allow(
  ...versions(
    "cargo",
    "rustc",
    "sccache",
    "node",
    "npm",
    "pnpm",
    "bun",
    "just",
    "make",
    "git",
    "gh",
    "docker",
    "wrangler",
    "go",
    "python",
    "python3",
    "uv",
    "ruff",
    "mypy",
    "pytest",
    "tsc",
    "eslint",
    "prettier",
    "dotnet",
  ),
  "go version", // go uses "go version" not "go --version"
);

// -- Cargo --
allow(
  ...subs("cargo", [
    "build",
    "check",
    "clippy",
    "test",
    "nextest run",
    "tree",
    "doc",
    "llvm-cov",
    "add",
    "remove",
    "machete",
    "udeps",
    "audit",
    "deny",
    "outdated",
    "search",
    "fmt",
    "metadata",
    "run",
    "bench",
  ]),
);

// -- Go --
allow(
  ...subs("go", [
    "build",
    "test",
    "vet",
    "fmt",
    "mod",
    "list",
    "env",
    "doc",
    "generate",
  ]),
  "gofmt",
  "gopls",
  "golangci-lint",
);

// -- Node / Bun --
allow(
  ...subs("npm", ["run", "audit", "ci", "list", "outdated"]),
  ...subs("pnpm", ["run", "list", "exec", "audit", "outdated", "install", "add", "remove", "uninstall"]),
  "bun",
  "bunx",
);

// -- Python --
allow(
  ...subs("uv", ["sync", "run", "pip list", "pip show"]),
  "pytest",
  "mypy",
  "ruff check",
  "ruff format",
  "black",
);

// -- Build tools --
allow("just", "./gradlew", "make run", "make build");

// -- Git --
tool("git", {
  allow: [
    "status",
    "log",
    "show",
    "diff",
    "fetch",
    "add",
    "commit",
    "rm",
    "mv",
    "restore",
    "ls-tree",
    "ls-files",
    "blame",
    "grep",
    "branch",
    "describe",
    "rev-parse",
    "shortlog",
    "tag",
    "remote -v",
    "config --get",
    "config --list",
    "worktree",
  ],
  ask: [
    "checkout",
    "pull",
    "merge",
    "branch -d",
    "rebase",
    "push",
    "stash",
    "switch",
  ],
  deny: [
    "push --force",
    "push -f",
    "push --force-with-lease",
    "reset --hard",
    "branch -D",
    "clean -fd",
    "clean -f",
    "filter-branch",
    "push --delete",
    "push origin --delete",
    "push origin :",
    "stash drop",
  ],
});

// -- GitHub CLI --
tool("gh", {
  allow: [
    "run list",
    "run view",
    "pr list",
    "pr view",
    "pr create",
    "api",
    "pr comment",
    "issue list",
    "issue view",
    "search",
  ],
  ask: [
    "pr close",
    "issue close",
    "run cancel",
  ],
  deny: [
    "repo delete",
    "repo archive",
    "secret delete",
    "release delete",
  ],
});

// -- Docker --
tool("docker", {
  allow: [
    "ps",
    "logs",
    "inspect",
    "exec",
    "port",
    "build",
    "images",
    "compose up",
    "compose down",
    "compose logs",
  ],
});

// -- General CLI (always safe) --
allow(
  "ls",
  "tree",
  "cat",
  "rg",
  "find",
  "mkdir",
  "touch",
  "mv",
  "cp",
  "cloc",
  "tokei",
  "curl",
  "netstat",
  "awk",
  "timeout",
  "chmod",
  "jq",
  "grep",
  "cd",
  "wc",
  "javap",
  "tee",
  "unzip",
  "rsync",
  "sed",
  "head",
  "tail",
  "diff",
  "sort",
  "uniq",
  "echo",
  "xargs",
  "du",
  "df",
  "jar",
  "zip",
  "stat",
  "file",
  "env",
  "printenv",
  "pwd",
  "which",
  "hyperfine",
  "script",
  "xxd",
  "fc-cache",
  "fd",
  "bat",
);

// -- DB / query tools --
allow("psql", "sqlite3");

// -- Chezmoi --
tool("chezmoi", {
  allow: [
    "status",
    "diff",
    "managed",
    "data",
    "source-path",
    "target-path",
    "cat",
    "execute-template",
    "doctor",
    "dump",
    "verify",
    "help",
    "--help",
    "apply",
  ],
  ask: ["add", "init", "update"],
});

// -- WSL / Windows --
allow(
  "wsl",
  "tasklist",
  "Select-String",
  "Select-Object",
  "findstr",
  "dir",
);

// -- Misc tools --
allow(
  "opencode",
  "mise install",
  "mise exec",
  "claude mcp add",
);

// -- Package managers (ask — lifecycle scripts, lockfile changes) --
ask(
  ...subs("npm", ["install", "update"]),
  ...subs("pnpm", ["update", "store prune"]),
  "cargo uninstall",
  "cargo update",
);

// -- Destructive file operations --
ask("rm", "rm -rf", "del");

// -- Deploy commands --
ask(
  "wrangler publish",
  "wrangler deploy",
  "pnpm run deploy",
  "pnpm run build --production",
  "npm run deploy",
  "bun run deploy",
  "Move-Item",
);

// -- Cargo dangerous --
deny("cargo clean", "cargo yank", "cargo uninstall --all");

// -- Wrangler dangerous --
deny("wrangler delete", "wrangler secret delete");

// -- Windows destructive --
deny("rmdir /s", "rd /s", "Remove-Item -Recurse -Force", "del /s");

// =============================================================================
// OUTPUT FORMATTING
// =============================================================================

type Format = "opencode" | "claude";

function formatOpenCode(): Record<string, string> {
  // OpenCode uses last-match-wins, so order: allow, then ask (overrides), then deny (overrides all)
  const result: Record<string, string> = { "*": "ask" };

  const byLevel: Record<Level, Entry[]> = { allow: [], ask: [], deny: [] };
  for (const e of entries) {
    byLevel[e.level].push(e);
  }

  // Allow first, then ask overrides, then deny overrides
  for (const level of ["allow", "ask", "deny"] as Level[]) {
    for (const e of byLevel[level]) {
      result[`${e.pattern}*`] = level;
    }
  }

  return result;
}

// Claude Code-specific non-bash permissions (WebFetch domains, MCP, built-in tools)
const claudeExtras = {
  allow: [
    // Built-in tools
    "Glob",
    "Read",
    "Grep",
    "WebSearch",
    "WebFetch",
    "Skill(superpowers:*)",
    // MCP servers — context7 (wildcards work for context7 tool names)
    "mcp__context7__resolve-library-id",
    "mcp__context7__query-docs",
    // gh_grep
    "mcp__gh_grep__searchGitHub",
    // Linear: read-only (must be exact names — wildcards not supported in MCP permissions)
    "mcp__linear__get_attachment",
    "mcp__linear__get_document",
    "mcp__linear__get_issue",
    "mcp__linear__get_issue_status",
    "mcp__linear__get_milestone",
    "mcp__linear__get_project",
    "mcp__linear__get_team",
    "mcp__linear__get_user",
    "mcp__linear__list_comments",
    "mcp__linear__list_cycles",
    "mcp__linear__list_documents",
    "mcp__linear__list_issue_labels",
    "mcp__linear__list_issue_statuses",
    "mcp__linear__list_issues",
    "mcp__linear__list_milestones",
    "mcp__linear__list_project_labels",
    "mcp__linear__list_projects",
    "mcp__linear__list_teams",
    "mcp__linear__list_users",
    "mcp__linear__search_documentation",
    "mcp__linear__extract_images",
    // Linear: light mutations (batch-friendly)
    "mcp__linear__save_issue",
    "mcp__linear__create_issue_label",
  ],
  deny: [
    "Task(Explore)",
  ],
  ask: [
    // Linear: one-off mutations that deserve confirmation
    "mcp__linear__save_project",
    "mcp__linear__save_milestone",
    "mcp__linear__save_comment",
    "mcp__linear__delete_comment",
    "mcp__linear__create_attachment",
    "mcp__linear__delete_attachment",
    "mcp__linear__create_document",
    "mcp__linear__update_document",
  ] as string[],
};

function formatClaude(): object {
  const result = {
    allow: [...claudeExtras.allow] as string[],
    deny: [...claudeExtras.deny] as string[],
    ask: [...claudeExtras.ask] as string[],
    defaultMode: "default",
  };

  for (const e of entries) {
    result[e.level as "allow" | "ask" | "deny"].push(`Bash(${e.pattern}:*)`);
  }

  return result;
}

// --- Main ---
const format = process.argv[2] as Format;
if (!format || !["opencode", "claude"].includes(format)) {
  console.error("Usage: permissions.ts <opencode|claude>");
  process.exit(1);
}

const output =
  format === "opencode"
    ? formatOpenCode()
    : formatClaude();

// Output indented JSON (2-space) for embedding in config files
console.log(JSON.stringify(output, null, 2));
