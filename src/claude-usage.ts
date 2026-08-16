/**
 * claude-usage - Fast CLI tool to fetch Anthropic Claude usage percentages
 */

import { existsSync } from 'fs';
import { join } from 'path';
import { homedir } from 'os';
import chalk from 'chalk';

// Configuration
const CLAUDE_CODE_PATH = join(homedir(), '.claude', '.credentials.json');
const OPENCODE_PATH = join(homedir(), '.local', 'share', 'opencode', 'auth.json');

// Active hours model: 9 AM to 2 AM active, 2 AM to 9 AM sleep
export const SLEEP_START_HOUR = 2;  // Sleep begins at 2:00 AM
export const WAKE_HOUR = 9;         // Active period begins at 9:00 AM
export const SLEEP_HOURS_PER_DAY = 7;   // 2 AM to 9 AM
export const ACTIVE_HOURS_PER_DAY = 17; // 9 AM to 2 AM

// Below this much time left until reset, a %/hour budget rate is too noisy to be
// useful (small denominator blows up the rate) — suppress it instead of showing it.
const BUDGET_SUPPRESS_THRESHOLD_HOURS = 0.5;

// Parse CLI flags
const args = process.argv.slice(2);
const VERBOSE = args.includes('-v') || args.includes('--verbose') || args.includes('-d') || args.includes('--debug');
// Default is the dedicated GET /api/oauth/usage endpoint. --legacy falls back to the
// original approach (a throwaway 1-token POST /v1/messages probe read for its rate-limit
// headers) — kept around for comparison/fallback, not used unless explicitly requested.
const LEGACY = args.includes('--legacy');

interface UsagePeriod {
  utilization: number;
  reset: number;
}

interface UsageData {
  five_hour: UsagePeriod;
  seven_day: UsagePeriod;
}

interface ClaudeCodeCredentials {
  claudeAiOauth?: {
    accessToken?: string;
  };
}

interface OpenCodeAuth {
  anthropic?: {
    type: string;
    access?: string;
    refresh?: string;
    expires?: number;
  };
}

interface TokenResult {
  token: string;
  source: 'claude-code' | 'opencode';
  expiresAt?: number;
}

interface PaceResult {
  diff: number;
  status: string;
  /** %/hour still available between now and reset to land exactly at 100% */
  budgetRate: number;
  /** %/hour used on average since the period started */
  avgRate: number;
  /** Hours left until reset (the denominator behind budgetRate) */
  remainingHours: number;
}

// The /api/oauth/usage response is undocumented and Anthropic has reshaped it before
// (per-model 7d buckets appeared unannounced in April 2026) — so it's parsed loosely,
// field-by-field, rather than trusting a fixed shape end to end.
type OAuthUsageResponse = Record<string, unknown>;

/** A single renderable usage window, after loosely parsing the raw API response. */
interface DisplayWindow {
  label: string;
  /** 0-100 */
  percent: number;
  /** Unix seconds, or null if the response didn't include a usable reset time */
  resetTimestamp: number | null;
  /** Which pace model to apply — null means "unrecognized window, don't guess a period" */
  kind: '5h' | '7d' | null;
  severity: string | null;
  /** false for anything not in the known kind/key list below — surfaced but not trusted for pace math */
  known: boolean;
}

/**
 * Interpolate between two hex colors
 */
function interpolateColor(color1: string, color2: string, ratio: number): string {
  const r1 = parseInt(color1.slice(1, 3), 16);
  const g1 = parseInt(color1.slice(3, 5), 16);
  const b1 = parseInt(color1.slice(5, 7), 16);
  
  const r2 = parseInt(color2.slice(1, 3), 16);
  const g2 = parseInt(color2.slice(3, 5), 16);
  const b2 = parseInt(color2.slice(5, 7), 16);
  
  const r = Math.round(r1 + (r2 - r1) * ratio);
  const g = Math.round(g1 + (g2 - g1) * ratio);
  const b = Math.round(b1 + (b2 - b1) * ratio);
  
  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
}

/**
 * Get pastel color based on usage percentage (soft rainbow gradient)
 */
function getColorForPercentage(pct: number): string {
  const stops = [
    { threshold: 0,   color: '#A5D8DD' }, // Soft cyan
    { threshold: 15,  color: '#9DCCB4' }, // Soft teal
    { threshold: 30,  color: '#B8D99A' }, // Soft lime
    { threshold: 45,  color: '#E8D4A2' }, // Soft yellow
    { threshold: 60,  color: '#F4B8A4' }, // Soft orange
    { threshold: 75,  color: '#F5A6A6' }, // Soft red
    { threshold: 85,  color: '#E89999' }, // Medium red
    { threshold: 95,  color: '#D88888' }, // Deeper red
  ];
  
  let lower = stops[0];
  let upper = stops[stops.length - 1];
  
  for (let i = 0; i < stops.length - 1; i++) {
    if (pct >= stops[i].threshold && pct < stops[i + 1].threshold) {
      lower = stops[i];
      upper = stops[i + 1];
      break;
    }
  }
  
  if (pct >= stops[stops.length - 1].threshold) {
    return stops[stops.length - 1].color;
  }
  
  const range = upper.threshold - lower.threshold;
  const position = range > 0 ? (pct - lower.threshold) / range : 0;
  
  return interpolateColor(lower.color, upper.color, position);
}

/**
 * Generate user_id metadata for the request (mimics Claude Code's format)
 */
function generateUserId(accessToken: string): string {
  // Use hash of token as device/user ID
  const hash = Bun.hash(accessToken).toString(16).padStart(64, '0').substring(0, 64);
  
  // Generate a session UUID
  const sessionId = crypto.randomUUID();
  
  return `user_${hash}_session_${sessionId}`;
}

/**
 * Spinner class for loading animation with rainbow gradient
 */
class Spinner {
  private frames = ['⣾', '⣽', '⣻', '⢿', '⡿', '⣟', '⣯', '⣷'];
  private colors = ['#A5D8DD', '#9DCCB4', '#B8D99A', '#E8D4A2', '#F4B8A4', '#F5A6A6'];
  private index = 0;
  private interval: Timer | null = null;
  
  start(message: string) {
    process.stdout.write('\x1B[?25l');
    
    this.interval = setInterval(() => {
      const colorIndex = this.index % this.colors.length;
      const frame = chalk.hex(this.colors[colorIndex])(this.frames[this.index]);
      process.stdout.write(`\r${frame} ${chalk.hex('#9CA3AF')(message)}`);
      this.index = (this.index + 1) % this.frames.length;
    }, 80);
  }
  
  stop() {
    if (this.interval) {
      clearInterval(this.interval);
      process.stdout.write('\r\x1B[K\x1B[?25h');
    }
  }
}

/**
 * Read OAuth token from credentials files (checks both Claude Code and OpenCode)
 */
async function readToken(): Promise<TokenResult> {
  const errors: string[] = [];
  
  // Try OpenCode first (likely to be more up-to-date)
  if (existsSync(OPENCODE_PATH)) {
    try {
      const fileContent = Bun.file(OPENCODE_PATH);
      const auth: OpenCodeAuth = await fileContent.json() as OpenCodeAuth;
      
      const token = auth.anthropic?.access;
      if (token) {
        const expiresAt = auth.anthropic?.expires;
        
        // Check if token is expired
        if (expiresAt && expiresAt < Date.now()) {
          errors.push(`OpenCode token found but expired (expired at ${new Date(expiresAt).toLocaleString()})`);
        } else {
          return {
            token,
            source: 'opencode',
            expiresAt
          };
        }
      } else {
        errors.push(`OpenCode auth file exists but no Anthropic access token found`);
      }
    } catch (error) {
      errors.push(`Failed to read OpenCode auth: ${error instanceof Error ? error.message : 'unknown error'}`);
    }
  }
  
  // Try Claude Code
  if (existsSync(CLAUDE_CODE_PATH)) {
    try {
      const fileContent = Bun.file(CLAUDE_CODE_PATH);
      const credentials: ClaudeCodeCredentials = await fileContent.json() as ClaudeCodeCredentials;
      
      const token = credentials.claudeAiOauth?.accessToken;
      if (token) {
        return {
          token,
          source: 'claude-code'
        };
      } else {
        errors.push('Claude Code credentials file exists but no access token found');
      }
    } catch (error) {
      errors.push(`Failed to read Claude Code credentials: ${error instanceof Error ? error.message : 'unknown error'}`);
    }
  }
  
  // No valid tokens found
  const errorMsg = [
    'No valid OAuth token found.',
    '',
    'Checked locations:',
    `  • OpenCode: ${OPENCODE_PATH}`,
    `  • Claude Code: ${CLAUDE_CODE_PATH}`,
  ];
  
  if (errors.length > 0) {
    errorMsg.push('', 'Details:');
    errors.forEach(err => errorMsg.push(`  • ${err}`));
  }
  
  throw new Error(errorMsg.join('\n'));
}

/**
 * Get human-readable status message from HTTP status code
 */
function getStatusMessage(status: number): string {
  const statusMessages: Record<number, string> = {
    400: 'Bad Request',
    401: 'Unauthorized',
    403: 'Forbidden',
    404: 'Not Found',
    429: 'Rate Limited',
    500: 'Internal Server Error',
    502: 'Bad Gateway',
    503: 'Service Unavailable',
  };
  return statusMessages[status] || 'Unknown Error';
}

/**
 * Format API error with enhanced details
 */
function formatApiError(status: number, url: string, accessToken: string, responseText: string, tokenSource?: string): string {
  const statusMsg = getStatusMessage(status);
  const tokenPreview = `${accessToken.substring(0, 8)}...${accessToken.substring(accessToken.length - 4)}`;
  
  let errorDetails = '';
  try {
    const errorJson = JSON.parse(responseText);
    
    // Extract key fields for compact display
    const errorType = errorJson.error?.type || errorJson.type || 'unknown';
    const errorMessage = errorJson.error?.message || errorJson.message || 'No message';
    
    errorDetails = chalk.hex('#E89999')(`  ${chalk.bold('Type:')} ${errorType}\n`);
    errorDetails += chalk.hex('#F4B8A4')(`  ${chalk.bold('Message:')} ${errorMessage}`);
  } catch {
    // If not JSON, show raw text (truncated)
    const truncated = responseText.length > 150 ? responseText.substring(0, 150) + '...' : responseText;
    errorDetails = chalk.hex('#F4B8A4')(`  ${truncated}`);
  }
  
  const lines = [
    chalk.hex('#E89999')(`${chalk.bold('API Error:')} ${status} ${statusMsg}`),
    chalk.hex('#9CA3AF')(`  ${chalk.bold('URL:')} ${url}`),
    chalk.hex('#9CA3AF')(`  ${chalk.bold('Token:')} ${tokenPreview}${tokenSource ? ` (from ${tokenSource})` : ''}`),
    errorDetails
  ];
  
  return lines.join('\n');
}

/**
 * Fetch usage data by making an API request and reading rate limit headers
 * This request matches what Claude Code sends for checking limit
 */
async function fetchUsage(accessToken: string, tokenSource?: string): Promise<UsageData> {
  const userId = generateUserId(accessToken);
  const apiUrl = 'https://api.anthropic.com/v1/messages?beta=true';
  
  const requestBody = {
    model: 'claude-haiku-4-5-20251001',
    max_tokens: 1,
    messages: [
      {
        role: 'user',
        content: 'limit',
      },
    ],
    metadata: {
      user_id: userId,
    },
  };
  
  const requestHeaders = {
    'Accept': 'application/json',
    'Authorization': `Bearer ${accessToken.substring(0, 20)}...`,
    'anthropic-version': '2023-06-01',
    'anthropic-beta': 'oauth-2025-04-20,interleaved-thinking-2025-05-14',
    'anthropic-dangerous-direct-browser-access': 'true',
    'x-app': 'cli',
    'User-Agent': 'claude-cli/2.0.76 (external, sdk-cli)',
    'Content-Type': 'application/json',
    'accept-language': '*',
    'sec-fetch-mode': 'cors',
    'accept-encoding': 'br, gzip, deflate',
  };
  
  if (VERBOSE) {
    console.log(chalk.hex('#6B7280')('\n=== Debug: API Request ==='));
    console.log(chalk.hex('#9CA3AF')('URL:'), apiUrl);
    console.log(chalk.hex('#9CA3AF')('Method:'), 'POST');
    console.log(chalk.hex('#9CA3AF')('Headers:'), JSON.stringify(requestHeaders, null, 2));
    console.log(chalk.hex('#9CA3AF')('Body:'), JSON.stringify(requestBody, null, 2));
    console.log();
  }
  
  // Make the exact same request Claude Code makes for checking limit
  const response = await fetch(apiUrl, {
    method: 'POST',
    headers: {
      'Accept': 'application/json',
      'Authorization': `Bearer ${accessToken}`,
      'anthropic-version': '2023-06-01',
      'anthropic-beta': 'oauth-2025-04-20,interleaved-thinking-2025-05-14',
      'anthropic-dangerous-direct-browser-access': 'true',
      'x-app': 'cli',
      'User-Agent': 'claude-cli/2.0.76 (external, sdk-cli)',
      'Content-Type': 'application/json',
      'accept-language': '*',
      'sec-fetch-mode': 'cors',
      'accept-encoding': 'br, gzip, deflate',
    },
    body: JSON.stringify(requestBody),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(formatApiError(response.status, apiUrl, accessToken, text, tokenSource));
  }

  // Extract rate limit headers from response
  const headers = response.headers;
  const fiveHourUtil = parseFloat(headers.get('anthropic-ratelimit-unified-5h-utilization') || '0');
  const sevenDayUtil = parseFloat(headers.get('anthropic-ratelimit-unified-7d-utilization') || '0');
  const fiveHourReset = parseInt(headers.get('anthropic-ratelimit-unified-5h-reset') || '0');
  const sevenDayReset = parseInt(headers.get('anthropic-ratelimit-unified-7d-reset') || '0');

  if (VERBOSE) {
    console.log(chalk.hex('#6B7280')('=== Debug: Response Headers ==='));
    console.log(chalk.hex('#9CA3AF')('Status:'), response.status, response.statusText);
    
    // Show all response headers
    const allHeaders: Record<string, string> = {};
    headers.forEach((value, key) => {
      allHeaders[key] = value;
    });
    console.log(chalk.hex('#9CA3AF')('All Headers:'), JSON.stringify(allHeaders, null, 2));
    
    console.log(chalk.hex('#6B7280')('\n=== Debug: Parsed Values ==='));
    console.log(chalk.hex('#9CA3AF')('5h utilization:'), fiveHourUtil);
    console.log(chalk.hex('#9CA3AF')('5h reset:'), fiveHourReset, `(${new Date(fiveHourReset * 1000).toLocaleString()})`);
    console.log(chalk.hex('#9CA3AF')('7d utilization:'), sevenDayUtil);
    console.log(chalk.hex('#9CA3AF')('7d reset:'), sevenDayReset, `(${new Date(sevenDayReset * 1000).toLocaleString()})`);
    console.log();
  }

  return {
    five_hour: {
      utilization: fiveHourUtil,
      reset: fiveHourReset,
    },
    seven_day: {
      utilization: sevenDayUtil,
      reset: sevenDayReset,
    },
  };
}

/**
 * Fetch usage data from the dedicated (undocumented) OAuth usage endpoint — a single
 * GET instead of a throwaway message probe. Same Bearer token Claude Code itself uses.
 * Returns the parsed JSON body as-is; callers must not assume a fixed shape, since this
 * endpoint isn't part of any published contract and Anthropic has reshaped it before.
 */
async function fetchOAuthUsage(accessToken: string, tokenSource?: string): Promise<OAuthUsageResponse> {
  const apiUrl = 'https://api.anthropic.com/api/oauth/usage';

  const requestHeaders = {
    'Accept': 'application/json',
    'Authorization': `Bearer ${accessToken}`,
    'anthropic-beta': 'oauth-2025-04-20',
    'anthropic-dangerous-direct-browser-access': 'true',
    'User-Agent': 'claude-cli/2.0.76 (external, sdk-cli)',
  };

  if (VERBOSE) {
    console.log(chalk.hex('#6B7280')('\n=== Debug: API Request ==='));
    console.log(chalk.hex('#9CA3AF')('URL:'), apiUrl);
    console.log(chalk.hex('#9CA3AF')('Method:'), 'GET');
    console.log(chalk.hex('#9CA3AF')('Headers:'), JSON.stringify({ ...requestHeaders, Authorization: `Bearer ${accessToken.substring(0, 20)}...` }, null, 2));
    console.log();
  }

  const response = await fetch(apiUrl, { method: 'GET', headers: requestHeaders });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(formatApiError(response.status, apiUrl, accessToken, text, tokenSource));
  }

  const text = await response.text();

  if (VERBOSE) {
    console.log(chalk.hex('#6B7280')('=== Debug: Response Body ==='));
    console.log(chalk.hex('#9CA3AF')('Status:'), response.status, response.statusText);
    console.log(chalk.hex('#9CA3AF')('Body:'), text);
    console.log();
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch (error) {
    throw new Error(`/api/oauth/usage returned unparseable JSON (schema may have changed): ${error instanceof Error ? error.message : 'unknown error'}\nRaw body (first 300 chars): ${text.slice(0, 300)}`);
  }

  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error(`/api/oauth/usage returned a non-object JSON body (schema may have changed): ${text.slice(0, 300)}`);
  }

  return parsed as OAuthUsageResponse;
}

function safeNumber(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null;
}

function safeString(v: unknown): string | null {
  return typeof v === 'string' && v.length > 0 ? v : null;
}

function safeObject(v: unknown): Record<string, unknown> | null {
  return typeof v === 'object' && v !== null && !Array.isArray(v) ? v as Record<string, unknown> : null;
}

function parseIsoToUnixSeconds(iso: string | null): number | null {
  if (!iso) return null;
  const ms = Date.parse(iso);
  return Number.isFinite(ms) ? ms / 1000 : null;
}

/** "weekly_all" -> "Weekly All", "nimbus_quill" -> "Nimbus Quill" */
function humanizeKey(key: string): string {
  return key
    .split(/[_-]+/)
    .filter(Boolean)
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/** Best-effort human label for a `limits[].scope` object of unknown shape. */
function describeScope(scope: unknown): string | null {
  const obj = safeObject(scope);
  if (!obj) return null;
  const model = safeObject(obj.model);
  const displayName = model ? safeString(model.display_name) : null;
  if (displayName) return displayName;
  return typeof obj.surface === 'string' ? obj.surface : null;
}

/**
 * Turn a raw /api/oauth/usage response into renderable windows. Drives primarily off
 * the `limits[]` array (the most structured, self-describing part of the response),
 * falls back to the top-level `five_hour`/`seven_day` objects if `limits` is missing
 * a window it's expected to have, and finally scans every other top-level key for
 * anything window-shaped (`{ utilization, resets_at, ... }`) that isn't accounted for
 * yet — that last pass is what lets a brand-new window Anthropic ships unannounced
 * (they've done exactly this before, e.g. `seven_day_opus` in April 2026) show up
 * automatically instead of silently vanishing.
 */
export function buildDisplayWindows(raw: OAuthUsageResponse): DisplayWindow[] {
  const windows: DisplayWindow[] = [];

  const fiveHourObj = safeObject(raw.five_hour);
  const sevenDayObj = safeObject(raw.seven_day);

  const limitsArr = Array.isArray(raw.limits) ? raw.limits : [];
  const renderedKinds = new Set<string>();

  for (const entry of limitsArr) {
    const obj = safeObject(entry);
    if (!obj) continue;

    const kind = safeString(obj.kind) ?? '';
    const group = safeString(obj.group) ?? '';
    const percent = safeNumber(obj.percent);
    if (percent === null) continue; // nothing to render without a percentage

    const resetTimestamp = parseIsoToUnixSeconds(safeString(obj.resets_at));
    const severity = safeString(obj.severity);
    const scopeLabel = describeScope(obj.scope);

    if (kind === 'session') {
      renderedKinds.add('session');
      // Prefer the top-level `five_hour` object in case it ever carries sub-percent
      // precision the `limits[].percent` integer doesn't.
      const preciseUtil = fiveHourObj ? safeNumber(fiveHourObj.utilization) : null;
      const preciseReset = fiveHourObj ? parseIsoToUnixSeconds(safeString(fiveHourObj.resets_at)) : null;
      windows.push({
        label: 'Hourly (5h)',
        percent: preciseUtil ?? percent,
        resetTimestamp: preciseReset ?? resetTimestamp,
        kind: '5h',
        severity,
        known: true,
      });
    } else if (kind === 'weekly_all') {
      renderedKinds.add('weekly_all');
      const preciseUtil = sevenDayObj ? safeNumber(sevenDayObj.utilization) : null;
      const preciseReset = sevenDayObj ? parseIsoToUnixSeconds(safeString(sevenDayObj.resets_at)) : null;
      windows.push({
        label: 'Weekly (7d)',
        percent: preciseUtil ?? percent,
        resetTimestamp: preciseReset ?? resetTimestamp,
        kind: '7d',
        severity,
        known: true,
      });
    } else if (kind === 'weekly_scoped') {
      windows.push({
        label: `Weekly — ${scopeLabel ?? 'Scoped'} (7d)`,
        percent,
        resetTimestamp,
        kind: '7d',
        severity,
        known: true,
      });
    } else {
      // A `limits[]` kind we don't recognize — Anthropic added something new.
      // Show it, but don't guess at a period length for pace math.
      const base = humanizeKey(kind || group || 'Unknown');
      windows.push({
        label: `${base}${scopeLabel ? ` — ${scopeLabel}` : ''} (new)`,
        percent,
        resetTimestamp,
        kind: null,
        severity,
        known: false,
      });
    }
  }

  // `limits[]` didn't include a session and/or weekly_all entry — fall back to the
  // top-level objects directly so the two headline numbers never silently disappear.
  if (!renderedKinds.has('session') && fiveHourObj) {
    const util = safeNumber(fiveHourObj.utilization);
    if (util !== null) {
      windows.unshift({
        label: 'Hourly (5h)',
        percent: util,
        resetTimestamp: parseIsoToUnixSeconds(safeString(fiveHourObj.resets_at)),
        kind: '5h',
        severity: null,
        known: true,
      });
    }
  }
  if (!renderedKinds.has('weekly_all') && sevenDayObj) {
    const util = safeNumber(sevenDayObj.utilization);
    if (util !== null) {
      windows.splice(1, 0, {
        label: 'Weekly (7d)',
        percent: util,
        resetTimestamp: parseIsoToUnixSeconds(safeString(sevenDayObj.resets_at)),
        kind: '7d',
        severity: null,
        known: true,
      });
    }
  }

  // Anything else window-shaped at the top level that `limits[]` didn't cover
  // (e.g. a newly-populated `seven_day_opus`, or a codenamed field like `nimbus_quill`).
  const HANDLED_TOP_LEVEL_KEYS = new Set(['five_hour', 'seven_day', 'limits', 'extra_usage', 'spend', 'member_dashboard_available']);
  for (const [key, value] of Object.entries(raw)) {
    if (HANDLED_TOP_LEVEL_KEYS.has(key)) continue;
    const obj = safeObject(value);
    if (!obj) continue; // null / non-object — not a populated window
    const util = safeNumber(obj.utilization);
    if (util === null) continue; // not window-shaped
    windows.push({
      label: `${humanizeKey(key)} (new)`,
      percent: util,
      resetTimestamp: parseIsoToUnixSeconds(safeString(obj.resets_at)),
      kind: null,
      severity: null,
      known: false,
    });
  }

  return windows;
}

/**
 * Get pastel color for ahead/under pace difference
 */
function getDiffColor(diffPct: number): string {
  if (diffPct > 15) return '#E89999';      // Soft red - way ahead
  if (diffPct > 5) return '#F4B8A4';       // Soft orange - ahead
  if (diffPct >= -5) return '#E8D4A2';     // Soft yellow - on track
  if (diffPct >= -15) return '#B8D99A';    // Soft lime - below (good)
  return '#A5D8DD';                        // Soft cyan - well under limit
}

/**
 * Get pastel color for reset time based on urgency
 */
function getResetColor(resetTimestamp: number): string {
  const now = Date.now() / 1000;
  const hoursRemaining = (resetTimestamp - now) / 3600;
  
  if (hoursRemaining < 1) return '#E89999';      // Soft red - < 1 hour
  if (hoursRemaining < 3) return '#F4B8A4';      // Soft orange - < 3 hours
  if (hoursRemaining < 12) return '#E8D4A2';     // Soft yellow - < 12 hours
  if (hoursRemaining < 48) return '#B8D99A';     // Soft lime - < 2 days
  return '#9DCCB4';                              // Soft teal - plenty of time
}

/**
 * Get status message based on pace difference
 */
function getPaceStatus(diffPct: number): string {
  if (diffPct > 15) return 'high usage rate';
  if (diffPct > 5) return 'slightly elevated';
  if (diffPct >= -5) return 'on track';
  if (diffPct >= -15) return 'comfortable margin';
  return 'well under limit';
}

/**
 * Calculate pace for 5-hour period (linear usage)
 */
function calculate5HourPace(utilization: number, resetTimestamp: number): PaceResult {
  const now = Date.now() / 1000;
  const secondsRemaining = resetTimestamp - now;
  const totalSeconds = 5 * 3600;
  const secondsElapsed = totalSeconds - secondsRemaining;

  const expectedUtil = secondsElapsed / totalSeconds;
  const diff = (utilization - expectedUtil) * 100;

  const pctUsed = utilization * 100;
  const hoursRemaining = secondsRemaining / 3600;
  const hoursElapsed = secondsElapsed / 3600;

  return {
    diff,
    status: getPaceStatus(diff),
    budgetRate: hoursRemaining > 0 ? (100 - pctUsed) / hoursRemaining : 0,
    avgRate: hoursElapsed > 0 ? pctUsed / hoursElapsed : 0,
    remainingHours: hoursRemaining,
  };
}

/**
 * Compute the offset (in fractional hours) into the current day-cycle.
 * A "cycle" runs from SLEEP_START_HOUR (2 AM) to the next SLEEP_START_HOUR.
 * Offset 0 = exactly 2:00 AM, offset 7 = 9:00 AM, offset 24 = next 2:00 AM.
 */
export function offsetInCycle(t: Date): number {
  const hours = t.getHours() + t.getMinutes() / 60 + t.getSeconds() / 3600;
  if (hours >= SLEEP_START_HOUR) return hours - SLEEP_START_HOUR;
  return hours + (24 - SLEEP_START_HOUR);
}

/**
 * Compute the start of the current day-cycle (the most recent 2:00 AM).
 */
export function getCycleStart(t: Date): Date {
  const result = new Date(t);
  if (t.getHours() >= SLEEP_START_HOUR) {
    result.setHours(SLEEP_START_HOUR, 0, 0, 0);
  } else {
    result.setDate(result.getDate() - 1);
    result.setHours(SLEEP_START_HOUR, 0, 0, 0);
  }
  return result;
}

/**
 * Compute active hours within a partial cycle given the offset from cycle start.
 * Sleep occupies offsets [0, SLEEP_HOURS_PER_DAY), active occupies [SLEEP_HOURS_PER_DAY, 24).
 * Returns a value in [0, ACTIVE_HOURS_PER_DAY].
 */
export function activeHoursInPartialCycle(offsetHours: number): number {
  if (offsetHours <= SLEEP_HOURS_PER_DAY) return 0;
  return Math.min(offsetHours - SLEEP_HOURS_PER_DAY, ACTIVE_HOURS_PER_DAY);
}

/**
 * Compute total elapsed active hours between two timestamps.
 * Uses a cycle-based model (2 AM to 2 AM) that is continuous and monotonically
 * increasing. During sleep hours (2 AM - 9 AM), the value is flat.
 * During active hours (9 AM - 2 AM), it increases linearly.
 */
export function elapsedActiveHoursBetween(start: Date, end: Date): number {
  const startCycleStart = getCycleStart(start);
  const endCycleStart = getCycleStart(end);

  const completeCycles = Math.round(
    (endCycleStart.getTime() - startCycleStart.getTime()) / 86_400_000
  );

  const startActiveInCycle = activeHoursInPartialCycle(offsetInCycle(start));
  const endActiveInCycle = activeHoursInPartialCycle(offsetInCycle(end));

  return completeCycles * ACTIVE_HOURS_PER_DAY + endActiveInCycle - startActiveInCycle;
}

/**
 * Calculate pace for 7-day period (accounting for active hours).
 * Uses cycle-based model: 9 AM to 2 AM active, 2 AM to 9 AM sleep.
 */
function calculate7DayPace(utilization: number, resetTimestamp: number): PaceResult {
  const now = new Date();
  const resetDate = new Date(resetTimestamp * 1000);
  const periodStart = new Date(resetDate.getTime() - 7 * 24 * 3600 * 1000);
  
  const totalActiveHours = 7 * ACTIVE_HOURS_PER_DAY;
  const elapsed = elapsedActiveHoursBetween(periodStart, now);
  const remainingActive = elapsedActiveHoursBetween(now, resetDate);

  const expectedUtil = elapsed / totalActiveHours;
  const diff = (utilization - expectedUtil) * 100;

  const pctUsed = utilization * 100;

  return {
    diff,
    status: getPaceStatus(diff),
    budgetRate: remainingActive > 0 ? (100 - pctUsed) / remainingActive : 0,
    avgRate: elapsed > 0 ? pctUsed / elapsed : 0,
    remainingHours: remainingActive,
  };
}

/**
 * Format reset time from Unix timestamp with relative time and contextual absolute time
 */
function formatResetTime(resetTimestamp: number): string {
  if (!resetTimestamp) return '';
  
  const resetDate = new Date(resetTimestamp * 1000); // Convert to milliseconds
  const now = new Date();
  const diffMs = resetDate.getTime() - now.getTime();
  
  if (diffMs < 0) return 'reset overdue';
  
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffMins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
  
  // Format time in local timezone
  const timeStr = resetDate.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true
  });
  
  // Calculate day difference
  const nowDay = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const resetDay = new Date(resetDate.getFullYear(), resetDate.getMonth(), resetDate.getDate());
  const dayDiff = Math.round((resetDay.getTime() - nowDay.getTime()) / (1000 * 60 * 60 * 24));
  
  let dateContext: string;
  if (dayDiff === 0) {
    dateContext = `${timeStr} today`;
  } else if (dayDiff === 1) {
    dateContext = `${timeStr} tomorrow`;
  } else if (dayDiff < 7) {
    // Day of week (abbreviated)
    const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    const dayName = dayNames[resetDate.getDay()];
    dateContext = `${dayName} ${timeStr}`;
  } else {
    // Check if it's exactly 7 days (same day of week)
    const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    const dayName = dayNames[resetDate.getDay()];
    if (resetDate.getDay() === now.getDay()) {
      dateContext = `next ${dayName} ${timeStr}`;
    } else {
      dateContext = `${dayName} ${timeStr}`;
    }
  }
  
  let relativeTime: string;
  if (diffHours < 24) {
    relativeTime = `${diffHours}h ${diffMins}m`;
  } else {
    const diffDays = Math.floor(diffHours / 24);
    const remainingHours = diffHours % 24;
    relativeTime = `${diffDays}d ${remainingHours}h`;
  }
  
  return `resets in ${relativeTime} (${dateContext})`;
}

/**
 * Format and display usage output with pastel colors and condensed layout
 */
function formatOutput(usage: UsageData): void {
  const fiveHourPct = usage.five_hour.utilization * 100;
  const sevenDayPct = usage.seven_day.utilization * 100;
  
  const fiveHourColor = getColorForPercentage(fiveHourPct);
  const sevenDayColor = getColorForPercentage(sevenDayPct);
  
  const fiveHourReset = formatResetTime(usage.five_hour.reset);
  const sevenDayReset = formatResetTime(usage.seven_day.reset);
  
  const fiveHourResetColor = getResetColor(usage.five_hour.reset);
  const sevenDayResetColor = getResetColor(usage.seven_day.reset);
  
  const fiveHourPace = calculate5HourPace(usage.five_hour.utilization, usage.five_hour.reset);
  const sevenDayPace = calculate7DayPace(usage.seven_day.utilization, usage.seven_day.reset);
  
  const fiveHourDiffColor = getDiffColor(fiveHourPace.diff);
  const sevenDayDiffColor = getDiffColor(sevenDayPace.diff);
  
  const fiveHourPctStr = fiveHourPct.toFixed(2) + '%';
  const sevenDayPctStr = sevenDayPct.toFixed(2) + '%';
  
  const fiveHourDiffStr = (fiveHourPace.diff >= 0 ? '+' : '') + fiveHourPace.diff.toFixed(2) + '%';
  const sevenDayDiffStr = (sevenDayPace.diff >= 0 ? '+' : '') + sevenDayPace.diff.toFixed(2) + '%';
  
  // Pastel color palette
  const headerColor = chalk.hex('#9CA3AF');  // Gray 400 - header
  const labelColor = chalk.hex('#6B7280');   // Gray 500 - period labels
  const bulletColor = chalk.hex('#9CA3AF');  // Gray 400 - bullets
  
  const fiveHourBudgetStr = fiveHourPace.remainingHours >= BUDGET_SUPPRESS_THRESHOLD_HOURS
    ? `budget ${fiveHourPace.budgetRate.toFixed(2)}%/h (avg ${fiveHourPace.avgRate.toFixed(2)}%/h)`
    : null;
  const sevenDayBudgetStr = sevenDayPace.remainingHours >= BUDGET_SUPPRESS_THRESHOLD_HOURS
    ? `budget ${sevenDayPace.budgetRate.toFixed(2)}%/h (avg ${sevenDayPace.avgRate.toFixed(2)}%/h)`
    : null;

  const fiveHourSegments = [
    chalk.hex(fiveHourColor)(fiveHourPctStr),
    `${chalk.hex(fiveHourDiffColor)(fiveHourPace.status)} ${chalk.hex(fiveHourDiffColor)(`(${fiveHourDiffStr} ${fiveHourPace.diff >= 0 ? 'ahead' : 'under'})`)}`,
    fiveHourBudgetStr ? labelColor(fiveHourBudgetStr) : null,
    chalk.hex(fiveHourResetColor)(fiveHourReset),
  ].filter((segment): segment is string => segment !== null);

  const sevenDaySegments = [
    chalk.hex(sevenDayColor)(sevenDayPctStr),
    `${chalk.hex(sevenDayDiffColor)(sevenDayPace.status)} ${chalk.hex(sevenDayDiffColor)(`(${sevenDayDiffStr} ${sevenDayPace.diff >= 0 ? 'ahead' : 'below'})`)}`,
    sevenDayBudgetStr ? labelColor(sevenDayBudgetStr) : null,
    chalk.hex(sevenDayResetColor)(sevenDayReset),
  ].filter((segment): segment is string => segment !== null);

  console.log(headerColor('Usage:'));
  console.log(`  ${labelColor('Hourly (5h)')} ${bulletColor('·')} ${fiveHourSegments.join(` ${bulletColor('•')} `)}`);
  console.log(`  ${labelColor('Weekly (7d)')} ${bulletColor('·')} ${sevenDaySegments.join(` ${bulletColor('•')} `)}`);
}

/**
 * Render one DisplayWindow as a line. Known 5h/7d windows get the full pace treatment
 * (status, diff, budget rate) via the same calculate5HourPace/calculate7DayPace used by
 * the legacy formatter. Unrecognized windows (kind === null) just show percent + reset —
 * guessing a period for pace math on a window we've never seen before would be worse
 * than not showing one.
 */
function buildWindowLine(w: DisplayWindow): string {
  const labelColor = chalk.hex('#6B7280');
  const bulletColor = chalk.hex('#9CA3AF');

  const pctColor = getColorForPercentage(w.percent);
  const segments: string[] = [chalk.hex(pctColor)(w.percent.toFixed(2) + '%')];

  if (w.kind && w.resetTimestamp !== null) {
    const utilFraction = w.percent / 100;
    const pace = w.kind === '5h'
      ? calculate5HourPace(utilFraction, w.resetTimestamp)
      : calculate7DayPace(utilFraction, w.resetTimestamp);
    const diffColor = getDiffColor(pace.diff);
    const diffStr = (pace.diff >= 0 ? '+' : '') + pace.diff.toFixed(2) + '%';
    const direction = pace.diff >= 0 ? 'ahead' : 'under';

    segments.push(`${chalk.hex(diffColor)(pace.status)} ${chalk.hex(diffColor)(`(${diffStr} ${direction})`)}`);
    if (pace.remainingHours >= BUDGET_SUPPRESS_THRESHOLD_HOURS) {
      segments.push(labelColor(`budget ${pace.budgetRate.toFixed(2)}%/h (avg ${pace.avgRate.toFixed(2)}%/h)`));
    }
  } else if (w.severity && w.severity !== 'normal') {
    segments.push(chalk.hex('#F4B8A4')(w.severity));
  }

  segments.push(
    w.resetTimestamp !== null
      ? chalk.hex(getResetColor(w.resetTimestamp))(formatResetTime(w.resetTimestamp))
      : labelColor('no reset info')
  );

  if (!w.known) {
    segments.push(chalk.hex('#9CA3AF')('unrecognized — schema may have changed'));
  }

  return `${labelColor(w.label)} ${bulletColor('·')} ${segments.join(` ${bulletColor('•')} `)}`;
}

/**
 * Format output from the dynamic /api/oauth/usage endpoint. Every window is rendered
 * independently, so a parse failure or unexpected shape on one window degrades to an
 * error line for that window rather than taking down the whole display.
 */
function formatDynamicOutput(raw: OAuthUsageResponse): void {
  const headerColor = chalk.hex('#9CA3AF');
  const labelColor = chalk.hex('#6B7280');
  const bulletColor = chalk.hex('#9CA3AF');
  const errorColor = chalk.hex('#E89999');

  let windows: DisplayWindow[];
  try {
    windows = buildDisplayWindows(raw);
  } catch (error) {
    console.log(headerColor('Usage:'));
    console.log(`  ${errorColor('Failed to interpret /api/oauth/usage response')} ${bulletColor('•')} ${labelColor(error instanceof Error ? error.message : 'unknown error')}`);
    if (VERBOSE) console.log(labelColor(JSON.stringify(raw, null, 2)));
    return;
  }

  if (windows.length === 0) {
    console.log(headerColor('Usage:'));
    console.log(`  ${errorColor('No recognizable usage windows in response')} ${bulletColor('•')} ${labelColor('run with -v to inspect the raw payload')}`);
    return;
  }

  console.log(headerColor('Usage:'));
  for (const w of windows) {
    try {
      console.log(`  ${buildWindowLine(w)}`);
    } catch (error) {
      console.log(`  ${labelColor(w.label)} ${bulletColor('•')} ${errorColor('failed to render')} (${error instanceof Error ? error.message : 'unknown error'})`);
    }
  }
}

/**
 * Main entry point
 */
async function main() {
  const spinner = new Spinner();
  
  try {
    if (VERBOSE) {
      console.log(chalk.hex('#6B7280')('=== Debug: Configuration ==='));
      console.log(chalk.hex('#9CA3AF')('Claude Code path:'), CLAUDE_CODE_PATH);
      console.log(chalk.hex('#9CA3AF')('OpenCode path:'), OPENCODE_PATH);
      console.log(chalk.hex('#9CA3AF')('Verbose mode:'), VERBOSE);
      console.log();
    }
    
    const tokenResult = await readToken();
    
    if (VERBOSE) {
      console.log(chalk.hex('#6B7280')('=== Debug: Authentication ==='));
      console.log(chalk.hex('#9CA3AF')('Token source:'), tokenResult.source);
      console.log(chalk.hex('#9CA3AF')('Token loaded:'), `${tokenResult.token.substring(0, 20)}...${tokenResult.token.substring(tokenResult.token.length - 10)}`);
      console.log(chalk.hex('#9CA3AF')('Token length:'), tokenResult.token.length);
      if (tokenResult.expiresAt) {
        const expiresDate = new Date(tokenResult.expiresAt);
        const isExpired = tokenResult.expiresAt < Date.now();
        console.log(chalk.hex('#9CA3AF')('Token expires:'), expiresDate.toLocaleString(), isExpired ? chalk.hex('#E89999')('(EXPIRED)') : chalk.hex('#9DCCB4')('(valid)'));
      }
      console.log();
    }

    if (!VERBOSE) {
      spinner.start('Fetching usage data...');
    }

    if (LEGACY) {
      const usage = await fetchUsage(tokenResult.token, tokenResult.source);
      if (!VERBOSE) spinner.stop();
      formatOutput(usage);
    } else {
      const raw = await fetchOAuthUsage(tokenResult.token, tokenResult.source);
      if (!VERBOSE) spinner.stop();
      formatDynamicOutput(raw);
    }

  } catch (error) {
    spinner.stop();
    
    if (error instanceof Error) {
      console.error(error.message);
    } else {
      console.error('Error: Unknown error occurred');
    }
    process.exit(1);
  }
}

main();
