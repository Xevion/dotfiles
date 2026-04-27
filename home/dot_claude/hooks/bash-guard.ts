// PreToolUse hook: discipline checks + compound command auto-approval.
//
// Phase 1 (Discipline): Parses Bash via shfmt AST, checks each command
// against rules. Rules can WARN (surface guidance, command runs) or
// BLOCK (exit 2, command refused).
//
// Phase 2 (Approval): Checks compound commands against Claude Code
// permission settings. Auto-approves when all sub-commands match the
// allow list, denies when any match the deny list.
//
// Single shfmt parse shared between both phases.

import { readFileSync, appendFileSync, mkdirSync, readdirSync, statSync } from "fs";
import { dirname } from "path";
import { createHash } from "crypto";

// Prefixes that are transparent — the real command is the next arg.
// NOTE: `sudo` is intentionally excluded so it surfaces as its own command
// and the sudo-block rule can catch it.
const TRANSPARENT = new Set([
  "command",
  "builtin",
  "env",
  "nice",
  "nohup",
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

// Session state for truncation-bump detection
const STATE_DIR = "/tmp/bash-guard";
const BUMP_WINDOW_MS = 5 * 60 * 1000;

// Filter-pipeline rewrite: redirect expensive commands' output to a file
// when they're piped into truncating filters, so Claude reads the file
// instead of re-running with different filter args.
const REWRITE_DIR = "/tmp/claude-bash";

// Commands whose output is expensive to regenerate. Only these get rewritten.
const EXPENSIVE_COMMANDS = new Set([
  "cargo",
  "rustc",
  "npm",
  "pnpm",
  "yarn",
  "bun",
  "just",
  "make",
  "gradle",
  "mvn",
  "go",
  "pytest",
  "python",
  "python3",
  "uv",
  "node",
  "tsc",
  "eslint",
  "biome",
  "ruff",
  "rspec",
  "jest",
  "vitest",
  "playwright",
  "deno",
  "tox",
  "ctest",
  "cmake",
  "ninja",
]);

// Pipeline filters whose presence indicates Claude is truncating output.
const FILTER_COMMANDS = new Set([
  "head",
  "tail",
  "rg",
  "grep",
  "sed",
  "awk",
  "cut",
  "wc",
  "sort",
  "uniq",
]);

interface CommandCtx {
  name: string;
  args: string[];
  inPipeline: boolean;
  isRightOfPipe: boolean;
  redirects: Redirect[];
  hasOrFallback: boolean;
}

interface Redirect {
  fd: string;
  op: number;
  target: string;
}

type Verdict = "warn" | "block";
interface Issue {
  verdict: Verdict;
  message: string;
}

interface RuleCtx {
  ctx: CommandCtx;
  command: string;
  sessionId: string;
}

type Rule = (rctx: RuleCtx) => Issue | null;

const rules: Rule[] = [
  // BLOCK: sudo never works in this harness
  ({ ctx }) => {
    if (ctx.name === "sudo")
      return {
        verdict: "block",
        message:
          "sudo will not work in this environment. If elevation is genuinely required, tell the user and stop.",
      };
    return null;
  },

  // BLOCK: pipe-to-shell (curl ... | bash)
  ({ ctx }) => {
    if (!ctx.isRightOfPipe) return null;
    if (!["bash", "sh", "zsh", "fish"].includes(ctx.name)) return null;
    // Allow `| bash -c '...'` (explicit script), block bare `| bash`
    if (ctx.args.includes("-c")) return null;
    return {
      verdict: "block",
      message:
        "Do not pipe remote output into a shell. Download, inspect, then execute.",
    };
  },

  // BLOCK: grep command (use Grep tool)
  ({ ctx }) => {
    if (ctx.name === "grep" && !ctx.isRightOfPipe)
      return {
        verdict: "block",
        message:
          "Use the Grep tool instead of the grep command. " +
          "It handles permissions correctly and doesn't require approval.",
      };
    return null;
  },

  // WARN: bare `cat <single-file>` — Read tool is better
  ({ ctx }) => {
    if (ctx.name !== "cat" || ctx.isRightOfPipe) return null;
    if (ctx.redirects.length > 0) return null;
    // args: ["cat", "file"] — exactly one non-flag arg, no heredoc
    if (ctx.args.length !== 2) return null;
    const arg = ctx.args[1];
    if (arg.startsWith("-") || arg.startsWith("<<")) return null;
    return {
      verdict: "warn",
      message:
        "Prefer the Read tool over `cat <file>` — it gives line numbers and supports offset/limit.",
    };
  },

  // WARN: find with name/type filters — Glob is usually nicer
  ({ ctx }) => {
    if (ctx.name !== "find" || ctx.isRightOfPipe) return null;
    if (ctx.args.some((a) => ["-name", "-iname", "-type"].includes(a)))
      return {
        verdict: "warn",
        message:
          "Consider the Glob tool instead of `find -name/-type` — faster and no approval needed.",
      };
    return null;
  },

  // WARN: 2>/dev/null suppression (except common feature-detection patterns)
  ({ ctx }) => {
    const bad = ctx.redirects.find(
      (r) => r.fd === "2" && r.op === REDIR_OUT && r.target === "/dev/null",
    );
    if (!bad) return null;
    // Allow feature-detection: `command -v X 2>/dev/null`, `which X 2>/dev/null`
    if (["command", "which", "type", "hash"].includes(ctx.name)) return null;
    return {
      verdict: "warn",
      message:
        "`2>/dev/null` hides the diagnosis when things break. Prefer letting errors surface.",
    };
  },
];

function parseTruncationN(args: string[]): number | null {
  for (let i = 1; i < args.length; i++) {
    const a = args[i];
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

// Session-aware truncation-bump detection.
// Fires only when the same base command is re-run with an INCREASED N
// within the bump window. Same-N re-runs (check→fix→check loop) pass silently.
function checkTruncationBump(
  command: string,
  contexts: CommandCtx[],
  sessionId: string,
): Issue | null {
  // Find the rightmost head/tail context that is right-of-pipe
  let truncCtx: CommandCtx | null = null;
  for (const ctx of contexts) {
    if (!ctx.isRightOfPipe) continue;
    if (ctx.name !== "head" && ctx.name !== "tail") continue;
    truncCtx = ctx;
  }
  if (!truncCtx) return null;
  const n = parseTruncationN(truncCtx.args);
  if (n === null) return null;

  // Base = everything left of the final `|`
  const lastPipe = command.lastIndexOf("|");
  if (lastPipe < 0) return null;
  const base = command.substring(0, lastPipe).trim();
  if (!base) return null;

  const prior = readSessionHistory(sessionId);
  const now = Date.now();
  const recent = prior.filter(
    (e) => e.base === base && now - e.t < BUMP_WINDOW_MS,
  );

  recordCommand(sessionId, { t: now, base, n, cmd: truncCtx.name });

  const prevMax = recent.reduce((m, e) => Math.max(m, e.n), 0);
  if (recent.length > 0 && n > prevMax) {
    return {
      verdict: "warn",
      message:
        `Re-running the same command with a larger '| ${truncCtx.name} -${n}' ` +
        `(previous: -${prevMax}). If you need more output, use the Bash tool's ` +
        `natural output, or save to a file and use Read with offset/limit.`,
    };
  }
  return null;
}

interface HistoryEntry {
  t: number;
  base: string;
  n: number;
  cmd: string;
}

function sessionStatePath(sessionId: string): string {
  return `${STATE_DIR}/${sessionId}.jsonl`;
}

function readSessionHistory(sessionId: string): HistoryEntry[] {
  try {
    const raw = readFileSync(sessionStatePath(sessionId), "utf-8");
    return raw
      .split("\n")
      .filter(Boolean)
      .map((l) => {
        try {
          return JSON.parse(l) as HistoryEntry;
        } catch {
          return null;
        }
      })
      .filter((e): e is HistoryEntry => e !== null);
  } catch {
    return [];
  }
}

function recordCommand(sessionId: string, entry: HistoryEntry): void {
  try {
    mkdirSync(STATE_DIR, { recursive: true });
    appendFileSync(sessionStatePath(sessionId), JSON.stringify(entry) + "\n");
  } catch {
    // Best-effort; state loss just means a missed bump detection
  }
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
  X?: AstNode;
  Y?: AstNode;
  N?: { Value?: string };
  Word?: AstNode;
  Cond?: AstNode[];
  Then?: AstNode[];
  Else?: AstNode;
  Do?: AstNode[];
  Loop?: AstNode;
  Items?: AstNode[];
  Stmt?: AstNode;
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

function findGitRoot(): string | null {
  const toplevel = Bun.spawnSync(["git", "rev-parse", "--show-toplevel"], {
    stderr: "pipe",
  });
  if (toplevel.exitCode !== 0) return null;

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

// Strip trailing redirects from a command string before prefix matching.
// `bq query foo 2>&1` should still match `Bash(bq query:*)`.
function stripTrailingRedirects(cmd: string): string {
  return cmd
    .replace(/\s+(?:\d*>>?&?\d*|\d*<)\s*\S+\s*$/g, "")
    .replace(/\s+(?:\d*>>?&?\d*|\d*<)\s*\S+\s*$/g, "")
    .trim();
}

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

function commandCandidates(ctx: CommandCtx): string[] {
  const full = ctx.args.join(" ");
  const candidates = [full, stripTrailingRedirects(full)];

  let i = 0;
  while (i < ctx.args.length && TRANSPARENT.has(pathBasename(ctx.args[i])))
    i++;
  if (i > 0 && i < ctx.args.length) {
    const stripped = ctx.args.slice(i).join(" ");
    candidates.push(stripped, stripTrailingRedirects(stripped));
  }

  return [...new Set(candidates)];
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

function hasCompoundStructure(cmd: string): boolean {
  return (
    /[|&;`]/.test(cmd) ||
    cmd.includes("$(") ||
    cmd.includes("<(") ||
    cmd.includes(">(")
  );
}

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

// Find the first top-level `|` in the command string, respecting quotes,
// escapes, and nesting in (), {}, []. Returns -1 if none found.
// Skips `||` (logical-or).
function findFirstTopLevelPipe(s: string): number {
  let parenDepth = 0;
  let braceDepth = 0;
  let bracketDepth = 0;
  let inSingle = false;
  let inDouble = false;
  let inBacktick = false;
  let escape = false;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (escape) {
      escape = false;
      continue;
    }
    if (c === "\\" && !inSingle) {
      escape = true;
      continue;
    }
    if (inSingle) {
      if (c === "'") inSingle = false;
      continue;
    }
    if (inDouble) {
      if (c === '"') inDouble = false;
      continue;
    }
    if (inBacktick) {
      if (c === "`") inBacktick = false;
      continue;
    }
    if (c === "'") {
      inSingle = true;
      continue;
    }
    if (c === '"') {
      inDouble = true;
      continue;
    }
    if (c === "`") {
      inBacktick = true;
      continue;
    }
    if (c === "(") {
      parenDepth++;
      continue;
    }
    if (c === ")") {
      if (parenDepth > 0) parenDepth--;
      continue;
    }
    if (c === "{") {
      braceDepth++;
      continue;
    }
    if (c === "}") {
      if (braceDepth > 0) braceDepth--;
      continue;
    }
    if (c === "[") {
      bracketDepth++;
      continue;
    }
    if (c === "]") {
      if (bracketDepth > 0) bracketDepth--;
      continue;
    }
    if (
      c === "|" &&
      parenDepth === 0 &&
      braceDepth === 0 &&
      bracketDepth === 0
    ) {
      if (s[i + 1] === "|") {
        i++;
        continue;
      }
      return i;
    }
  }
  return -1;
}

interface RewriteDecision {
  base: string;
  rewritten: string;
  path: string;
}

// Detect: top-level pipeline where leftmost is an expensive command and
// at least one downstream segment is a truncating filter. If matched,
// produce a rewrite that runs the base command with output redirected to
// a hash-keyed file under REWRITE_DIR.
function detectFilterRewrite(
  command: string,
  ast: AstNode,
): RewriteDecision | null {
  const stmts = ast.Stmts ?? [];
  if (stmts.length !== 1) return null;
  const stmt = stmts[0];

  // Reject if the top-level statement carries its own redirects — the user
  // is already routing output, don't second-guess.
  if ((stmt.Redirs ?? []).length > 0) return null;

  const cmd = stmt.Cmd ?? stmt;
  if (cmd.Type !== "BinaryCmd" || cmd.Op !== OP_PIPE) return null;

  // Walk left-to-right collecting CallExpr leaves of the pipeline.
  const segments: { name: string; node: AstNode; redirs: AstNode[] }[] = [];
  function collect(node: AstNode | undefined): boolean {
    if (!node) return false;
    const innerStmt = node;
    const inner = innerStmt.Cmd ?? innerStmt;
    if (inner.Type === "BinaryCmd" && inner.Op === OP_PIPE) {
      return collect(inner.X) && collect(inner.Y);
    }
    if (inner.Type === "CallExpr") {
      const args = extractArgs(inner);
      if (args.length === 0) return false;
      segments.push({
        name: effectiveName(args),
        node: inner,
        redirs: innerStmt.Redirs ?? [],
      });
      return true;
    }
    return false;
  }
  if (!collect(cmd)) return null;
  if (segments.length < 2) return null;

  const base = segments[0];
  if (!EXPENSIVE_COMMANDS.has(base.name)) return null;

  // If the base segment redirects stdout to a file, the wrap would
  // double-redirect and lose output. Bail.
  for (const r of base.redirs) {
    if (r.Op === REDIR_OUT) {
      const target = wordValue(r.Word);
      // Allow stderr-only: `2> file` (fd === "2"). Reject any 1> or default-fd `>`.
      const fd = r.N?.Value ?? "1";
      if (fd === "1") return null;
    }
  }

  // At least one downstream segment must be a truncating filter.
  const hasFilter = segments
    .slice(1)
    .some((s) => FILTER_COMMANDS.has(s.name));
  if (!hasFilter) return null;

  // Slice base from the original command at the first top-level pipe.
  const pipeIdx = findFirstTopLevelPipe(command);
  if (pipeIdx < 0) return null;
  const baseStr = command.substring(0, pipeIdx).trim();
  if (!baseStr) return null;

  const hash = createHash("sha256").update(baseStr).digest("hex").slice(0, 12);
  const path = `${REWRITE_DIR}/${hash}.log`;

  // Rewritten command — uses only allowed primitives (mkdir, stat, echo, exit).
  // Brace group ensures `> path 2>&1` applies to the whole base regardless
  // of any inner `2>&1` it carried.
  const rewritten =
    `mkdir -p ${REWRITE_DIR}; ` +
    `{ ${baseStr}; } > ${path} 2>&1; ` +
    `ec=$?; ` +
    `sz=$(stat -c%s ${path}); ` +
    `echo "[bash-guard] saved: ${path} ($sz bytes, exit $ec). Use the Read tool with offset/limit, or Grep, to inspect — do not re-run with different filter args."; ` +
    `exit $ec`;

  return { base: baseStr, rewritten, path };
}

interface HookInput {
  tool_name: string;
  tool_input: { command?: string };
  session_id?: string;
}

const input: HookInput = await Bun.stdin.json();
if (input.tool_name !== "Bash") process.exit(0);

const command = input.tool_input?.command;
if (!command) process.exit(0);

const sessionId = input.session_id ?? "unknown";

let ast: AstNode;
try {
  ast = parseAst(command);
} catch {
  process.exit(0);
}

const contexts: CommandCtx[] = [];
for (const stmt of ast.Stmts ?? []) {
  for (const ctx of walkStmt(stmt)) contexts.push(ctx);
}

// Phase 1: Discipline
const seen = new Set<string>();
const issues: Issue[] = [];

function pushIssue(i: Issue | null) {
  if (!i) return;
  if (seen.has(i.message)) return;
  seen.add(i.message);
  issues.push(i);
}

for (const ctx of contexts) {
  for (const rule of rules) {
    pushIssue(rule({ ctx, command, sessionId }));
  }
}

pushIssue(checkTruncationBump(command, contexts, sessionId));

const blocks = issues.filter((i) => i.verdict === "block");
const warns = issues.filter((i) => i.verdict === "warn");

if (blocks.length > 0) {
  const all = [
    ...blocks.map((i) => `\u2022 ${i.message}`),
    ...warns.map((i) => `\u2022 (warn) ${i.message}`),
  ];
  console.error(all.join("\n"));
  process.exit(2);
}

// Phase 1.5: Filter-pipeline rewrite. If matched, substitute the command
// in-memory. Approval (Phase 2) checks the BASE command's contexts \u2014 the
// wrapper primitives (mkdir, brace group, assignments, stat, echo, exit)
// are injected by us and trusted by construction; they don't need to be
// in the user's allow list.
const rewrite = detectFilterRewrite(command, ast);
let effectiveCommand = command;
let effectiveContexts = contexts;
let rewriteContext: string | undefined;
let updatedInput: { command: string } | undefined;

if (rewrite) {
  effectiveCommand = rewrite.rewritten;
  updatedInput = { command: rewrite.rewritten };
  rewriteContext =
    `[bash-guard] Rewrote pipeline: '${rewrite.base}' was piped into truncating filters. ` +
    `Output redirected to ${rewrite.path}. ` +
    `Use the Read tool with offset/limit, or Grep, to inspect \u2014 do NOT re-run with different filter args. ` +
    `If the same base command runs again, output goes to the same path.`;
  try {
    const baseAst = parseAst(rewrite.base);
    effectiveContexts = [];
    for (const stmt of baseAst.Stmts ?? []) {
      for (const ctx of walkStmt(stmt)) effectiveContexts.push(ctx);
    }
  } catch {
    // If the base somehow fails to parse, drop the rewrite and fall
    // through with the original command.
    effectiveCommand = command;
    effectiveContexts = contexts;
    updatedInput = undefined;
    rewriteContext = undefined;
  }
}

// Phase 2: Approval
function emit(opts: {
  permissionDecision?: "allow";
  additionalContext?: string;
  updatedInput?: { command: string };
}): void {
  const out: any = {
    hookSpecificOutput: { hookEventName: "PreToolUse" },
  };
  if (opts.permissionDecision)
    out.hookSpecificOutput.permissionDecision = opts.permissionDecision;
  if (opts.additionalContext)
    out.hookSpecificOutput.additionalContext = opts.additionalContext;
  if (opts.updatedInput)
    out.hookSpecificOutput.updatedInput = opts.updatedInput;
  console.log(JSON.stringify(out));
}

const warnLines = warns.map((i) => `\u2022 ${i.message}`);
const contextParts: string[] = [];
if (rewriteContext) contextParts.push(rewriteContext);
if (warnLines.length > 0) contextParts.push(warnLines.join("\n"));
const additionalContext =
  contextParts.length > 0 ? contextParts.join("\n\n") : undefined;

function emitFinal(opts: { permissionDecision?: "allow" } = {}): void {
  if (!additionalContext && !updatedInput && !opts.permissionDecision) return;
  emit({
    permissionDecision: opts.permissionDecision,
    additionalContext,
    updatedInput,
  });
}

if (!hasCompoundStructure(effectiveCommand)) {
  emitFinal();
  process.exit(0);
}

const prefixes = loadPrefixes();
if (prefixes.allowed.length === 0) {
  emitFinal();
  process.exit(0);
}

const allContexts = expandShellC(effectiveContexts);
if (allContexts.length === 0) {
  emitFinal();
  process.exit(0);
}

const statuses = allContexts.map((ctx) => checkPermission(ctx, prefixes));

if (statuses.every((s) => s === "allowed")) {
  emitFinal({ permissionDecision: "allow" });
  process.exit(0);
}

if (statuses.some((s) => s === "denied")) {
  console.error("Compound command contains a denied sub-command.");
  process.exit(2);
}

emitFinal();
process.exit(0);
