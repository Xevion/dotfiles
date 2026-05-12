import { appendFileSync, mkdirSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

const LOG_DIR = join(
	process.env.XDG_CACHE_HOME || join(homedir(), ".cache"),
	"commit",
);
const LOG_PATH = join(LOG_DIR, "debug.log");

let initialized = false;
let sessionId = "";

function ensureInit(): void {
	if (initialized) return;
	try {
		mkdirSync(LOG_DIR, { recursive: true });
	} catch {
		/* ignore — logging must never throw */
	}
	sessionId = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
	initialized = true;
	logRaw("session-start", { pid: process.pid, argv: process.argv.slice(2) });
}

function logRaw(event: string, data: unknown): void {
	try {
		const line = `${JSON.stringify({
			ts: new Date().toISOString(),
			session: sessionId,
			event,
			data,
		})}\n`;
		appendFileSync(LOG_PATH, line);
	} catch {
		/* ignore */
	}
}

export function logEvent(event: string, data: unknown = null): void {
	ensureInit();
	logRaw(event, data);
}

export function logError(event: string, err: unknown): void {
	ensureInit();
	if (err instanceof Error) {
		logRaw(event, {
			name: err.name,
			message: err.message,
			stack: err.stack,
			cause:
				err.cause instanceof Error
					? {
							name: err.cause.name,
							message: err.cause.message,
							stack: err.cause.stack,
						}
					: err.cause,
		});
	} else {
		logRaw(event, { value: String(err) });
	}
}

export function formatErrorForDisplay(err: unknown): string {
	if (!(err instanceof Error)) return String(err);
	const parts = [`${err.name}: ${err.message}`];
	if (err.stack) parts.push(err.stack);
	if (err.cause) {
		if (err.cause instanceof Error) {
			parts.push(`Caused by: ${err.cause.name}: ${err.cause.message}`);
			if (err.cause.stack) parts.push(err.cause.stack);
		} else {
			parts.push(`Caused by: ${String(err.cause)}`);
		}
	}
	return parts.join("\n");
}

export const debugLogPath = LOG_PATH;
