#!/usr/bin/env bun

// MCP server definitions — single source of truth for Claude, OpenCode, Codex.
// Called by chezmoi templates:
//   {{ output "bun" "meta/mcp-servers.ts" "<format>" <context7_key> <exa_key> }}
// Secrets are passed as argv so Doppler values only cross process boundary once.

type Format = "opencode" | "claude" | "codex";

const [, , format, context7Key = "", exaKey = ""] = process.argv as [
  string,
  string,
  Format,
  string?,
  string?,
];

if (!format || !["opencode", "claude", "codex"].includes(format)) {
  console.error("Usage: mcp-servers.ts <opencode|claude|codex> [context7_key] [exa_key]");
  process.exit(1);
}

type Server = {
  url: string;
  headers?: Record<string, string>;
  oauth?: boolean;
  only?: Format[];
  tools?: Record<string, Record<string, string>>;
};

const allServers: Record<string, Server> = {
  context7: {
    url: "https://mcp.context7.com/mcp",
    headers: { CONTEXT7_API_KEY: context7Key },
  },
  gh_grep: {
    url: "https://mcp.grep.app",
  },
  // exa: opencode-only — Claude/Codex have built-in web search
  exa: {
    url: `https://mcp.exa.ai/mcp?exaApiKey=${exaKey}`,
    only: ["opencode"],
  },
  linear: {
    url: "https://mcp.linear.app/mcp",
    oauth: true,
    tools: {
      save_issue: { approval_mode: "approve" },
      save_project: { approval_mode: "approve" },
      save_comment: { approval_mode: "approve" },
    },
  },
};

const servers: Record<string, Server> = Object.fromEntries(
  Object.entries(allServers).filter(
    ([, s]) => !s.only || s.only.includes(format),
  ),
);

function formatOpenCode(): unknown {
  const out: Record<string, unknown> = {};
  for (const [name, s] of Object.entries(servers)) {
    const block: Record<string, unknown> = { type: "remote", url: s.url };
    if (s.headers) block.headers = s.headers;
    if (s.oauth) block.oauth = {};
    out[name] = block;
  }
  return out;
}

function formatClaude(): unknown {
  // Shape: { required: { name: {...} }, managed: [all managed names] }
  // `managed` is the full name set this script owns — the modify script
  // prunes any managed name not in `required` so disabled servers don't linger.
  // Claude's MCP schema has no OAuth field; linear uses its own flow.
  const required: Record<string, unknown> = {};
  for (const [name, s] of Object.entries(servers)) {
    const block: Record<string, unknown> = { type: "http", url: s.url };
    if (s.headers) block.headers = s.headers;
    required[name] = block;
  }
  return { required, managed: Object.keys(allServers) };
}

// Codex TOML output — emits [mcp_servers.<name>] blocks, newline-separated.
function formatCodex(): string {
  const lines: string[] = [];
  for (const [name, s] of Object.entries(servers)) {
    lines.push(`[mcp_servers.${name}]`);
    lines.push(`url = ${JSON.stringify(s.url)}`);
    if (s.headers) {
      const pairs = Object.entries(s.headers)
        .map(([k, v]) => `${k} = ${JSON.stringify(v)}`)
        .join(", ");
      lines.push(`headers = { ${pairs} }`);
    }
    if (s.tools) {
      for (const [toolName, toolConfig] of Object.entries(s.tools)) {
        lines.push("");
        lines.push(`[mcp_servers.${name}.tools.${toolName}]`);
        for (const [k, v] of Object.entries(toolConfig)) {
          lines.push(`${k} = ${JSON.stringify(v)}`);
        }
      }
    }
    lines.push("");
  }
  return lines.join("\n").trimEnd() + "\n";
}

if (format === "codex") {
  process.stdout.write(formatCodex());
} else {
  const result = format === "opencode" ? formatOpenCode() : formatClaude();
  console.log(JSON.stringify(result, null, 2));
}
