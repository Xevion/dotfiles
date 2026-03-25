#!/usr/bin/env bun

import { execSync, spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { isCancel, Prompt, SelectPrompt } from "@clack/core";
import * as p from "@clack/prompts";
import { defineCommand, runMain } from "citty";
import pc from "picocolors";
import {
	buildUserPrompt,
	generateOptions,
	type ModelAlias,
	THINKING_DEFAULTS,
	type ThinkingLevel,
} from "./ai.ts";
import type { CommitOption } from "./schema.ts";

const MAX_AI_RETRIES = 3;

type PostAction = "commit" | "edit" | "tweak";

interface Settings {
	model: ModelAlias;
	thinking: ThinkingLevel;
	count: number;
}

const DEFAULTS: Settings = {
	model: "sonnet",
	thinking: "low",
	count: 5,
};

const S_STEP_ACTIVE = "◆";
const S_STEP_SUBMIT = "◇";
const S_STEP_CANCEL = "■";
const S_BAR = "│";
const S_BAR_END = "└";
const S_RADIO_ACTIVE = "●";
const S_RADIO_INACTIVE = "○";

type ColorFn = (text: string) => string;

const OPTION_COLORS: Record<string, Record<string, ColorFn>> = {
	model: {
		haiku: (t) => pc.bgGreen(pc.black(t)),
		sonnet: (t) => pc.bgBlue(pc.white(t)),
		opus: (t) => pc.bgMagenta(pc.white(t)),
	},
	thinking: {
		none: (t) => pc.gray(t),
		low: (t) => pc.bgGreen(pc.black(t)),
		medium: (t) => pc.bgBlue(pc.white(t)),
		high: (t) => pc.bgYellow(pc.black(t)),
		max: (t) => pc.bgRed(pc.white(t)),
	},
};

const DEFAULT_OPTION_COLOR: ColorFn = (t) => pc.bgCyan(pc.black(t));

function getOptionColor(key: string, value: string): ColorFn {
	return OPTION_COLORS[key]?.[value] ?? DEFAULT_OPTION_COLOR;
}

function bail(msg?: string): never {
	p.cancel(msg ?? "Cancelled.");
	process.exit(1);
}

function shell(cmd: string): string | null {
	try {
		return execSync(cmd, {
			encoding: "utf-8",
			stdio: ["pipe", "pipe", "pipe"],
		}).trim();
	} catch {
		return null;
	}
}

function isGitRepo(): boolean {
	const result = spawnSync("git", ["rev-parse", "--is-inside-work-tree"], {
		stdio: "pipe",
	});
	return result.status === 0;
}

function hasAnyCommits(): boolean {
	const result = spawnSync("git", ["rev-parse", "HEAD"], { stdio: "pipe" });
	return result.status === 0;
}

function hasStagedChanges(): boolean {
	const result = spawnSync("git", ["diff", "--cached", "--quiet"], {
		stdio: "pipe",
	});
	return result.status === 1;
}

function gatherDiffContext(): string {
	const out = shell("commit-helper --staged");
	if (!out) {
		p.log.error("Failed to gather staged changes.");
		p.log.info(`Is ${pc.cyan("commit-helper")} installed and on PATH?`);
		bail();
	}
	return out;
}

function writeTempFile(prefix: string, content: string): string {
	const path = `/tmp/${prefix}-${Date.now()}.txt`;
	require("node:fs").writeFileSync(path, content, "utf-8");
	return path;
}

function removeTempFile(path: string): void {
	try {
		require("node:fs").unlinkSync(path);
	} catch {
		/* ignore */
	}
}

function editInEditor(message: string): string {
	const tmpFile = writeTempFile("commit-edit", message);
	const editor = process.env.EDITOR || process.env.VISUAL || "vi";
	spawnSync(editor, [tmpFile], { stdio: "inherit" });
	const edited = readFileSync(tmpFile, "utf-8").trim();
	removeTempFile(tmpFile);
	return edited;
}

function toCommitMessage(opt: CommitOption): string {
	return opt.body ? `${opt.subject}\n\n${opt.body}` : opt.subject;
}

interface SettingRow {
	key: keyof Settings;
	label: string;
	options: readonly (string | number)[];
	idx: number;
	visibleWhen?: (s: Settings) => boolean;
}

function buildSettingRows(defaults: Settings): SettingRow[] {
	const indexOf = (opts: readonly (string | number)[], v: string | number) => {
		const i = (opts as (string | number)[]).indexOf(v);
		return i >= 0 ? i : 0;
	};
	return [
		{
			key: "model",
			label: "Model",
			options: ["sonnet", "haiku", "opus"] as const,
			idx: indexOf(["sonnet", "haiku", "opus"], defaults.model),
		},
		{
			key: "thinking",
			label: "Thinking",
			options: ["none", "low", "medium", "high", "max"] as const,
			idx: indexOf(["none", "low", "medium", "high", "max"], defaults.thinking),
			visibleWhen: (s) => s.model !== "haiku",
		},
		{
			key: "count",
			label: "Options",
			options: [3, 4, 5, 6, 7, 8, 10] as const,
			idx: indexOf([3, 4, 5, 6, 7, 8, 10], defaults.count),
		},
	];
}

function rowsToSettings(rows: SettingRow[]): Settings {
	const s: Partial<Settings> = {};
	for (const row of rows) s[row.key] = row.options[row.idx] as never;
	return s as Settings;
}

function syncThinkingDefault(rows: SettingRow[]) {
	const modelRow = rows.find((r) => r.key === "model")!;
	const thinkingRow = rows.find((r) => r.key === "thinking")!;
	const model = modelRow.options[modelRow.idx] as ModelAlias;
	const def = THINKING_DEFAULTS[model];
	const newIdx = (thinkingRow.options as string[]).indexOf(def);
	if (newIdx >= 0) thinkingRow.idx = newIdx;
}

async function promptSettings(
	overrides: Partial<Settings> = {},
): Promise<Settings | null> {
	const initial: Settings = { ...DEFAULTS, ...overrides };
	if (!overrides.thinking) initial.thinking = THINKING_DEFAULTS[initial.model];

	const rows = buildSettingRows(initial);
	let rowCursor = 0;

	const getVisible = (): SettingRow[] => {
		const s = rowsToSettings(rows);
		return rows.filter((r) => !r.visibleWhen || r.visibleWhen(s));
	};

	const renderForm = (state: string): string => {
		const visible = getVisible();

		if (state === "submit") {
			const parts = visible
				.map((r) => {
					const val = String(r.options[r.idx]);
					return getOptionColor(r.key, val)(` ${val} `);
				})
				.join(" ");
			return `${pc.gray(S_BAR)}\n${pc.green(S_STEP_SUBMIT)}  ${parts}`;
		}
		if (state === "cancel") {
			return `${pc.gray(S_BAR)}\n${pc.red(S_STEP_CANCEL)}  ${pc.dim("Cancelled")}`;
		}

		const lines: string[] = [
			pc.gray(S_BAR),
			`${pc.cyan(S_STEP_ACTIVE)}  ${pc.bold("commit")}  ${pc.dim("↕ row  ←→ option  Enter generate")}`,
		];

		for (let i = 0; i < visible.length; i++) {
			const row = visible[i]!;
			const active = i === rowCursor;
			const label = row.label.padEnd(9);

			const blocks = (row.options as readonly (string | number)[])
				.map((opt, j) => {
					const text = ` ${String(opt)} `;
					if (j === row.idx) {
						return getOptionColor(row.key, String(opt))(text);
					}
					return active ? pc.dim(text) : pc.dim(pc.gray(text));
				})
				.join(" ");

			const prefix = active ? pc.cyan(S_BAR) : pc.gray(S_BAR);
			const labelStr = active ? pc.bold(label) : pc.dim(label);
			lines.push(`${prefix}  ${labelStr} ${blocks}`);
		}

		lines.push(pc.cyan(S_BAR_END));
		return lines.join("\n");
	};

	const prompt = new Prompt(
		{
			initialValue: initial,
			render(this: Prompt) {
				return renderForm(this.state);
			},
		},
		false,
	);

	prompt.on("cursor", (key) => {
		const visible = getVisible();

		if (key === "up") {
			rowCursor = (rowCursor - 1 + visible.length) % visible.length;
		} else if (key === "down") {
			rowCursor = (rowCursor + 1) % visible.length;
		} else if (key === "left") {
			const row = visible[rowCursor]!;
			row.idx = (row.idx - 1 + row.options.length) % row.options.length;
			if (row.key === "model") syncThinkingDefault(rows);
		} else if (key === "right") {
			const row = visible[rowCursor]!;
			row.idx = (row.idx + 1) % row.options.length;
			if (row.key === "model") syncThinkingDefault(rows);
		}

		const newVisible = getVisible();
		if (rowCursor >= newVisible.length) rowCursor = newVisible.length - 1;

		prompt.value = rowsToSettings(rows);
	});

	const result = await prompt.prompt();
	if (isCancel(result)) return null;
	return result as unknown as Settings;
}

type CommitSelectValue = number | "regenerate";

interface CommitSelectOption {
	value: CommitSelectValue;
	subject: string;
	body?: string;
	isAction?: boolean;
}

function commitSelect(
	options: CommitOption[],
): Promise<CommitSelectValue | symbol> {
	const selectOptions: CommitSelectOption[] = [
		...options.map((opt, i) => ({
			value: i as CommitSelectValue,
			subject: opt.subject,
			body: opt.body,
		})),
		{
			value: "regenerate" as const,
			subject: "Regenerate options",
			isAction: true,
		},
	];

	return new SelectPrompt<CommitSelectOption & { disabled?: boolean }>({
		options: selectOptions.map((o) => ({ ...o, disabled: false })),
		initialValue: 0 as CommitSelectValue,
		render(this: SelectPrompt<CommitSelectOption & { disabled?: boolean }>) {
			const state = this.state;
			const cur = selectOptions[this.cursor];

			if (state === "submit") {
				return `${pc.gray(S_BAR)}\n${pc.green(S_STEP_SUBMIT)}  ${pc.dim(cur?.subject)}`;
			}
			if (state === "cancel") {
				return `${pc.gray(S_BAR)}\n${pc.red(S_STEP_CANCEL)}  ${pc.strikethrough(pc.dim(cur?.subject))}\n${pc.gray(S_BAR)}`;
			}

			const lines: string[] = [
				pc.gray(S_BAR),
				`${pc.cyan(S_STEP_ACTIVE)}  ${pc.dim("↕ select  Enter confirm")}`,
			];

			for (let i = 0; i < selectOptions.length; i++) {
				const o = selectOptions[i]!;
				const active = i === this.cursor;
				const bar = active ? pc.cyan(S_BAR) : pc.gray(S_BAR);

				if (o.isAction) {
					lines.push(pc.gray(S_BAR));
					const icon = active
						? pc.yellow(S_RADIO_ACTIVE)
						: pc.dim(S_RADIO_INACTIVE);
					const label = active ? pc.yellow(o.subject) : pc.dim(o.subject);
					lines.push(`${bar}  ${icon} ${label}`);
				} else {
					const icon = active
						? pc.cyan(S_RADIO_ACTIVE)
						: pc.dim(S_RADIO_INACTIVE);
					const label = active ? pc.bold(o.subject) : pc.dim(o.subject);
					lines.push(`${bar}  ${icon} ${label}`);
					if (active && o.body) {
						for (const line of o.body.split("\n")) {
							lines.push(`${bar}     ${pc.dim(line)}`);
						}
					}
				}
			}

			lines.push(pc.cyan(S_BAR_END));
			return lines.join("\n");
		},
	}).prompt() as Promise<CommitSelectValue | symbol>;
}

async function promptPostAction(): Promise<PostAction> {
	const action = await p.select({
		message: "Action",
		options: [
			{
				value: "commit" as PostAction,
				label: "Commit",
				hint: "git commit",
			},
			{ value: "edit" as PostAction, label: "Edit", hint: "$EDITOR" },
			{
				value: "tweak" as PostAction,
				label: "Tweak",
				hint: "AI baseline",
			},
		],
	});
	if (isCancel(action)) bail();
	return action as PostAction;
}

async function runCommit(message: string, dryRun: boolean): Promise<boolean> {
	if (dryRun) {
		p.log.info(pc.yellow("Dry run — would commit with:"));
		console.log();
		console.log(pc.bold(message));
		console.log();
		return true;
	}

	p.log.info("Commit message:");
	console.log(pc.dim("─".repeat(60)));
	console.log(message);
	console.log(pc.dim("─".repeat(60)));

	const tmpFile = `/tmp/commit-msg-${Date.now()}.txt`;
	await Bun.write(tmpFile, message);
	let currentMessage = message;

	while (true) {
		const result = spawnSync("git", ["commit", "-F", tmpFile], {
			encoding: "utf-8",
			stdio: ["pipe", "pipe", "pipe"],
		});

		if (result.status === 0) {
			const stdout = result.stdout?.trim() || "";
			if (stdout) p.log.success(stdout);
			removeTempFile(tmpFile);
			return true;
		}

		const stderr = result.stderr?.trim() || "";
		const stdout = result.stdout?.trim() || "";
		p.log.error("git commit failed:");
		if (stderr) console.error(stderr);
		if (stdout) console.log(stdout);

		const isHookFailure =
			stderr.includes("hook") ||
			stderr.includes("pre-commit") ||
			stderr.includes("commit-msg") ||
			result.status === 1;

		type RecoveryAction = "retry" | "force" | "edit" | "copy" | "abort";
		const recoveryOptions: {
			value: RecoveryAction;
			label: string;
			hint: string;
		}[] = [
			{ value: "retry", label: "Retry", hint: "run git commit again" },
			...(isHookFailure
				? [
						{
							value: "force" as RecoveryAction,
							label: "Force",
							hint: "git commit --no-verify",
						},
					]
				: []),
			{ value: "edit", label: "Edit message", hint: "$EDITOR" },
			{
				value: "copy",
				label: "Copy & abort",
				hint: "message printed above",
			},
			{ value: "abort", label: "Abort", hint: "exit" },
		];

		const action = await p.select({
			message: isHookFailure
				? "Pre-commit hook failed. What now?"
				: "Commit failed. What now?",
			options: recoveryOptions,
		});

		if (isCancel(action) || action === "abort") {
			p.log.warn(`Commit message preserved at: ${pc.cyan(tmpFile)}`);
			return false;
		}

		if (action === "copy") {
			p.log.info(`Message printed above. Temp file: ${pc.cyan(tmpFile)}`);
			return false;
		}

		if (action === "force") {
			const forceResult = spawnSync(
				"git",
				["commit", "--no-verify", "-F", tmpFile],
				{
					encoding: "utf-8",
					stdio: ["pipe", "pipe", "pipe"],
				},
			);
			if (forceResult.status === 0) {
				const out = forceResult.stdout?.trim() || "";
				if (out) p.log.success(out);
				removeTempFile(tmpFile);
				return true;
			}
			p.log.error("Force commit also failed:");
			if (forceResult.stderr?.trim()) console.error(forceResult.stderr.trim());
			continue;
		}

		if (action === "edit") {
			currentMessage = editInEditor(currentMessage);
			if (!currentMessage) {
				p.log.warn(`Empty message. Temp file: ${pc.cyan(tmpFile)}`);
				return false;
			}
			await Bun.write(tmpFile, currentMessage);
		}
	}
}

const promptCmd = defineCommand({
	meta: {
		name: "prompt",
		description: "Print the full prompt that would be sent to the AI",
	},
	args: {
		context: {
			type: "string",
			alias: "c",
			description: "Optional developer context to include",
		},
		system: {
			type: "boolean",
			alias: "s",
			description: "Include the system prompt (off by default)",
		},
	},
	run({ args }) {
		if (!isGitRepo()) {
			console.error("Not inside a git repository. Run `git init` first.");
			process.exit(1);
		}
		if (!hasStagedChanges()) {
			console.error("No staged changes. Stage files with `git add` first.");
			process.exit(1);
		}

		const diffContext = gatherDiffContext();
		const count = 5;
		const userPrompt = buildUserPrompt(diffContext, args.context ?? "", count);

		if (args.system) {
			console.log("=== SYSTEM PROMPT ===\n");
			// Re-import to avoid circular dep — just inline for debug command
			console.log(
				`[System prompt for ${count} options — use --system to see full text]`,
			);
			console.log("\n=== USER PROMPT ===\n");
		}
		console.log(userPrompt);
	},
});

const main = defineCommand({
	meta: {
		name: "commit",
		description:
			"Interactive AI-powered commit message generator (Claude Agent SDK)",
	},
	subCommands: { prompt: promptCmd },
	args: {
		dryRun: {
			type: "boolean",
			alias: "n",
			description: "Preview the commit message without actually committing",
		},
		count: {
			type: "string",
			alias: "N",
			description: "Number of commit message options to generate (1-15)",
		},
		context: {
			type: "string",
			alias: "c",
			description: "Context for the AI (skips the interactive context prompt)",
		},
		model: {
			type: "string",
			alias: "m",
			description: "Model override: sonnet, haiku, or opus",
		},
	},
	async run({ args }) {
		const dryRun = !!args.dryRun;
		const userContext = args.context ?? "";

		p.intro(pc.bgCyan(pc.black(" commit ")));

		if (!isGitRepo()) {
			p.log.error(
				`Not inside a git repository. Run ${pc.cyan("git init")} first.`,
			);
			bail();
		}

		if (!hasStagedChanges()) {
			p.log.error(
				`No staged changes. Stage files with ${pc.cyan("git add")} first.`,
			);
			bail();
		}

		if (!hasAnyCommits()) {
			p.log.info(
				pc.dim("Initial commit — no prior history for style reference."),
			);
		}

		const overrides: Partial<Settings> = {};
		if (
			args.model === "haiku" ||
			args.model === "sonnet" ||
			args.model === "opus"
		)
			overrides.model = args.model;
		if (args.count) {
			const n = Number.parseInt(args.count, 10);
			if (!Number.isNaN(n) && n >= 1 && n <= 15) overrides.count = n;
		}

		const settings = await promptSettings(overrides);
		if (!settings) bail();

		const { model, thinking, count } = settings;

		const s = p.spinner();
		s.start("Gathering staged changes...");
		const diffContext = shell("commit-helper --staged");
		if (!diffContext) {
			s.stop("Failed to gather changes.");
			p.log.error("commit-helper returned nothing.");
			p.log.info(`Is ${pc.cyan("commit-helper")} installed and on PATH?`);
			bail();
		}
		s.stop("Context gathered.");

		let refinement: string | undefined;
		let tweakBaseline: string | undefined;

		mainLoop: while (true) {
			let currentRefinement = refinement;
			if (tweakBaseline) {
				const feedback = await p.text({
					message: "How should the AI tweak this message?",
					placeholder: "e.g. shorter subject, focus on the API changes...",
				});
				if (isCancel(feedback)) bail();

				currentRefinement = `The user liked this message as a starting point:\n\n> ${tweakBaseline.replace(/\n/g, "\n> ")}\n\nFeedback: ${(feedback as string).trim()}\n\nGenerate ${count} variations based on this.`;
				tweakBaseline = undefined;
			}

			const label = pc.yellow(model);
			let lastRawOutput: string | undefined;
			let generatedOptions: CommitOption[] = [];

			for (let attempt = 0; attempt < MAX_AI_RETRIES; attempt++) {
				const spinner = p.spinner();
				const t0 = Date.now();
				const attemptSuffix =
					attempt > 0
						? pc.dim(` (attempt ${attempt + 1}/${MAX_AI_RETRIES})`)
						: "";
				spinner.start(`Generating via ${label}...${attemptSuffix}`);

				const timer = setInterval(() => {
					const elapsed = ((Date.now() - t0) / 1000).toFixed(0);
					spinner.message(
						`Generating via ${label}... ${pc.dim(`${elapsed}s`)}${attemptSuffix}`,
					);
				}, 1000);

				try {
					const result = await generateOptions(
						model,
						diffContext,
						userContext,
						count,
						thinking,
						currentRefinement,
					);
					clearInterval(timer);
					const elapsed = ((Date.now() - t0) / 1000).toFixed(1);
					spinner.stop(`Generated. ${pc.dim(`(${elapsed}s)`)}`);

					if (result.options.length > 0) {
						generatedOptions = result.options;
						break;
					}

					lastRawOutput = result.rawFallback;
					p.log.warn("Could not parse AI response — retrying...");
				} catch (err) {
					clearInterval(timer);
					spinner.stop("Generation failed.");
					p.log.error(
						err instanceof Error ? err.message : "Unknown error from AI.",
					);
					if (attempt < MAX_AI_RETRIES - 1) {
						p.log.info(`Retrying... (${attempt + 2}/${MAX_AI_RETRIES})`);
					}
				}
			}

			if (generatedOptions.length === 0) {
				p.log.error("Failed to generate commit options after all retries.");
				if (lastRawOutput) {
					p.log.warn("Last raw AI output:");
					console.log(pc.dim("─".repeat(60)));
					console.log(lastRawOutput.slice(0, 3000));
					if (lastRawOutput.length > 3000)
						console.log(
							pc.dim(`... (${lastRawOutput.length - 3000} chars truncated)`),
						);
					console.log(pc.dim("─".repeat(60)));
				}
				bail();
			}

			refinement = undefined;

			const pick = await commitSelect(generatedOptions);
			if (isCancel(pick)) bail();

			if (pick === "regenerate") {
				continue;
			}

			const chosen = generatedOptions[pick as number]!;
			const action = await promptPostAction();

			switch (action) {
				case "commit": {
					const ok = await runCommit(toCommitMessage(chosen), dryRun);
					if (!ok) bail("Commit failed.");
					break mainLoop;
				}
				case "edit": {
					const draft = toCommitMessage(chosen);
					const edited = editInEditor(draft);
					if (!edited) {
						p.log.warn("Empty message — aborting.");
						bail();
					}
					const ok = await runCommit(edited, dryRun);
					if (!ok) bail("Commit failed.");
					break mainLoop;
				}
				case "tweak": {
					tweakBaseline = toCommitMessage(chosen);
					refinement = undefined;
					continue mainLoop;
				}
			}
		}

		p.outro(dryRun ? pc.yellow("Dry run complete.") : pc.green("Committed!"));
	},
});

runMain(main);
