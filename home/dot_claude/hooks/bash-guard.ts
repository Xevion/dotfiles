// PreToolUse hook: discipline checks + compound command auto-approval.
//
// Phase 1 (Discipline): Parses Bash via shfmt AST, checks each command
// against data-driven rules. Blocks misuse of cat/grep/head/tail/find
// and bad redirect patterns.
//
// Phase 2 (Approval): If discipline passes, checks compound commands
// against Claude Code permission settings. Auto-approves when all
// sub-commands match the allow list, denies when any match the deny list.
//
// Single shfmt parse shared between both phases.
// Exit codes: 0 = allow/fall-through, 2 = block

import { readFileSync } from "fs";
import { dirname } from "path";

const TRUNCATION_LIMIT = 100;

// Prefixes that are transparent — the real command is the next arg
const TRANSPARENT = new Set([
  "command",
  "builtin",
  "env",
  "nice",
  "nohup",
  "sudo",
  "time",
  "doas",
  "exec",
  "strace",
  "ltrace",
  "xargs",
]);

// shfmt binary op codes
const OP_PIPE = 13;
const OP_OR = 14;

// shfmt redirect op codes
const REDIR_OUT = 63; // >
const REDIR_DUP_OUT = 68; // >&

interface CommandCtx {
  name: string; // effective command name (basename, prefix-stripped)
  args: string[]; // all literal arg values
  inPipeline: boolean;
  isRightOfPipe: boolean;
  redirects: Redirect[];
  hasOrFallback: boolean; // part of || chain
}

interface Redirect {
  fd: string; // "2", "1", etc.
  op: number;
  target: string; // "/dev/null", "1", "error.log", etc.
}

type Rule = (ctx: CommandCtx) => string | null;

const rules: Rule[] = [
  (ctx) => {
    if (ctx.name === "grep" && !ctx.isRightOfPipe)
      return (
        "Use the Grep tool instead of the grep command. " +
        "It handles permissions correctly and doesn't require approval."
      );
    return null;
  },

  (ctx) => {
    if (ctx.name === "cat" && !ctx.isRightOfPipe) {
      if (ctx.args.some((a) => a.startsWith("<<"))) return null;
      return (
        "Use the Read tool instead of cat. " +
        "It provides line numbers and supports offset/limit."
      );
    }
    return null;
  },

  (ctx) => {
    if (
      (ctx.name === "head" || ctx.name === "tail") &&
      !ctx.isRightOfPipe
    )
      return "Use the Read tool with offset/limit instead of head/tail on files.";
    return null;
  },

  (ctx) => {
    if (ctx.name === "find" && !ctx.isRightOfPipe) {
      if (ctx.args.some((a) => ["-name", "-iname", "-type"].includes(a)))
        return "Use the Glob tool instead of find. It's faster and doesn't need approval.";
    }
    return null;
  },

  (ctx) => {
    if (!ctx.isRightOfPipe) return null;
    if (ctx.name !== "head" && ctx.name !== "tail") return null;
    const n = parseTruncationN(ctx.args);
    if (n !== null && n <= TRUNCATION_LIMIT) {
      return (
        `Avoid '| ${ctx.name} -${n}' — small truncation values encourage ` +
        "wasteful re-runs. Let the Bash tool handle output naturally, " +
        `or use the Read tool on saved output (limit > ${TRUNCATION_LIMIT}).`
      );
    }
    return null;
  },

  (ctx) => {
    const bad = ctx.redirects.find(
      (r) => r.fd === "2" && r.op === REDIR_OUT && r.target === "/dev/null",
    );
    if (bad)
      return (
        "Do not suppress stderr with 2>/dev/null. " +
        "Error output often contains the diagnosis, and suppression " +
        "makes commands unrecognizable to the permission system."
      );
    return null;
  },

  (ctx) => {
    const bad = ctx.redirects.find(
      (r) => r.fd === "2" && r.op === REDIR_DUP_OUT && r.target === "1",
    );
    if (bad)
      return (
        "Remove '2>&1' — the Bash tool captures both stdout and stderr " +
        "automatically. Adding it is unnecessary and can interfere with " +
        "permission pattern matching."
      );
    return null;
  },

  (ctx) => {
    if (!ctx.hasOrFallback) return null;
    if (ctx.name !== "echo") return null;
    const suppression = /^(not found|NOT FOUND|error|no |none)/;
    if (ctx.args.slice(1).some((a) => suppression.test(a)))
      return (
        "Do not suppress errors with '|| echo ...'. " +
        "Let commands fail naturally so errors are visible."
      );
    return null;
  },
];

function parseTruncationN(args: string[]): number | null {
  for (let i = 1; i < args.length; i++) {
    const a = args[i];
    // -n +N is offset syntax — not truncation, allow it
    if (a === "-n" && args[i + 1]?.startsWith("+")) return null;
    if (a.startsWith("-n+") || a.startsWith("+")) return null;
    if (a === "-n" && i + 1 < args.length) {
      const n = parseInt(args[i + 1], 10);
      return isNaN(n) ? null : n;
    }
    if (a.startsWith("-n") && a.length > 2) {
      const n = parseInt(a.slice(2), 10);
      return isNaN(n) ? null : n;
    }
    if (/^-\d+$/.test(a)) return parseInt(a.slice(1), 10);
  }
  return null;
}

interface AstNode {
  Type?: string;
  Stmts?: AstNode[];
  Cmd?: AstNode;
  Redirs?: AstNode[];
  Args?: AstNode[];
  Parts?: AstNode[];
  Value?: string;
  Op?: number;
  X?: AstNode; // BinaryCmd left
  Y?: AstNode; // BinaryCmd right
  N?: { Value?: string }; // redirect fd
  Word?: AstNode; // redirect target
  Cond?: AstNode[];
  Then?: AstNode[];
  Else?: AstNode;
  Do?: AstNode[];
  Loop?: AstNode;
  Items?: AstNode[];
  Stmt?: AstNode; // TimeClause inner statement
  Assigns?: AstNode[];
  Array?: { Elems?: AstNode[] };
}

function wordValue(word: AstNode | undefined): string {
  if (!word?.Parts) return word?.Value ?? "";
  return word.Parts.map((p) => {
    if (p.Value !== undefined) return p.Value;
    if (p.Type === "DblQuoted" || p.Type === "SglQuoted")
      return p.Parts?.map((pp) => pp.Value ?? "").join("") ?? p.Value ?? "";
    if (p.Type === "ParamExp") return "$" + ((p as any).Param?.Value ?? "");
    if (p.Type === "CmdSubst") return "$(..)";
    return "";
  }).join("");
}

function extractArgs(call: AstNode): string[] {
  return (call.Args ?? []).map(wordValue);
}

function pathBasename(s: string): string {
  const i = s.lastIndexOf("/");
  return i >= 0 ? s.substring(i + 1) : s;
}

function effectiveName(args: string[]): string {
  for (const a of args) {
    const name = pathBasename(a);
    if (!TRANSPARENT.has(name)) return name;
  }
  return args.length > 0 ? pathBasename(args[args.length - 1]) : "";
}

function extractRedirects(stmt: AstNode): Redirect[] {
  return (stmt.Redirs ?? []).map((r) => ({
    fd: r.N?.Value ?? "1",
    op: r.Op ?? 0,
    target: wordValue(r.Word),
  }));
}

interface WalkOpts {
  inPipeline?: boolean;
  isRightOfPipe?: boolean;
  hasOrFallback?: boolean;
  parentRedirects?: Redirect[];
}

function* walkStmt(
  stmt: AstNode,
  opts: WalkOpts = {},
): Generator<CommandCtx> {
  const redirects = [
    ...(opts.parentRedirects ?? []),
    ...extractRedirects(stmt),
  ];
  const cmd = stmt.Cmd ?? stmt;
  yield* walkNode(cmd, { ...opts, parentRedirects: redirects });
}

function* walkNode(
  node: AstNode,
  opts: WalkOpts = {},
): Generator<CommandCtx> {
  if (!node) return;
  const type = node.Type;

  if (type === "CallExpr") {
    const args = extractArgs(node);
    if (args.length === 0) return;
    yield {
      name: effectiveName(args),
      args,
      inPipeline: opts.inPipeline ?? false,
      isRightOfPipe: opts.isRightOfPipe ?? false,
      redirects: opts.parentRedirects ?? [],
      hasOrFallback: opts.hasOrFallback ?? false,
    };
    return;
  }

  if (type === "TimeClause" && node.Stmt) {
    yield* walkStmt(node.Stmt, opts);
    return;
  }

  if (type === "BinaryCmd") {
    const isPipe = node.Op === OP_PIPE;
    const isOr = node.Op === OP_OR;
    if (node.X)
      yield* walkStmt(node.X, {
        ...opts,
        inPipeline: opts.inPipeline || isPipe,
        parentRedirects: opts.parentRedirects,
      });
    if (node.Y)
      yield* walkStmt(node.Y, {
        ...opts,
        inPipeline: opts.inPipeline || isPipe,
        isRightOfPipe: isPipe || opts.isRightOfPipe,
        hasOrFallback: isOr || opts.hasOrFallback,
        parentRedirects: opts.parentRedirects,
      });
    return;
  }

  // Container types — recurse into their statement lists
  const stmtLists: (AstNode[] | undefined)[] = [
    node.Stmts,
    node.Cond,
    node.Then,
    node.Do,
    node.Else?.Stmts ? node.Else.Stmts : node.Else ? [node.Else] : undefined,
  ];
  if (node.Items) {
    for (const item of node.Items) {
      if (item.Stmts) stmtLists.push(item.Stmts);
    }
  }
  for (const list of stmtLists) {
    if (!list) continue;
    for (const s of list) yield* walkStmt(s, opts);
  }

  if (node.Cmd) yield* walkNode(node.Cmd, opts);
}

function parseAst(command: string): AstNode {
  const proc = Bun.spawnSync(["shfmt", "-ln", "bash", "-tojson"], {
    stdin: Buffer.from(command),
    stderr: "pipe",
  });
  if (proc.exitCode !== 0) {
    throw new Error(
      `shfmt failed (exit ${proc.exitCode}): ${proc.stderr.toString().trim()}`,
    );
  }
  return JSON.parse(proc.stdout.toString());
}

// Approval: find the git root for project-level settings
function findGitRoot(): string | null {
  const toplevel = Bun.spawnSync(["git", "rev-parse", "--show-toplevel"], {
    stderr: "pipe",
  });
  if (toplevel.exitCode !== 0) return null;

  // Handle worktrees: common-dir differs from git-dir
  const gitDir = Bun.spawnSync(["git", "rev-parse", "--git-dir"], {
    stderr: "pipe",
  });
  const commonDir = Bun.spawnSync(["git", "rev-parse", "--git-common-dir"], {
    stderr: "pipe",
  });
  if (gitDir.exitCode === 0 && commonDir.exitCode === 0) {
    const gd = gitDir.stdout.toString().trim();
    const cd = commonDir.stdout.toString().trim();
    if (gd !== cd) return dirname(cd);
  }

  return toplevel.stdout.toString().trim();
}

// Extract the command prefix from a Bash(...) permission pattern
function extractBashPrefix(pattern: string): string | null {
  if (!pattern.startsWith("Bash(")) return null;
  let s = pattern.slice(5);
  s = s.replace(/( \*|\*|:\*)\)$/, "");
  s = s.replace(/\)$/, "");
  return s || null;
}

interface PermissionPrefixes {
  allowed: string[];
  denied: string[];
}

function loadPrefixes(): PermissionPrefixes {
  const home = process.env.HOME ?? "";
  const gitRoot = findGitRoot();

  const files = [
    `${home}/.claude/settings.json`,
    `${home}/.claude/settings.local.json`,
  ];
  if (gitRoot) {
    files.push(
      `${gitRoot}/.claude/settings.json`,
      `${gitRoot}/.claude/settings.local.json`,
    );
  } else {
    files.push(".claude/settings.json", ".claude/settings.local.json");
  }

  const allowedSet = new Set<string>();
  const deniedSet = new Set<string>();

  for (const file of files) {
    let data: any;
    try {
      data = JSON.parse(readFileSync(file, "utf-8"));
    } catch {
      continue;
    }
    for (const p of data?.permissions?.allow ?? []) {
      const prefix = extractBashPrefix(p);
      if (prefix !== null) allowedSet.add(prefix);
    }
    for (const p of data?.permissions?.deny ?? []) {
      const prefix = extractBashPrefix(p);
      if (prefix !== null) deniedSet.add(prefix);
    }
  }

  return { allowed: [...allowedSet], denied: [...deniedSet] };
}

// Approval: prefix matching against permission lists
function matchesPrefix(cmd: string, prefix: string): boolean {
  return (
    cmd === prefix ||
    cmd.startsWith(prefix + " ") ||
    cmd.startsWith(prefix + "/")
  );
}

function matchesPrefixList(cmd: string, prefixes: string[]): boolean {
  return prefixes.some((p) => matchesPrefix(cmd, p));
}

// Generate candidate command strings for prefix matching.
// Includes the full arg string and a version with transparent prefixes stripped.
function commandCandidates(ctx: CommandCtx): string[] {
  const full = ctx.args.join(" ");
  const candidates = [full];

  let i = 0;
  while (i < ctx.args.length && TRANSPARENT.has(pathBasename(ctx.args[i])))
    i++;
  if (i > 0 && i < ctx.args.length) {
    candidates.push(ctx.args.slice(i).join(" "));
  }

  return candidates;
}

function checkPermission(
  ctx: CommandCtx,
  prefixes: PermissionPrefixes,
): "allowed" | "denied" | "unknown" {
  const candidates = commandCandidates(ctx);
  for (const c of candidates) {
    if (matchesPrefixList(c, prefixes.denied)) return "denied";
  }
  for (const c of candidates) {
    if (matchesPrefixList(c, prefixes.allowed)) return "allowed";
  }
  return "unknown";
}

// Quick check for shell metacharacters that indicate compound structure.
// Simple commands don't need approval — Claude Code handles them natively.
function hasCompoundStructure(cmd: string): boolean {
  return (
    /[|&;`]/.test(cmd) ||
    cmd.includes("$(") ||
    cmd.includes("<(") ||
    cmd.includes(">(")
  );
}

// Recursively expand sh -c / bash -c into inner commands for approval checking
function expandShellC(contexts: CommandCtx[]): CommandCtx[] {
  const expanded: CommandCtx[] = [];
  for (const ctx of contexts) {
    expanded.push(ctx);
    const base = pathBasename(ctx.args[0] ?? "");
    if (base !== "bash" && base !== "sh") continue;
    const cIdx = ctx.args.indexOf("-c");
    if (cIdx < 0 || cIdx + 1 >= ctx.args.length) continue;
    try {
      const innerAst = parseAst(ctx.args[cIdx + 1]);
      for (const stmt of innerAst.Stmts ?? []) {
        for (const inner of walkStmt(stmt)) expanded.push(inner);
      }
    } catch {
      // Inner parse failure — outer command stays for checking
    }
  }
  return expanded;
}

const ALLOW_JSON = JSON.stringify({
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "allow",
  },
});

interface HookInput {
  tool_name: string;
  tool_input: { command?: string };
}

const input: HookInput = await Bun.stdin.json();
if (input.tool_name !== "Bash") process.exit(0);

const command = input.tool_input?.command;
if (!command) process.exit(0);

let ast: AstNode;
try {
  ast = parseAst(command);
} catch {
  // shfmt unavailable or parse failure — allow through
  process.exit(0);
}

const contexts: CommandCtx[] = [];
for (const stmt of ast.Stmts ?? []) {
  for (const ctx of walkStmt(stmt)) contexts.push(ctx);
}

// Phase 1: Discipline — block bad tool usage patterns
const seen = new Set<string>();
const issues: string[] = [];
for (const ctx of contexts) {
  for (const rule of rules) {
    const msg = rule(ctx);
    if (msg && !seen.has(msg)) {
      seen.add(msg);
      issues.push(msg);
    }
  }
}

if (issues.length > 0) {
  console.error(issues.map((i) => `\u2022 ${i}`).join("\n"));
  process.exit(2);
}

// Phase 2: Approval — auto-approve compound commands when all segments are allowed
if (!hasCompoundStructure(command)) process.exit(0);

const prefixes = loadPrefixes();
if (prefixes.allowed.length === 0) process.exit(0);

const allContexts = expandShellC(contexts);
if (allContexts.length === 0) process.exit(0);

const statuses = allContexts.map((ctx) => checkPermission(ctx, prefixes));

if (statuses.every((s) => s === "allowed")) {
  console.log(ALLOW_JSON);
  process.exit(0);
}

if (statuses.some((s) => s === "denied")) {
  console.error("Compound command contains a denied sub-command.");
  process.exit(2);
}

process.exit(0);
