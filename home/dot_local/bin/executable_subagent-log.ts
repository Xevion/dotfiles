#!/usr/bin/env bun
// Browse and read Claude Code subagent transcripts, independent of /tasks.
// Every run is persisted at ~/.claude/projects/<encoded-cwd>/<session>/subagents/agent-<id>.jsonl.

import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

interface Run {
    path: string;
    agentId: string;
    sessionId: string;
    mtimeMs: number;
    agentType: string;
    description: string;
}

function encodeCwd(cwd: string): string {
    return cwd.replace(/[/.]/g, "-");
}

function projectDir(): string {
    return join(process.env.HOME ?? "", ".claude", "projects", encodeCwd(process.cwd()));
}

function findRuns(root: string): Run[] {
    const runs: Run[] = [];
    if (!existsSync(root)) return runs;
    for (const sessionEntry of readdirSync(root, { withFileTypes: true })) {
        if (!sessionEntry.isDirectory()) continue;
        const subagentsDir = join(root, sessionEntry.name, "subagents");
        if (!existsSync(subagentsDir)) continue;
        for (const file of readdirSync(subagentsDir)) {
            if (!file.startsWith("agent-") || !file.endsWith(".jsonl")) continue;
            const path = join(subagentsDir, file);
            const agentId = file.slice("agent-".length, -".jsonl".length);
            const metaPath = join(subagentsDir, `agent-${agentId}.meta.json`);
            let agentType = "?";
            let description = "";
            if (existsSync(metaPath)) {
                try {
                    const meta = JSON.parse(readFileSync(metaPath, "utf8"));
                    agentType = meta.agentType ?? agentType;
                    description = meta.description ?? description;
                } catch {}
            }
            runs.push({
                path,
                agentId,
                sessionId: sessionEntry.name,
                mtimeMs: statSync(path).mtimeMs,
                agentType,
                description,
            });
        }
    }
    return runs.sort((a, b) => b.mtimeMs - a.mtimeMs);
}

function getReport(path: string): string {
    const lines = readFileSync(path, "utf8").split("\n").filter(Boolean);
    for (let i = lines.length - 1; i >= 0; i--) {
        let entry: any;
        try {
            entry = JSON.parse(lines[i]!);
        } catch {
            continue;
        }
        if (entry.type === "assistant" && entry.message?.role === "assistant") {
            const content = entry.message.content ?? [];
            return content
                .filter((c: any) => c.type === "text")
                .map((c: any) => c.text)
                .join("\n");
        }
    }
    return "(no assistant text found in this transcript)";
}

function formatWhen(ms: number): string {
    const d = new Date(ms);
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatListLine(r: Run): string {
    return `${formatWhen(r.mtimeMs)}  ${r.agentId}  session=${r.sessionId}  ${r.agentType}: ${r.description}`;
}

function commandExists(cmd: string): boolean {
    return spawnSync("which", [cmd]).status === 0;
}

function renderMarkdown(text: string, paging: boolean): void {
    if (commandExists("bat")) {
        const args = ["--style=plain", "--color=always", "--language=markdown"];
        if (paging) args.push("--paging=always");
        spawnSync("bat", args, { input: text, stdio: ["pipe", "inherit", "inherit"] });
    } else if (paging && process.stdout.isTTY) {
        spawnSync("less", ["-R"], { input: text, stdio: ["pipe", "inherit", "inherit"] });
    } else {
        console.log(text);
    }
}

function truncateForPreview(text: string): string {
    const maxChars = 20000;
    if (text.length <= maxChars) return text;
    return text.slice(0, maxChars) + "\n\n… truncated — press enter to view the full report";
}

function browse(runs: Run[]): void {
    if (!commandExists("fzf")) {
        for (const r of runs) console.log(formatListLine(r));
        return;
    }
    const scriptPath = process.argv[1]!;
    const lines = runs
        .map((r) => `${r.agentId}\t${formatWhen(r.mtimeMs)}\t${r.sessionId.slice(0, 8)}\t${r.agentType}: ${r.description}`)
        .join("\n");
    const result = spawnSync(
        "fzf",
        [
            "--ansi",
            "--delimiter",
            "\t",
            "--with-nth",
            "2,3,4",
            "--preview",
            `bun "${scriptPath}" --preview {1}`,
            "--preview-window",
            "right:60%:wrap",
            "--header",
            "enter: view full report   ctrl-c: quit",
        ],
        { input: lines, stdio: ["pipe", "pipe", "inherit"], encoding: "utf8" }
    );
    const selected = result.stdout?.trim();
    if (!selected) return;
    const agentId = selected.split("\t")[0];
    const run = runs.find((r) => r.agentId === agentId);
    if (!run) {
        console.error("Could not resolve selection.");
        process.exit(1);
    }
    renderMarkdown(getReport(run.path), true);
}

function usage(): void {
    console.error(
        [
            "usage: subagent-log.ts                 interactive fzf browser (enter: view report)",
            "       subagent-log.ts list             plain list, newest first",
            "       subagent-log.ts latest            print the most recent subagent's report",
            "       subagent-log.ts <agent-id|query>  print a specific run's report",
        ].join("\n")
    );
}

const argv = process.argv.slice(2);
const root = projectDir();

if (!existsSync(root)) {
    console.error(`No Claude Code project dir found for ${process.cwd()} (${root})`);
    process.exit(1);
}

if (argv[0] === "--preview") {
    const runs = findRuns(root);
    const run = runs.find((r) => r.agentId === argv[1]);
    if (!run) {
        console.error("not found");
        process.exit(1);
    }
    renderMarkdown(truncateForPreview(getReport(run.path)), false);
    process.exit(0);
}

const runs = findRuns(root);
if (runs.length === 0) {
    console.error(`No subagent runs found under ${root}`);
    process.exit(1);
}

if (argv[0] === "-h" || argv[0] === "--help") {
    usage();
} else if (argv[0] === "latest") {
    renderMarkdown(getReport(runs[0]!.path), true);
} else if (argv[0] === "list" || (!process.stdout.isTTY && argv.length === 0)) {
    for (const r of runs) console.log(formatListLine(r));
} else if (argv.length > 0) {
    const q = argv[0]!;
    const run = runs.find((r) => r.agentId.startsWith(q)) ?? runs.find((r) => r.description.toLowerCase().includes(q.toLowerCase()));
    if (!run) {
        console.error(`No subagent run matching '${q}'`);
        process.exit(1);
    }
    renderMarkdown(getReport(run.path), true);
} else {
    browse(runs);
}
