import { query } from "@anthropic-ai/claude-agent-sdk";
import Ajv from "ajv/dist/2020";
import {
	type CommitOption,
	type CommitOptionsOutput,
	commitOptionsSchema,
} from "./schema.ts";

export type ModelAlias = "haiku" | "sonnet" | "opus";
export type ThinkingLevel = "none" | "low" | "medium" | "high" | "max";

const MODEL_IDS: Record<ModelAlias, string> = {
	haiku: "haiku",
	sonnet: "sonnet",
	opus: "opus",
};

export const THINKING_DEFAULTS: Record<ModelAlias, ThinkingLevel> = {
	haiku: "none",
	sonnet: "low",
	opus: "low",
};

const ajv = new Ajv();
const validateCommitOptions = ajv.compile<CommitOptionsOutput>(
	commitOptionsSchema as Record<string, unknown>,
);

/** Strip $schema directive — the SDK doesn't support it */
function stripSchemaDirective(
	schema: Record<string, unknown>,
): Record<string, unknown> {
	const { $schema: _, ...rest } = schema;
	return rest;
}

function buildSystemPrompt(count: number): string {
	return `You are a commit message generator. You will be given a diff of staged changes and optional context from the developer.

Your task: produce exactly ${count} commit message options.

## CRITICAL: Each option must describe the ENTIRE commit

Every option must summarize ALL the staged changes as a whole. The ${count} options are alternative wordings for the SAME commit — not different perspectives that each highlight a different subset of changes.

Bad: 5 options where each focuses on a different file or feature area.
Good: 5 options that each describe the full scope of changes, varying in wording, emphasis, or abstraction level.

If the commit touches multiple areas (e.g. refactoring + new features + API changes), every option should reflect that breadth. Vary options by:
- Different prefix choices (feat vs refactor vs chore) when the change is ambiguous
- Different levels of abstraction (specific vs high-level summary)
- Different emphasis on the "why" vs the "what"
- Different phrasing of the same core change

## CRITICAL: Match the existing commit style EXACTLY

The input includes a "Recent Commit Style" section showing the repository's actual recent commits.
This section is your PRIMARY source of truth. You MUST replicate the exact formatting conventions you see there.

Before writing ANY option, analyze the recent commits and answer these questions internally:
1. Do they use a prefix like "type: description"? If yes, EVERY option you generate MUST use a prefix.
2. What prefix types appear? (e.g. feat, fix, config, refactor, docs) Use only prefixes that appear in the history or are semantically appropriate for the change.
3. What comes after the prefix — lowercase or capitalized? Match it.
4. How long are the messages? Match that level of detail.

If the recent commits use conventional commit style (e.g. "feat:", "fix:", "config:"), then ALL ${count} of your options MUST use a prefix. Never mix prefixed and unprefixed styles.

## Formatting rules

- Subject line under 72 characters
- Imperative mood ("add", "fix", "refactor", not "added", "fixes")
- No trailing periods on subject lines
- Single-line subject for most commits
- Add a body (separated by blank line) ONLY for genuinely complex changes
- Focus on WHAT changed and WHY, not implementation details
- NEVER mention: test results, lockfile changes, file counts, build status

## Output format

Respond with a JSON object containing an "options" array with exactly ${count} objects. Each object has a "subject" field (required, string) and an optional "body" field (string). Do not include any text outside the JSON.`;
}

export function buildUserPrompt(
	diffContext: string,
	userContext: string,
	count: number,
	refinement?: string,
): string {
	let prompt = diffContext;
	if (userContext.trim()) {
		prompt += `\n\n## Developer Context\n\n${userContext.trim()}`;
	}
	if (refinement) {
		prompt += `\n\n## Refinement Request\n\n${refinement}`;
	}
	prompt += `\n\nGenerate exactly ${count} commit message options in the specified format.`;
	return prompt;
}

/**
 * Fallback: extract and validate commit options JSON from raw result text.
 * Handles responses where the model emitted JSON as text (possibly code-fenced)
 * instead of using the SDK's structured output tool.
 */
function tryExtractCommitOptions(raw: string): CommitOptionsOutput | null {
	const stripped = raw
		.replace(/^```(?:json)?\s*\n?/m, "")
		.replace(/\n?```\s*$/m, "");

	const jsonMatch = stripped.match(/\{[\s\S]*\}/);
	if (!jsonMatch) return null;

	let parsed: unknown;
	try {
		parsed = JSON.parse(jsonMatch[0]);
	} catch {
		return null;
	}

	if (validateCommitOptions(parsed)) {
		return parsed;
	}
	return null;
}

export interface GenerateResult {
	options: CommitOption[];
	rawFallback?: string;
}

export async function generateOptions(
	model: ModelAlias,
	diffContext: string,
	userContext: string,
	count: number,
	thinking: ThinkingLevel,
	refinement?: string,
): Promise<GenerateResult> {
	const systemPrompt = buildSystemPrompt(count);
	const userPrompt = buildUserPrompt(
		diffContext,
		userContext,
		count,
		refinement,
	);
	const modelId = MODEL_IDS[model];

	const effort =
		thinking === "none"
			? ("low" as const)
			: (thinking as "low" | "medium" | "high" | "max");

	let output: CommitOptionsOutput | null = null;
	let rawResultText: string | null = null;

	for await (const message of query({
		prompt: userPrompt,
		options: {
			model: modelId,
			systemPrompt,
			thinking:
				thinking === "none"
					? { type: "disabled" }
					: { type: "enabled", budgetTokens: thinkingBudget(thinking) },
			persistSession: false,
			settingSources: [],
			maxTurns: 4,
			effort,
			permissionMode: "dontAsk",
			outputFormat: {
				type: "json_schema",
				schema: stripSchemaDirective(
					commitOptionsSchema as Record<string, unknown>,
				),
			},
		},
	})) {
		if (message.type === "result") {
			if (message.subtype === "success") {
				rawResultText = message.result;
				if (message.structured_output) {
					const candidate = message.structured_output as CommitOptionsOutput;
					if (validateCommitOptions(candidate)) {
						output = candidate;
					}
				}
			}
		}
	}

	if (!output && rawResultText) {
		output = tryExtractCommitOptions(rawResultText);
	}

	if (output) {
		const options = output.options
			.filter((opt) => opt.subject && typeof opt.subject === "string")
			.map((opt) => ({
				subject: opt.subject.trim(),
				body: opt.body?.trim() || undefined,
			}));
		if (options.length > 0) {
			return { options };
		}
	}

	return { options: [], rawFallback: rawResultText ?? undefined };
}

function thinkingBudget(level: ThinkingLevel): number {
	switch (level) {
		case "low":
			return 1024;
		case "medium":
			return 4096;
		case "high":
			return 16384;
		case "max":
			return 65536;
		default:
			return 1024;
	}
}
