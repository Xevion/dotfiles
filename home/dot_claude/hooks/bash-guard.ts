// PreToolUse hook: discipline checks + truncation rewrite + auto-approval.
//
// Phase 1 (Discipline): Parses Bash via shfmt AST, checks each command
// against rules. Rules can WARN (surface guidance, command runs) or
// BLOCK (exit 2, command refused).
//
// Phase 1.5 (Truncation rewrite): When a command's output is piped into
// head/tail, rewrite that pipeline so the FULL output streams to a file
// (via tee) while only a bounded preview shows inline. Claude can no longer
// hide output behind a small `-n`; the complete output is always on disk.
//
// Phase 2 (Approval): Checks compound commands against Claude Code
// permission settings. Auto-approves when all sub-commands match the
// allow list, denies when any match the deny list.
//
// Single shfmt parse shared between all phases.

import { readFileSync, mkdirSync, renameSync } from "fs";
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

// Truncation rewrite: when a command's output is piped into head/tail, the
// pipeline is rewritten to stream the FULL output to a file (via tee) while
// showing a bounded live preview. Claude's requested N is ignored — we always
// capture everything so output can never be hidden behind a small `-n`.
const BUFFER_DIR = "/tmp/claude-bash";

// Inline preview sizes. Show the first N lines live as they stream; when the
// output exceeds that, also surface the last few lines plus the total line
// count and a pointer to the saved file.
const PREVIEW_HEAD = 50;
const PREVIEW_TAIL = 5;

// The compiled hook binary, which doubles as the `--preview` stdin filter the
// truncation rewrite pipes into. Matches the path wired in settings.json.
const SELF_BIN = `${process.env.HOME ?? "~"}/.claude/hooks/bash-guard`;

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

  // BLOCK: git stash — hard ban, no exceptions
  ({ ctx }) => {
    if (ctx.name !== "git") return null;
    const firstNonFlag = ctx.args.slice(1).find((a) => !a.startsWith("-"));
    if (firstNonFlag !== "stash") return null;
    return {
      verdict: "block",
      message:
        "STOP. `git stash` is BANNED in this environment. Do NOT use it. Do NOT try to work around this with `git stash push`, `git stash save`, plumbing equivalents, or any other variation. This is not a transient error, not a permission glitch, not something to retry — it is a HARD RULE the user set deliberately. Do NOT touch the user's git state to work around it: no commits, no branches, no resets, no checkouts, no `git add`, nothing. Leave the working tree exactly as it is and pick a different approach that doesn't involve modifying git state at all. If you genuinely cannot proceed without touching git, STOP and ask the user.",
    };
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

  // WARN: find with name/type filters — fd/rg are usually nicer
  ({ ctx }) => {
    if (ctx.name !== "find" || ctx.isRightOfPipe) return null;
    if (ctx.args.some((a) => ["-name", "-iname", "-type"].includes(a)))
      return {
        verdict: "warn",
        message:
          "Consider `fd` or `rg --files` instead of `find -name/-type` — faster and respects .gitignore.",
      };
    return null;
  },

  // WARN: `rg -rn`-style flag bundles — grep muscle memory that silently mangles output.
  // ripgrep is recursive by default and `-r` is `--replace`, so `-rn` is parsed as
  // "replace each match with the text 'n'", not "recursive + line numbers".
  ({ ctx }) => {
    if (ctx.name !== "rg") return null;
    for (const a of ctx.args) {
      if (a === "--") break;
      if (!/^-r[a-zA-Z]+$/.test(a)) continue;
      const repl = a.slice(2);
      return {
        verdict: "warn",
        message:
          `\`rg ${a}\` is grep muscle memory: ripgrep searches recursively by default, and \`-r\` is \`--replace\`. ` +
          `\`${a}\` makes ripgrep print each match with the matched text replaced by "${repl}" instead of showing line numbers ` +
          `(e.g. "UPDATE ALL FROM" becomes "${repl} ALL FROM"). ` +
          "For line numbers use `rg -n` (recursion is automatic); to actually replace, use `rg --replace=TEXT`.",
      };
    }
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

interface AstNode {
  Type?: string;
  Pos?: { Offset?: number };
  End?: { Offset?: number };
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
  asked: string[];
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
  const askedSet = new Set<string>();

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
    for (const p of data?.permissions?.ask ?? []) {
      const prefix = extractBashPrefix(p);
      if (prefix !== null) askedSet.add(prefix);
    }
  }

  return {
    allowed: [...allowedSet],
    denied: [...deniedSet],
    asked: [...askedSet],
  };
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

// Verbs of `xevion projects content` that only edit the long-form body of a
// portfolio entry. Their PM-JSON payloads begin with `{"`, which trips Claude
// Code's built-in brace-quote ("expansion obfuscation") heuristic and forces a
// manual prompt on every insert/replace even though `Bash(xevion:*)` already
// allows them. `set` is intentionally excluded — it replaces the entire
// document and is deliberately gated behind an `ask` rule.
const XEVION_SAFE_CONTENT_VERBS = new Set([
  "list",
  "get",
  "insert",
  "replace",
  "rm",
  "move",
]);

// True for a single `xevion projects content <safe-verb>` call the user has
// neither denied nor gated behind `ask`. Such calls get an explicit hook allow
// to suppress the brace-quote prompt. We re-check the settings deny/ask lists
// here so this carve-out can never override an explicit restriction, regardless
// of how Claude Code orders hook decisions against permission rules.
function isSafeXevionContent(contexts: CommandCtx[]): boolean {
  if (contexts.length !== 1) return false;
  const ctx = contexts[0];

  let i = 0;
  while (i < ctx.args.length && TRANSPARENT.has(pathBasename(ctx.args[i]))) i++;
  const argv = ctx.args.slice(i);

  if (pathBasename(argv[0] ?? "") !== "xevion") return false;
  if (argv[1] !== "projects" || argv[2] !== "content") return false;
  if (!XEVION_SAFE_CONTENT_VERBS.has(argv[3] ?? "")) return false;

  const prefixes = loadPrefixes();
  const candidates = commandCandidates(ctx);
  if (candidates.some((c) => matchesPrefixList(c, prefixes.denied)))
    return false;
  if (candidates.some((c) => matchesPrefixList(c, prefixes.asked)))
    return false;

  return true;
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

// True when a pipeline's final segment is a `head`/`tail` that truncates piped
// stdin. Returns the command name, or null when it should be left alone:
// following mode (`-f`), or reading an explicit file argument (not stdin).
function truncKind(cmd: AstNode): "head" | "tail" | null {
  const args = extractArgs(cmd);
  const name = effectiveName(args);
  if (name !== "head" && name !== "tail") return null;

  let expectValue = false;
  for (let i = 1; i < args.length; i++) {
    const a = args[i];
    if (expectValue) {
      expectValue = false;
      continue;
    }
    if (a === "-f" || a === "--follow" || a.startsWith("--follow")) return null;
    if (a === "-n" || a === "-c" || a === "--lines" || a === "--bytes") {
      expectValue = true;
      continue;
    }
    if (a.startsWith("-")) continue; // `-5`, `-n50`, `--lines=5`, `--`
    return null; // a bare argument is a file operand → reads from disk, not stdin
  }
  return name;
}

interface RewriteTarget {
  start: number; // byte offset of the head/tail segment to replace
  end: number;
}

// Find every pipeline whose final segment truncates with head/tail, anywhere it
// sits in command position — a bare statement, or an operand of `;`/`&&`/`||`,
// or inside a compound body. Pipelines feeding a downstream consumer
// (`cmd | head | grep`) are skipped: rewriting them would corrupt that stdin.
function collectRewriteTargets(ast: AstNode): RewriteTarget[] {
  const targets: RewriteTarget[] = [];

  function visitStmt(stmt: AstNode | undefined): void {
    if (!stmt) return;
    visitCmd(stmt.Cmd ?? stmt, stmt);
  }

  function visitCmd(cmd: AstNode | undefined, stmt: AstNode): void {
    if (!cmd) return;

    if (cmd.Type === "BinaryCmd") {
      if (cmd.Op === OP_PIPE) {
        // Final segment of `X | Y` is Y (pipes are left-associative, so the
        // outermost pipe node's Y is the last command in the chain).
        const yCmd = cmd.Y?.Cmd ?? cmd.Y;
        const kind =
          yCmd?.Type === "CallExpr" ? truncKind(yCmd) : null;
        // Replace ONLY the final `head/tail` segment [Y.Pos, Y.End); LEFT and
        // the `|` are left verbatim.
        const ys = cmd.Y?.Pos?.Offset;
        const ye = cmd.Y?.End?.Offset;
        if (kind && ys !== undefined && ye !== undefined) {
          targets.push({ start: ys, end: ye });
        }
        // Do not descend into X (it is the captured base) or Y (terminal).
        return;
      }
      // Logical `&&` / `||`: both operands run in command position.
      visitStmt(cmd.X);
      visitStmt(cmd.Y);
      return;
    }

    // Compound bodies (subshells, blocks, if/for/while, case) hold statement
    // lists whose members are themselves in command position.
    for (const list of [cmd.Stmts, cmd.Cond, cmd.Then, cmd.Do]) {
      if (list) for (const s of list) visitStmt(s);
    }
    if (cmd.Else?.Stmts) for (const s of cmd.Else.Stmts) visitStmt(s);
    else if (cmd.Else) visitStmt(cmd.Else);
    if (cmd.Items)
      for (const it of cmd.Items)
        if (it.Stmts) for (const s of it.Stmts) visitStmt(s);
    if (cmd.Cmd) visitCmd(cmd.Cmd, stmt);
  }

  for (const stmt of ast.Stmts ?? []) visitStmt(stmt);
  return targets;
}

interface RewriteResult {
  command: string;
  count: number;
}

// Swap every head/tail segment for the bare `--preview` filter — the whole
// point is that the rewritten command reads as `LEFT | …bash-guard --preview`.
// Replacements run right-to-left so earlier byte offsets stay valid. The filter
// picks (and creates) its own output path, so nothing else is injected.
function applyRewrites(
  command: string,
  targets: RewriteTarget[],
): RewriteResult {
  const ordered = [...targets].sort((a, b) => b.start - a.start);
  let out = command;
  for (const t of ordered) {
    out = out.slice(0, t.start) + `${SELF_BIN} --preview` + out.slice(t.end);
  }
  return { command: out, count: targets.length };
}

// `--preview [logPath]` mode: the stdin filter the truncation rewrite pipes
// into. Streams the first PREVIEW_HEAD lines live, tees the complete stream to a
// file, keeps a ring buffer of the last PREVIEW_TAIL, then prints a footer
// (total lines, saved path, last lines). Exits 0, mirroring how a real
// head/tail terminates the pipeline.
//
// logPath is optional: when omitted (the production rewrite passes no path), the
// stream is content-hashed and saved as `<hash>.log` so identical output reuses
// one file instead of piling up. An explicit path (for testing) is used as-is.
async function previewMode(logPath?: string): Promise<never> {
  const writePath = logPath ?? `${BUFFER_DIR}/.pending-${process.pid}.log`;
  try {
    mkdirSync(dirname(writePath), { recursive: true });
  } catch {
    // Best-effort; the writer below surfaces any real failure.
  }
  const sink = Bun.file(writePath).writer();
  const hash = logPath ? null : createHash("sha256");
  const decoder = new TextDecoder();

  let total = 0;
  let headPrinted = 0;
  const ring: string[] = new Array(PREVIEW_TAIL);
  let pending = "";

  const onLine = (line: string) => {
    total++;
    if (headPrinted < PREVIEW_HEAD) {
      process.stdout.write(line + "\n");
      headPrinted++;
    }
    ring[(total - 1) % PREVIEW_TAIL] = line;
  };

  for await (const chunk of Bun.stdin.stream()) {
    sink.write(chunk); // exact tee of the raw bytes
    hash?.update(chunk);
    pending += decoder.decode(chunk, { stream: true });
    let nl: number;
    while ((nl = pending.indexOf("\n")) >= 0) {
      onLine(pending.slice(0, nl));
      pending = pending.slice(nl + 1);
    }
  }
  pending += decoder.decode();
  if (pending.length > 0) onLine(pending); // trailing line without a newline
  await sink.end();

  let finalPath = writePath;
  if (hash) {
    finalPath = `${BUFFER_DIR}/${hash.digest("hex").slice(0, 12)}.log`;
    try {
      renameSync(writePath, finalPath);
    } catch {
      finalPath = writePath;
    }
  }
  if (total > PREVIEW_HEAD) {
    process.stdout.write(
      `[bash-guard] ${total} total lines; showed first ${PREVIEW_HEAD}. ` +
        `full output: ${finalPath}. last ${PREVIEW_TAIL} lines:\n`,
    );
    for (let i = total - PREVIEW_TAIL; i < total; i++) {
      process.stdout.write((ring[i % PREVIEW_TAIL] ?? "") + "\n");
    }
  } else {
    process.stdout.write(
      `[bash-guard] ${total} line(s); full output: ${finalPath}\n`,
    );
  }
  process.exit(0);
}

interface HookInput {
  tool_name: string;
  tool_input: { command?: string };
  session_id?: string;
}

// Filter mode: `bash-guard --preview [logPath]` reads piped output rather than
// a hook payload. Dispatch before touching stdin as JSON.
if (process.argv[2] === "--preview") {
  await previewMode(process.argv[3]);
}

const input: HookInput = await Bun.stdin.json();
if (input.tool_name !== "Bash") process.exit(0);

const command = input.tool_input?.command;
if (!command) process.exit(0);

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
    pushIssue(rule({ ctx, command }));
  }
}

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

// Phase 1.5: Truncation rewrite. When the command pipes into head/tail, swap
// that final segment for our `--preview` filter, which streams the full output
// to a file. Approval (Phase 2) runs against the ORIGINAL command's
// sub-commands, so when the original was approvable the rewrite is auto-approved
// too; the user is never re-prompted just because output is being preserved.
const targets = collectRewriteTargets(ast);
let effectiveCommand = command;
let rewriteContext: string | undefined;
let updatedInput: { command: string } | undefined;

if (targets.length > 0) {
  const rewrite = applyRewrites(command, targets);
  effectiveCommand = rewrite.command;
  updatedInput = { command: rewrite.command };
  const plural = rewrite.count === 1 ? "pipeline" : "pipelines";
  rewriteContext =
    `[bash-guard] Swapped head/tail for the preview filter in ${rewrite.count} ` +
    `${plural}: the full output now streams to a file while only the first ` +
    `${PREVIEW_HEAD} lines (plus a footer with the total line count and last ` +
    `${PREVIEW_TAIL} lines) show inline. The exact saved-file path is printed in ` +
    `the [bash-guard] footer \u2014 read it with the Read tool (offset/limit) or rg; ` +
    `do NOT re-run with a different -n/-N to see more.`;
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

// Auto-approve safe `xevion projects content` body edits. These are single,
// non-compound commands, so without this they fall straight through to Claude
// Code's permission flow and hit the built-in brace-quote prompt on every
// PM-JSON payload. Runs before the generic compound handling below.
if (isSafeXevionContent(contexts)) {
  emitFinal({ permissionDecision: "allow" });
  process.exit(0);
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

const allContexts = expandShellC(contexts);
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
