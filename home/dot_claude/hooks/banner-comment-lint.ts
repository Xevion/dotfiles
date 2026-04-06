// PostToolUse hook: detect banner/decorative comments and AI anti-patterns.
//
// Fires after Write or Edit tool calls. Scans written content for:
// - Banner-style comment patterns (decorative dividers, box-drawing)
// - Bare section-label comments (// Handlers, // Utilities)
// - Refactoring/history comments (// Previously used..., // Migrated from...)
//
// Reports matches to Claude via stderr with file:line context.
// Exit codes: 0 = clean, 2 = issues found

const EXCLUDED_SUFFIXES = new Set([".md", ".lock", ".svg", ".snap"]);
const EXCLUDED_PARTS = [
  "node_modules",
  "migrations",
  ".min.",
  "vendor",
  "generated",
];
const EXCLUDED_FILENAMES = new Set(["gradlew", "gradlew.bat"]);

function isExcluded(filePath: string): boolean {
  const lastSlash = filePath.lastIndexOf("/");
  const fileName =
    lastSlash >= 0 ? filePath.substring(lastSlash + 1) : filePath;
  if (EXCLUDED_FILENAMES.has(fileName)) return true;

  const lastDot = fileName.lastIndexOf(".");
  if (lastDot >= 0 && EXCLUDED_SUFFIXES.has(fileName.substring(lastDot)))
    return true;

  return EXCLUDED_PARTS.some((part) => filePath.includes(part));
}

type Category = "banner" | "bare-label" | "history";

interface Match {
  line: number;
  text: string;
  category: Category;
}

const BANNER_PATTERNS: RegExp[] = [
  // comment prefix followed by 5+ repeated symbol chars
  /^\s*(?:\/\/[\/!]?|\/\*+|#|--|<!--|@rem|::|\*)\s*[-=*#~+._^─═\/]{5,}/m,
  // standalone line of 10+ repeated symbol chars
  /^\s*[-=*#~+._^─═\/]{10,}/m,
  // symbols wrapping a label on both sides
  /(?:\/\/[\/!]?|#|--|\/\*+|<!--)\s*[-=*#~+._^─═]{2,}\s+\S.{0,60}\s*[-=*#~+._^─═]{2,}/m,
  // trailing symbols before block-comment close
  /[-=*#~+._^─═\/]{5,}\s*(?:\*\/|-->)/m,
  // box-drawing unicode (3+ consecutive)
  /[╔╗╚╝║│┌┐└┘├┤┬┴┼]{3,}/m,
  // symbols around bracketed text
  /(?:\/\/[\/!]?|#|--|\/\*+|<!--)\s*[-=*#~+._^─═]{1,}\s*\[.{1,60}\]\s*[-=*#~+._^─═]{2,}/m,
];

// Bare section labels: comment containing only a generic category word.
// These add no information — if context is needed, write a real explanation.
const BARE_LABELS = new Set([
  "handlers",
  "handler",
  "utilities",
  "utility",
  "utils",
  "util",
  "helpers",
  "helper",
  "private",
  "public",
  "protected",
  "internal",
  "constants",
  "config",
  "configuration",
  "types",
  "interfaces",
  "imports",
  "exports",
  "methods",
  "functions",
  "properties",
  "variables",
  "state",
  "components",
  "styles",
  "tests",
  "setup",
  "cleanup",
  "init",
  "initialization",
  "main",
  "api",
  "routes",
  "middleware",
  "models",
  "views",
  "controllers",
  "services",
  "repositories",
  "definitions",
  "declarations",
  "enums",
  "structs",
  "classes",
  "traits",
  "implementations",
  "constructors",
  "getters",
  "setters",
  "callbacks",
  "listeners",
  "observers",
  "factories",
  "validators",
  "formatters",
  "parsers",
  "serializers",
  "converters",
  "transformers",
  "mappers",
  "resolvers",
  "actions",
  "reducers",
  "selectors",
  "mutations",
  "queries",
  "subscriptions",
  "hooks",
  "providers",
  "context",
  "effects",
  "signals",
  "stores",
  "computed",
]);

// Match a comment line containing only 1-2 words
const BARE_LABEL_LINE =
  /^\s*(?:\/\/[\/!]?\s*|#\s*|\/\*\*?\s*|\*\s*)([\w]+(?:\s+[\w]+)?)\s*(?:\*\/)?\s*$/;

// Refactoring/history comments that belong in commit messages, not code
const HISTORY_PATTERN =
  /^\s*(?:\/\/[\/!]?|#|\/\*\*?|\*)\s*(?:Refactored|Previously|Migrated|Was formerly|Changed from|Replaced|Old approach|Removed|Used to|Originally|This (?:function|method|class|module) was)/i;

function findIssues(content: string): Match[] {
  const lines = content.split("\n");
  const seen = new Set<number>();
  const matches: Match[] = [];

  const add = (lineNo: number, text: string, category: Category) => {
    if (!seen.has(lineNo)) {
      seen.add(lineNo);
      matches.push({ line: lineNo, text, category });
    }
  };

  // Banner patterns: scan full content with global matching
  for (const pattern of BANNER_PATTERNS) {
    const global = new RegExp(pattern.source, pattern.flags + "g");
    for (const m of content.matchAll(global)) {
      const lineNo = content.substring(0, m.index!).split("\n").length;
      add(lineNo, lines[lineNo - 1]?.trimEnd() ?? m[0], "banner");
    }
  }

  // Line-by-line patterns
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const lineNo = i + 1;

    // Bare section labels
    const labelMatch = BARE_LABEL_LINE.exec(line);
    if (labelMatch) {
      const words = labelMatch[1].toLowerCase();
      const allBare = words
        .split(/\s+/)
        .every((w) => BARE_LABELS.has(w));
      if (allBare) {
        add(lineNo, line.trimEnd(), "bare-label");
      }
    }

    // History/refactoring comments
    if (HISTORY_PATTERN.test(line)) {
      add(lineNo, line.trimEnd(), "history");
    }
  }

  return matches.sort((a, b) => a.line - b.line);
}

const CATEGORY_MESSAGES: Record<Category, string> = {
  banner:
    "Remove decorative dividers. If an adjacent label adds context, keep it as a plain comment.",
  "bare-label":
    "Remove bare section-label comments (// Handlers, // Utils). " +
    "If context is needed, use a doc comment that explains purpose, not just a category name.",
  history:
    "Remove refactoring/history comments (// Previously used..., // Migrated from...). " +
    "Code history belongs in commit messages, not inline comments.",
};

interface HookInput {
  tool_name: string;
  tool_input: { content?: string; new_string?: string; file_path?: string };
}

let input: HookInput;
try {
  input = await Bun.stdin.json();
} catch {
  process.exit(0);
}

if (input.tool_name !== "Write" && input.tool_name !== "Edit")
  process.exit(0);

const filePath = input.tool_input?.file_path ?? "";
if (isExcluded(filePath)) process.exit(0);

const content =
  input.tool_name === "Write"
    ? input.tool_input?.content
    : input.tool_input?.new_string;
if (!content) process.exit(0);

const issues = findIssues(content);
if (issues.length === 0) process.exit(0);

const locationNote =
  input.tool_name === "Edit"
    ? " (line numbers relative to replacement text)"
    : "";

// Group by category for clear feedback
const categories = [...new Set(issues.map((i) => i.category))] as Category[];
const out: string[] = [`Comment lint issues in ${filePath}${locationNote}:`];

for (const cat of categories) {
  const catIssues = issues.filter((i) => i.category === cat);
  for (const issue of catIssues.slice(0, 5)) {
    out.push(`  ${filePath}:${issue.line}: ${issue.text.trim()}`);
  }
  if (catIssues.length > 5) {
    out.push(`  ... and ${catIssues.length - 5} more`);
  }
  out.push("");
  out.push(CATEGORY_MESSAGES[cat]);
}

out.push("");
out.push(
  "If this is a false positive (generated file, intentional delimiter), disregard.",
);

console.error(out.join("\n"));
process.exit(2);
