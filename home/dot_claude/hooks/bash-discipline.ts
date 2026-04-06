// PreToolUse hook: enforce bash command discipline via shfmt AST analysis.
//
// Parses Bash commands into a structured AST using shfmt, then checks each
// command segment against a data-driven rule set. This avoids regex false
// positives (patterns inside strings) and false negatives (commands behind
// prefixes like sudo/env/command or via absolute paths).
//
// Exit codes:  0 = allow,  2 = block (stderr shown to Claude as feedback)

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
      // Allow heredoc usage (cat <<EOF) — that's writing, not reading
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

// Parse -n N, -N, -n+N (offset, allowed) from head/tail args
function parseTruncationN(args: string[]): number | null {
  for (let i = 1; i < args.length; i++) {
    const a = args[i];
    // -n +N is offset syntax — not truncation, allow it
    if (a === "-n" && args[i + 1]?.startsWith("+")) return null;
    if (a.startsWith("-n+") || a.startsWith("+")) return null;
    // -n N
    if (a === "-n" && i + 1 < args.length) {
      const n = parseInt(args[i + 1], 10);
      return isNaN(n) ? null : n;
    }
    // -nN (combined)
    if (a.startsWith("-n") && a.length > 2) {
      const n = parseInt(a.slice(2), 10);
      return isNaN(n) ? null : n;
    }
    // -N (bare number)
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
    if (p.Type === "ParamExp") return "$" + (p as any).Param?.Value ?? "";
    if (p.Type === "CmdSubst") return "$(..)";
    return "";
  }).join("");
}

function extractArgs(call: AstNode): string[] {
  return (call.Args ?? []).map(wordValue);
}

function basename(s: string): string {
  const i = s.lastIndexOf("/");
  return i >= 0 ? s.substring(i + 1) : s;
}

function effectiveName(args: string[]): string {
  for (const a of args) {
    const name = basename(a);
    if (!TRANSPARENT.has(name)) return name;
  }
  return args.length > 0 ? basename(args[args.length - 1]) : "";
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

  // TimeClause wraps a Stmt — recurse into it transparently
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

  // Container types — recurse into their statements
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

  // If none of the above matched but there's a Cmd, walk it
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


interface HookInput {
  tool_name: string;
  tool_input: { command?: string };
}

const input: HookInput = await Bun.stdin.json();
if (input.tool_name !== "Bash") process.exit(0);

const command = input.tool_input?.command;
if (!command) process.exit(0);

let issues: string[];

try {
  const ast = parseAst(command);
  const contexts: CommandCtx[] = [];
  for (const stmt of ast.Stmts ?? []) {
    for (const ctx of walkStmt(stmt)) {
      contexts.push(ctx);
    }
  }

  // Deduplicate: same message shouldn't appear twice
  const seen = new Set<string>();
  issues = [];
  for (const ctx of contexts) {
    for (const rule of rules) {
      const msg = rule(ctx);
      if (msg && !seen.has(msg)) {
        seen.add(msg);
        issues.push(msg);
      }
    }
  }
} catch {
  // shfmt unavailable or parse failure — allow through rather than block
  process.exit(0);
}

if (issues.length > 0) {
  console.error(issues.map((i) => `• ${i}`).join("\n"));
  process.exit(2);
}
