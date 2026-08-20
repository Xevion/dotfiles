#!/usr/bin/env bun

// Permission builder for OpenCode and Claude Code
// Called by chezmoi templates via {{ output "bun" "meta/permissions.ts" "opencode"|"claude" }}
// Outputs JSON fragments to be inserted into each tool's config file.

type Level = "allow" | "ask" | "deny";
type Entry = { pattern: string; level: Level };

const entries: Entry[] = [];

// Builder helpers

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

// PERMISSIONS

// Version / info (read-only, always safe)
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
    "mise",
    "fish",
  ),
  "go version", // go uses "go version" not "go --version"
);

// Cargo
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
    "info",
    "install",
    "update",
    "uninstall",
  ]),
);
tool("rustup", { allow: ["show", "component", "target", "toolchain"] });

// Go
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
    "run",
  ]),
  "gofmt",
  "gofumpt",
  "gopls",
  "golangci-lint",
  "go get",
  "go install",
);
tool("mvn", { allow: ["test", "compile", "package", "verify", "clean package", "clean install", "dependency:tree", "assembly:single"] });

// Node / Bun
allow(
  ...subs("npm", ["run", "audit", "ci", "list", "outdated", "info", "view", "show", "install", "update"]),
  ...subs("pnpm", ["run", "list", "exec", "audit", "outdated", "install", "add", "remove", "uninstall", "update", "store prune"]),
  "bun",
  "bunx",
  "npx",
  "eslint",
  "prettier",
  "tsc",
  "dprint",
);

// Python
allow(
  ...subs("uv", ["sync", "run", "pip list", "pip show", "pip install", "venv", "tool", "add", "build", "init"]),
  "python3",
  "python",
  ".venv/bin/python",
  "pytest",
  "mypy",
  "ruff check",
  "ruff format",
  "black",
  "uvx",
);
tool("pyenv", { allow: ["install", "versions", "version", "global", "local", "which"] });
tool("pipenv", { allow: ["run", "sync", "install", "lock"] });

// Build tools
allow("just", "./gradlew", "make run", "make build");

// Git
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
    "reflog",
    "cherry -v",
    "clean -n",
    "clean --dry-run",
    "stash list",
    "stash show",
    "checkout",
    "switch",
    "clone",
  ],
  ask: [
    "pull",
    "merge",
    "rebase",
    "push",
    "stash",
    "restore",
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

// GitHub CLI
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
    "repo view",
    "repo list",
    "release list",
    "release view",
    "workflow list",
    "workflow view",
    "gist list",
    "gist view",
    "label list",
    "pr diff",
    "pr checks",
    "run cancel",
  ],
  ask: [
    "pr close",
    "issue close",
    "repo archive",
    "secret delete",
    "release delete",
  ],
  deny: [
    "repo delete", // no recovery path once GitHub's grace window lapses
  ],
});

// Docker
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
    "rm",
  ],
});

// `timeout`/`flock` are absent on purpose: guard's TRANSPARENT list
// evaluates the wrapped command against these rules instead of bypassing.

// General CLI (always safe)
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
  "exit",
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
  "tar",
  "java",
  "journalctl",
  "whois",
  "apt-cache",
  "7z",
  "ln",
  "eza",
  "ps",
  "perf",
  "xz",
  "ast-grep",
  "gzip",
  "zstd",
  "ffmpeg",
  "convert",
  "magick",
  "xmllint",
  "protoc",
  "shellcheck",
  "shfmt",
  "typos",
  "zizmor",
  "actionlint",
  "lychee",
  "ktlint",
  "dust",
  "pkg-config",
  "nvidia-smi",
  "lspci",
  "lsusb",
);
// ast-grep footgun: `-U`/`--update-all` rewrites every match across the codebase unattended
ask("ast-grep run -U", "ast-grep run --update-all", "ast-grep scan --update-all");

// DB / query tools
allow("psql", "sqlite3", "sqlc", "tygo", "mysql", "duckdb");
// redis-cli mixes read commands with destructive ones (FLUSHALL, DEL, SET) at
// the same argument position, so only the read verbs are allowed.
tool("redis-cli", { allow: ["GET", "KEYS", "TTL", "TYPE", "INFO", "PING", "DBSIZE"] });

// Binary inspection (read-only unless -w is passed to patch)
allow("nm", "objdump", "ldd", "ilspycmd", "rabin2", "r2", "radare2");
ask("r2 -w", "radare2 -w");

// Disk usage
allow("dust");
tool("dua", { allow: ["aggregate"] }); // `dua interactive` has an in-TUI delete key

// Process management
allow("kill", "pkill");

// systemd (status queries only; start/stop/enable/disable stay ungated)
tool("systemctl", {
  allow: ["is-active", "list-unit-files", "list-units", "status", "--user list-unit-files", "--user list-units", "--user show-environment", "--user status"],
});

// Deployment CLIs (read-only status checks; up/deploy stay ungated)
tool("railway", { allow: ["status", "whoami"] });

// Privileged reads: dmidecode (SMBIOS) and kernelstub -p (print boot
// options) only read, no state change despite needing root. `pkexec bash`/
// `pkexec cat <path>` stay ungated on purpose - privileged arbitrary
// exec/read is the same class sudo is blocked for, not a permission tweak.
tool("pkexec", { allow: ["dmidecode", "kernelstub -p"] });

// Virtualization / system inspection (read-only status/info queries; the
// state-changing virsh verbs - destroy/undefine/console/start/shutdown -
// stay ungated)
tool("virsh", {
  allow: [
    "list",
    "dumpxml",
    "domstate",
    "capabilities",
    "domcapabilities",
    "nodeinfo",
    "net-list",
    "domiflist",
    "domblklist",
  ],
});
tool("qemu-img", { allow: ["info"] });

// Nix (builds into the immutable store; no project mutation). `nix run`
// executes an arbitrary flake output and `nix develop` (bare) drops into an
// interactive shell - same class as npx/bash, so both stay ungated except
// the non-interactive `develop --command` form actually used here.
tool("nix", {
  allow: [
    "build",
    "eval",
    "flake check",
    "flake show",
    "flake metadata",
    "flake archive",
    "log",
    "search",
    "profile list",
    "profile diff-closures",
    "develop --command",
    "flake init",
    "flake new",
    "profile install",
  ],
  ask: [
    "flake update",
    "flake lock",
    "profile remove",
    "profile upgrade",
    "profile wipe-history",
  ],
});
tool("nix-store", { allow: ["-q", "--query"] });

// Cloud CLIs: only the read-only describe/list/get surface, never
// create/apply/delete.
tool("aws", { allow: ["sts get-caller-identity", "configure list", "s3 ls"] });
tool("gcloud", { allow: ["config list", "projects list"] });

// Static site generators / codegen (scoped to the project's own output dir)
tool("hugo", { allow: ["build", "server", "list", "config", "env"], ask: ["deploy"] });
tool("zola", { allow: ["build", "serve", "check"] });
tool("typst", { allow: ["compile"] });

// System diagnostics (read-only)
allow("whoami", "uname", "hostname", "id", "groups", "lscpu", "free", "uptime", "nproc", "lsof");

// Network diagnostics
// `ip` and `ss` can mutate/kill state, so only fully-qualified read subcommands
// are allowed here rather than the bare tool (avoids "ip addr:*" wildcarding
// into "ip addr add ...").
allow("ping", "traceroute", "tracepath", "dig", "nslookup", "host", "ss");
tool("ip", {
  allow: ["addr show", "route show", "route get", "link show", "-s link", "neigh show", "rule show"],
});

// Package listing (read-only queries only; install/remove untouched)
tool("apt", { allow: ["list", "search", "show", "policy"] });
tool("dpkg", { allow: ["-l", "-L", "-s", "-S", "--list", "--listfiles"] });
tool("brew", { allow: ["list", "info", "search", "outdated", "leaves", "deps", "--version"] });
tool("snap", { allow: ["list", "info", "find", "install", "remove"] });

// xevion (xevion.dev content CLI)
// Authoring is iterative and runs against production by design (edits are atomic
// and cheap to revise), so the read + content-mutation surface is allowed wholesale.
// Only the irreversibly-destructive verbs are gated. Node deletion (`content rm`)
// is ordinary authoring and stays allowed.
allow("xevion");
tool("xevion", {
  // Reversible-but-clobbering: prompt rather than run blind.
  ask: [
    "projects content set", // replaces the entire detail document
    "targets rm", // removes a configured API target
    "logout", // revokes the server-side session + clears the token
  ],
  // Purely destructive, no undo: never auto-run.
  deny: [
    "projects delete", // deletes an entire project
  ],
});

// Chezmoi
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
tool("doppler", { allow: ["configs", "projects"] });

// WSL / Windows
allow(
  "wsl",
  "tasklist",
  "Select-String",
  "Select-Object",
  "findstr",
  "dir",
);

// Misc tools
allow(
  ...versions("code", "zed", "micro"),
  "code --list-extensions",
  "ssh-keygen -l", // fingerprint only, no key material touched
  "openssl version",
  "openssl x509", // cert inspection, no key generation
  "opencode",
  "mise install",
  "mise exec",
  "mise ls",
  "mise ls-remote",
  "mise registry",
  "mise trust",
  "mise use",
  "mise run",
  "claude mcp add",
  "tempo",
  "ffprobe",
  "looking-glass-client",
  // Custom local tooling on external storage
  "/mnt/storage/unity/bin/unity",
  "/mnt/storage/unity/bin/pcommit",
  // Personal tools deployed by this dotfiles repo or ~/.local/bin
  "~/.claude/hooks/guard",
  "~/.local/bin/protonhax",
);

// Package managers (ask, lifecycle scripts and lockfile changes)
// `sudo apt` can never actually run here - guard blocks all sudo outright,
// and there's no TTY for a password prompt even if it didn't - but the ask
// entry stays so nothing else accidentally shadows it later.
ask("sudo apt");

// Destructive file operations
ask("rm", "rm -rf", "del");

// Deploy commands
ask(
  "wrangler publish",
  "wrangler deploy",
  "pnpm run deploy",
  "pnpm run build --production",
  "npm run deploy",
  "bun run deploy",
  "Move-Item",
  "ssh",
  "scp",
);

// Cargo dangerous
ask("cargo yank", "cargo uninstall --all"); // recoverable (un-yank, reinstall), but affects a public registry / local toolchain
deny("cargo clean"); // not unsafe, just wastes time re-compiling; never run per standing instruction

// Wrangler dangerous
ask("wrangler secret delete"); // recoverable by re-setting the secret
deny("wrangler delete"); // deletes a live Worker, no undo

// Windows destructive
ask("rmdir /s", "rd /s", "Remove-Item -Recurse -Force", "del /s"); // Windows equivalents of `rm -rf`, which is already ask

// ripgrep footgun: `-r`/`--replace` is substitution, not "recursive"
// (ported `grep -rn` silently rewrites matches instead of printing line numbers)
deny("rg -r", "rg --replace");

// OUTPUT FORMATTING

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
    "Skill(brainstorming)",
    "Skill(interview)",
    // /tmp is scratch space, the guard hook saves captured command output
    // under /tmp/claude-guard, and /tmp is generally throwaway.
    // Allow the file tools to operate there without prompting.
    "Read(/tmp/**)",
    "Edit(/tmp/**)",
    // Dependency source/cache directories - read the actual source of installed
    // packages (registries, module caches, extracted stores) without prompting.
    "Read(~/.cargo/registry/**)", // crates.io source + index
    "Read(~/.cargo/git/**)", // git-sourced crate checkouts
    "Read(~/go/pkg/mod/**)", // Go module cache (verified via `go env GOMODCACHE`)
    "Read(~/.npm/**)",
    "Read(~/.local/share/pnpm/store/**)", // pnpm content-addressable store (verified via `pnpm store path`)
    "Read(~/.cache/yarn/**)",
    "Read(~/.bun/install/cache/**)",
    "Read(~/.cache/pip/**)",
    "Read(~/.cache/uv/**)", // verified via `uv cache dir`
    "Read(~/.local/share/uv/**)", // uv-managed tool venvs
    "Read(~/.gem/**)",
    "Read(~/.m2/repository/**)",
    "Read(~/.gradle/**)",
    "Read(~/.local/share/mise/installs/**)", // mise-managed toolchains (ruby gems, python site-packages, etc. live here on this machine)
    // MCP servers: context7 (wildcards work for context7 tool names)
    "mcp__context7__resolve-library-id",
    "mcp__context7__query-docs",
    // gh_grep
    "mcp__gh_grep__searchGitHub",
    // reverse-engineering MCPs (read-only inspection tools; no rename/patch/write tools exist)
    "mcp__ida__*",
    "mcp__ida-pro-mcp__*",
    "mcp__ghidra__*",
    // rustdoc-mcp (crate/item docs lookup)
    "mcp__rustdoc-mcp__*",
    // local-web-fetch (read-only URL fetch, same trust level as built-in WebFetch)
    "mcp__local-web-fetch__*",
    // ark-ui (component doc/example lookup)
    "mcp__ark-ui__*",
    // Linear: read-only (must be exact names, wildcards not supported in MCP permissions)
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
    // Linear: light mutations (batch-friendly), unblocked, run without prompting
    "mcp__linear__save_issue",
    "mcp__linear__create_issue_label",
    "mcp__linear__save_comment",
    "mcp__linear__create_attachment",
    "mcp__linear__create_document",
    "mcp__linear__update_document",
  ],
  deny: [
    "Task(Explore)",
  ],
  ask: [
    // Linear: destructive / heavier mutations that deserve confirmation
    "mcp__linear__save_project",
    "mcp__linear__save_milestone",
    "mcp__linear__delete_comment",
    "mcp__linear__delete_attachment",
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
