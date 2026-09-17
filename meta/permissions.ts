#!/usr/bin/env bun

// Permission builder for OpenCode and Claude Code
// Called by chezmoi templates via {{ output "bun" "meta/permissions.ts" "opencode"|"claude" }}
// Outputs JSON fragments to be inserted into each tool's config file.

type Level = "allow" | "ask" | "deny";
type Entry = { pattern: string; level: Level };

const home = process.env.HOME ?? "";

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
    "kitty",
    "vivaldi",
    "jshell",
    "typst",
    "cmake",
    "mpv",
    "ghidra",
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
  "goimports",
  "rustfmt",
  "nasm",
  "vkd3d-compiler",
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
  ...subs("uv", [
    "sync", "run", "venv", "tool", "add", "remove", "build", "init", "lock",
    "export", "tree", "format", "check", "version", "workspace",
    "pip list", "pip show", "pip install", "pip freeze", "pip tree",
    "python list", "python find", "python install",
    "cache dir", "cache size",
  ]),
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
allow("just", "./gradlew");

// Compilers: the only write in an ordinary invocation is the `-o` target (or
// ./a.out), the same mechanism the allowed `cargo build` / `go build` carry.
allow("rustc", "gcc", "clang", "clang-21", "cc");

// `make`/`cmake` run repo-authored recipes, the trust assumption `just` and
// ./gradlew already carry. Targets are listed rather than the bare tool:
// `cmake -E rm -rRf` is a plain recursive delete. Flag-first forms (`make -j4`,
// `make -C dir`) are omitted because the flag precedes the target, so allowing
// one would allow every target after it; `-n` is fine since a dry run is inert.
allow(...subs("make", ["run", "build", "test", "check", "all", "debug", "release", "help", "-n"]));
tool("cmake", {
  allow: ["-B", "-S", "-L", "--build", "--preset", "--list-presets", "-E capabilities", "-E environment"],
});

// Script runtimes: these run caller-supplied code, so the real decision is
// about the code. They are allowed anyway because scripting through them is
// routine here, matching the existing python/bun entries. `bash`, `sh`, `perl`
// and `ruby` stay absent: they are not scripted in here, and `perl -pi -e` /
// `ruby -i -pe` rewrite files in place.
allow("node", "deno");

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
    "switch",
    "clone",
  ],
  ask: [
    // `checkout -- <path>` and `checkout .` destroy uncommitted work with no
    // reflog entry, the same class as `restore`. Branch switching has `switch`.
    "checkout",
    "reset",
    "revert",
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

// General CLI (always safe).
// `xargs` and `env` are deliberately absent: both take a command as their
// argument, so a bare allow on either prefix-matches any command at all and
// silently overrides the rest of this table. The guard hook strips them and
// judges the wrapped command instead.

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
  "du",
  "df",
  "jar",
  "zip",
  "stat",
  "file",
  "exit",
  "printenv",
  "shuf",
  "tr",
  "comm",
  "printf",
  "date",
  "brotli",
  "unrar",
  "innoextract",
  "aria2c",
  "pgrep",
  "blkid",
  "vainfo",
  "avahi-browse",
  "frida-ps",
  "inferno-collapse-guess",
  "virt-xml-validate",
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
allow(
  "nm", "objdump", "ldd", "ilspycmd", "rabin2", "r2", "radare2",
  "readelf", "strings", "hexdump", "addr2line",
  "x86_64-w64-mingw32-addr2line", "minidump-stackwalk",
  // pev suite: every option is an output format or an offset selector
  "peldd", "pescan", "pestr", "pepack", "pehash", "pedis", "readpe",
  // rizin suite, matching the r2 entries above
  "rizin", "rz-bin", "rasm2", "rahash2", "rafind2", "ragg2",
  // wasm: dumps to stdout, or a named -o / --out-dir for the two builders
  "wasm-objdump", "wasm2wat", "wasm-decompile", "wasm-opt", "wasm-bindgen",
);
ask("r2 -w", "radare2 -w", "rizin -w");
// `-a`/`-x`/`-X` write every embedded resource into the cwd as files.
tool("peres", { allow: ["-i", "-l", "-s", "-v", "--info", "--list", "--statistics", "--file-version"] });
// Listing is the default, but `-x -o PATH` writes extracted resources.
tool("wrestool", { allow: ["-l", "--list"] });

// Documents / media (read metadata, or derive a new file; never edit the input)
allow("pdftotext", "pdftoppm", "pdfinfo", "transmission-show", "minizinc");

// Disk usage
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
// `eval` runs arbitrary Typst, `watch` blocks, `update` replaces the binary.
tool("typst", { allow: ["compile", "fonts", "info"] });

// System diagnostics (read-only)
allow("whoami", "uname", "hostname", "id", "groups", "lscpu", "free", "uptime", "nproc", "lsof");

// Network diagnostics
// `ip` and `ss` can mutate/kill state, so only fully-qualified read subcommands
// are allowed here rather than the bare tool (avoids "ip addr:*" wildcarding
// into "ip addr add ...").
allow("ping", "traceroute", "tracepath", "dig", "nslookup", "host", "ss");
// `--show` reports; bare `swapon` and `-a` activate swap devices.
tool("swapon", { allow: ["--show", "-s"] });
// `-w` writes a kernel parameter and `-p` loads a file of them. A bare
// `sysctl <key>` read is indistinguishable by prefix from `sysctl -w`, so only
// the fully-qualified read forms are listed.
tool("sysctl", { allow: ["-a", "-n"] });
// `remove` unpairs a device and `script <file>` runs a command file.
tool("bluetoothctl", { allow: ["list", "show", "devices", "info", "version"] });

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
  // Personal tools deployed by this dotfiles repo or ~/.local/bin.
  // Bash rules match the command text literally, so tools invoked by absolute
  // path need that form listed too - the tilde alone never matches.
  "~/.claude/hooks/guard",
  `${home}/.claude/hooks/guard`,
  "~/.local/bin/protonhax",
  `${home}/.local/bin/protonhax`,
);

// Package managers (ask, lifecycle scripts and lockfile changes)
// `sudo apt` can never actually run here - guard blocks all sudo outright,
// and there's no TTY for a password prompt even if it didn't - but the ask
// entry stays so nothing else accidentally shadows it later.
ask("sudo apt");

// Deploy commands
ask(
  "wrangler publish",
  "wrangler deploy",
  "pnpm run deploy",
  "pnpm run build --production",
  "npm run deploy",
  "bun run deploy",
  "Move-Item",
);

// Cargo dangerous
ask("uv publish"); // PyPI refuses re-upload of a version once it exists
ask("cargo yank", "cargo uninstall --all"); // recoverable (un-yank, reinstall), but affects a public registry / local toolchain
deny("cargo clean"); // not unsafe, just wastes time re-compiling; never run per standing instruction

// Wrangler dangerous
ask("wrangler secret delete"); // recoverable by re-setting the secret
deny("wrangler delete"); // deletes a live Worker, no undo

// Windows destructive
ask("rmdir /s", "rd /s", "Remove-Item -Recurse -Force", "del /s"); // recursive force-delete, no undo

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
    "Skill(brainstorming)",
    "Skill(interview)",
    // /tmp is scratch space, the guard hook saves captured command output
    // under /tmp/claude-guard, and /tmp is generally throwaway.
    // Allow the file tools to operate there without prompting.
    "Read(//tmp/**)",
    "Edit(//tmp/**)",
    // Claude Code's own session/job/debug scratch data, not project content -
    // treat it like /tmp rather than prompting per-file.
    "Read(~/.claude/projects/**)",
    "Edit(~/.claude/projects/**)",
    "Read(~/.claude/jobs/**)",
    "Edit(~/.claude/jobs/**)",
    "Read(~/.claude/debug/**)",
    "Edit(~/.claude/debug/**)",
    // Read-only: these can mirror real project content (pastes, plans), so
    // no blanket Edit.
    "Read(~/.claude/paste-cache/**)",
    "Read(~/.claude/image-cache/**)",
    "Read(~/.claude/plans/**)",
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
