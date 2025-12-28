#!/usr/bin/env bun
/**
 * Font Installer for Chezmoi
 *
 * Downloads and installs fonts from Google Fonts based on ~/.config/fontconfig/fonts.toml
 * Uses google-webfonts-helper API (no auth required).
 * Also supports GitHub-sourced fonts (like Iosevka) via special handlers.
 *
 * Usage:
 *   install-fonts.ts          # Install fonts from config
 *   install-fonts.ts --list   # List all available fonts
 *   install-fonts.ts --search <query>  # Search for fonts
 */

import { existsSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join, basename } from "node:path";
import { parseArgs } from "node:util";
import { $ } from "bun";

// ============================================================================
// Types
// ============================================================================

interface GoogleFont {
  id: string;
  family: string;
  variants: string[];
  subsets: string[];
  category: string;
  version: string;
  lastModified: string;
  popularity: number;
  defSubset: string;
  defVariant: string;
}

interface FontVariant {
  id: string;
  fontFamily: string;
  fontStyle: string;
  fontWeight: string;
  woff: string;
  woff2: string;
  ttf: string;
}

interface FontDetails extends GoogleFont {
  variants: FontVariant[];
}

interface FontConfig {
  [category: string]: {
    primary: string;
    fallback?: string;
  };
}

interface FuseResult<T> {
  item: T;
  refIndex: number;
  score?: number;
}

// ============================================================================
// Constants
// ============================================================================

const FONTS_DIR = join(homedir(), ".local", "share", "fonts");
const CONFIG_PATH = join(homedir(), ".config", "fontconfig", "fonts.toml");
const API_BASE = "https://gwfh.mranftl.com/api";
const CACHE_FILE = join(homedir(), ".cache", "font-catalog.json");
const CACHE_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

// GitHub-sourced fonts not available on Google Fonts
// Maps font name to { repo, assetPattern, variant? }
interface GitHubFontConfig {
  repo: string;
  assetPattern: RegExp;
  variant?: string; // For Iosevka variants
}

const GITHUB_FONTS: Record<string, GitHubFontConfig> = {
  Iosevka: {
    repo: "be5invis/Iosevka",
    assetPattern: /^PkgTTF-Iosevka-[\d.]+\.zip$/,
  },
  "Iosevka Fixed": {
    repo: "be5invis/Iosevka",
    assetPattern: /^PkgTTF-IosevkaFixed-[\d.]+\.zip$/,
  },
  "Iosevka Slab": {
    repo: "be5invis/Iosevka",
    assetPattern: /^PkgTTF-IosevkaSlab-[\d.]+\.zip$/,
  },
  "Iosevka Curly": {
    repo: "be5invis/Iosevka",
    assetPattern: /^PkgTTF-IosevkaCurly-[\d.]+\.zip$/,
  },
  "Iosevka Aile": {
    repo: "be5invis/Iosevka",
    assetPattern: /^PkgTTF-IosevkaAile-[\d.]+\.zip$/,
  },
  "Iosevka Etoile": {
    repo: "be5invis/Iosevka",
    assetPattern: /^PkgTTF-IosevkaEtoile-[\d.]+\.zip$/,
  },
};

// ANSI colors
const RED = "\x1b[31m";
const GREEN = "\x1b[32m";
const YELLOW = "\x1b[33m";
const BLUE = "\x1b[34m";
const CYAN = "\x1b[36m";
const RESET = "\x1b[0m";
const BOLD = "\x1b[1m";

// ============================================================================
// Logging
// ============================================================================

const log = {
  info: (msg: string) => console.log(`${BLUE}[info]${RESET} ${msg}`),
  success: (msg: string) => console.log(`${GREEN}[ok]${RESET} ${msg}`),
  warn: (msg: string) => console.log(`${YELLOW}[warn]${RESET} ${msg}`),
  error: (msg: string) => console.log(`${RED}[error]${RESET} ${msg}`),
  step: (msg: string) => console.log(`${CYAN}>>>${RESET} ${msg}`),
};

// ============================================================================
// Simple Fuzzy Matching (no external deps)
// ============================================================================

function fuzzyScore(needle: string, haystack: string): number {
  const n = needle.toLowerCase();
  const h = haystack.toLowerCase();

  // Exact match
  if (n === h) return 1.0;

  // Starts with
  if (h.startsWith(n)) return 0.9;

  // Contains
  if (h.includes(n)) return 0.7;

  // Subsequence match
  let ni = 0;
  let consecutiveBonus = 0;
  let lastMatchIdx = -2;

  for (let hi = 0; hi < h.length && ni < n.length; hi++) {
    if (h[hi] === n[ni]) {
      if (hi === lastMatchIdx + 1) {
        consecutiveBonus += 0.1;
      }
      lastMatchIdx = hi;
      ni++;
    }
  }

  if (ni === n.length) {
    const baseScore = 0.3 + (n.length / h.length) * 0.3;
    return Math.min(baseScore + consecutiveBonus, 0.65);
  }

  return 0;
}

function fuzzySearch<T>(
  items: T[],
  query: string,
  getKey: (item: T) => string,
  threshold = 0.3,
  limit = 5
): FuseResult<T>[] {
  const results: FuseResult<T>[] = [];

  for (let i = 0; i < items.length; i++) {
    const score = fuzzyScore(query, getKey(items[i]));
    if (score >= threshold) {
      results.push({ item: items[i], refIndex: i, score });
    }
  }

  return results
    .sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
    .slice(0, limit);
}

// ============================================================================
// API Functions
// ============================================================================

async function fetchWithRetry(
  url: string,
  retries = 3
): Promise<Response | null> {
  for (let i = 0; i < retries; i++) {
    try {
      const response = await fetch(url);
      if (response.ok) return response;
      if (response.status === 404) return null;
      log.warn(`HTTP ${response.status} for ${url}, retrying...`);
    } catch (e) {
      log.warn(`Network error for ${url}, retrying... (${i + 1}/${retries})`);
    }
    await Bun.sleep(1000 * (i + 1));
  }
  return null;
}

async function fetchFontCatalog(): Promise<GoogleFont[]> {
  // Check cache
  if (existsSync(CACHE_FILE)) {
    const stat = Bun.file(CACHE_FILE);
    const mtime = (await stat.stat()).mtime;
    if (Date.now() - mtime.getTime() < CACHE_MAX_AGE_MS) {
      try {
        const cached = await Bun.file(CACHE_FILE).json();
        log.info(`Using cached font catalog (${cached.length} fonts)`);
        return cached;
      } catch {
        // Cache corrupt, fetch fresh
      }
    }
  }

  log.step("Fetching font catalog from google-webfonts-helper...");
  const response = await fetchWithRetry(`${API_BASE}/fonts`);

  if (!response) {
    throw new Error("Failed to fetch font catalog");
  }

  const fonts: GoogleFont[] = await response.json();
  log.success(`Fetched ${fonts.length} fonts`);

  // Cache the catalog
  const cacheDir = join(homedir(), ".cache");
  if (!existsSync(cacheDir)) {
    mkdirSync(cacheDir, { recursive: true });
  }
  await Bun.write(CACHE_FILE, JSON.stringify(fonts));

  return fonts;
}

async function fetchFontDetails(fontId: string): Promise<FontDetails | null> {
  // Emoji fonts don't have latin subset, so we handle them specially
  const isEmoji = fontId.toLowerCase().includes("emoji");
  const url = isEmoji
    ? `${API_BASE}/fonts/${fontId}`
    : `${API_BASE}/fonts/${fontId}?subsets=latin,latin-ext`;

  const response = await fetchWithRetry(url);
  if (!response) return null;
  return response.json();
}

// ============================================================================
// Font Installation
// ============================================================================

async function downloadFont(
  url: string,
  destPath: string
): Promise<boolean> {
  try {
    const response = await fetchWithRetry(url);
    if (!response) return false;

    const buffer = await response.arrayBuffer();
    await Bun.write(destPath, buffer);
    return true;
  } catch (e) {
    log.error(`Failed to download: ${url}`);
    return false;
  }
}

// ============================================================================
// GitHub Font Installation (Iosevka, etc.)
// ============================================================================

interface GitHubRelease {
  tag_name: string;
  assets: Array<{
    name: string;
    browser_download_url: string;
  }>;
}

async function installGitHubFont(fontName: string): Promise<boolean> {
  const config = GITHUB_FONTS[fontName];
  if (!config) return false;

  const fontDir = join(FONTS_DIR, fontName.replace(/\s+/g, ""));

  // Check if already installed
  if (existsSync(fontDir)) {
    const files = readdirSync(fontDir).filter((f) => f.endsWith(".ttf"));
    if (files.length > 0) {
      log.success(`${fontName} already installed (${files.length} files)`);
      return true;
    }
  }

  log.step(`Installing ${fontName} from GitHub...`);

  // Fetch latest release
  const releaseUrl = `https://api.github.com/repos/${config.repo}/releases/latest`;
  const response = await fetchWithRetry(releaseUrl);
  if (!response) {
    log.error(`Failed to fetch release info for ${fontName}`);
    return false;
  }

  const release: GitHubRelease = await response.json();
  const asset = release.assets.find((a) => config.assetPattern.test(a.name));

  if (!asset) {
    log.error(`No matching asset found for ${fontName} in ${release.tag_name}`);
    log.info(`Available assets: ${release.assets.map((a) => a.name).join(", ")}`);
    return false;
  }

  // Download ZIP
  log.info(`Downloading ${asset.name}...`);
  const zipResponse = await fetchWithRetry(asset.browser_download_url);
  if (!zipResponse) {
    log.error(`Failed to download ${asset.name}`);
    return false;
  }

  const zipPath = join(tmpdir(), asset.name);
  const zipBuffer = await zipResponse.arrayBuffer();
  await Bun.write(zipPath, zipBuffer);

  // Create font directory
  if (!existsSync(fontDir)) {
    mkdirSync(fontDir, { recursive: true });
  }

  // Extract ZIP using unzip command
  try {
    await $`unzip -o -j ${zipPath} "*.ttf" -d ${fontDir}`.quiet();
  } catch (e) {
    log.error(`Failed to extract ${asset.name}`);
    rmSync(zipPath, { force: true });
    return false;
  }

  // Clean up ZIP
  rmSync(zipPath, { force: true });

  const files = readdirSync(fontDir).filter((f) => f.endsWith(".ttf"));
  log.success(`Installed ${fontName}: ${files.length} files (${release.tag_name})`);

  return true;
}

function isGitHubFont(fontName: string): boolean {
  return fontName in GITHUB_FONTS;
}

function variantToFilename(family: string, variant: FontVariant): string {
  const weight = variant.fontWeight;
  const style = variant.fontStyle === "italic" ? "Italic" : "";
  const weightName = weightToName(parseInt(weight));

  return `${family.replace(/\s+/g, "")}-${weightName}${style}.ttf`;
}

function weightToName(weight: number): string {
  const weights: Record<number, string> = {
    100: "Thin",
    200: "ExtraLight",
    300: "Light",
    400: "Regular",
    500: "Medium",
    600: "SemiBold",
    700: "Bold",
    800: "ExtraBold",
    900: "Black",
  };
  return weights[weight] ?? `W${weight}`;
}

async function installFont(
  fontName: string,
  catalog: GoogleFont[]
): Promise<boolean> {
  // Fuzzy search for the font
  const results = fuzzySearch(catalog, fontName, (f) => f.family, 0.3, 5);

  if (results.length === 0) {
    log.error(`Font "${fontName}" not found in Google Fonts.`);

    // Check if there's a GitHub font match
    const githubFontNames = Object.keys(GITHUB_FONTS);
    const githubMatches = fuzzySearch(
      githubFontNames,
      fontName,
      (name) => name,
      0.3,
      3
    );
    if (githubMatches.length > 0) {
      const suggestions = githubMatches.map((r) => r.item).join(", ");
      log.info(`GitHub fonts available: ${suggestions}`);
      log.info(`Use the exact name in fonts.toml to install.`);
      return false;
    }

    const looseSuggestions = fuzzySearch(
      catalog,
      fontName,
      (f) => f.family,
      0.2,
      5
    );
    if (looseSuggestions.length > 0) {
      const suggestions = looseSuggestions.map((r) => r.item.family).join(", ");
      log.info(`Did you mean: ${suggestions}?`);
    }
    return false;
  }

  const bestMatch = results[0];
  const font = bestMatch.item;

  // Warn if not an exact match
  if (font.family.toLowerCase() !== fontName.toLowerCase()) {
    log.warn(
      `"${fontName}" matched to "${font.family}" (score: ${(bestMatch.score ?? 0).toFixed(2)})`
    );
  }

  const fontDir = join(FONTS_DIR, font.family.replace(/\s+/g, ""));

  // Check if already installed (with at least some TTF files)
  if (existsSync(fontDir)) {
    const files = readdirSync(fontDir).filter((f) => f.endsWith(".ttf"));
    if (files.length > 0) {
      log.success(`${font.family} already installed (${files.length} files)`);
      return true;
    }
  }

  log.step(`Installing ${font.family}...`);

  // Fetch font details
  const details = await fetchFontDetails(font.id);
  if (!details) {
    log.error(`Failed to fetch details for ${font.family}`);
    return false;
  }

  // Create font directory
  if (!existsSync(fontDir)) {
    mkdirSync(fontDir, { recursive: true });
  }

  // Download all variants
  let successCount = 0;
  let failCount = 0;

  for (const variant of details.variants) {
    if (typeof variant === "string") continue; // Skip if just variant name

    const filename = variantToFilename(font.family, variant);
    const destPath = join(fontDir, filename);

    // Prefer TTF, fall back to WOFF2
    const url = variant.ttf || variant.woff2;
    if (!url) {
      log.warn(`No download URL for ${filename}`);
      failCount++;
      continue;
    }

    const success = await downloadFont(url, destPath);
    if (success) {
      successCount++;
    } else {
      failCount++;
    }
  }

  if (successCount > 0) {
    log.success(
      `Installed ${font.family}: ${successCount} files` +
        (failCount > 0 ? ` (${failCount} failed)` : "")
    );
    return true;
  } else {
    log.error(`Failed to install any files for ${font.family}`);
    // Clean up empty directory
    if (existsSync(fontDir) && readdirSync(fontDir).length === 0) {
      rmSync(fontDir);
    }
    return false;
  }
}

// ============================================================================
// Configuration
// ============================================================================

async function loadConfig(): Promise<FontConfig> {
  if (!existsSync(CONFIG_PATH)) {
    throw new Error(`Config not found: ${CONFIG_PATH}`);
  }

  const content = await Bun.file(CONFIG_PATH).text();

  // Simple TOML parser for our specific format
  const config: FontConfig = {};
  let currentSection = "";

  for (const line of content.split("\n")) {
    const trimmed = line.trim();

    // Skip comments and empty lines
    if (!trimmed || trimmed.startsWith("#")) continue;

    // Section header
    const sectionMatch = trimmed.match(/^\[(\w+)\]$/);
    if (sectionMatch) {
      currentSection = sectionMatch[1];
      config[currentSection] = { primary: "" };
      continue;
    }

    // Key-value pair
    const kvMatch = trimmed.match(/^(\w+)\s*=\s*"([^"]+)"$/);
    if (kvMatch && currentSection) {
      const [, key, value] = kvMatch;
      if (key === "primary" || key === "fallback") {
        config[currentSection][key] = value;
      }
    }
  }

  return config;
}

// ============================================================================
// Main Commands
// ============================================================================

async function listFonts(): Promise<void> {
  const catalog = await fetchFontCatalog();

  console.log(`\n${BOLD}Available Fonts (${catalog.length} total)${RESET}\n`);

  // Group by category
  const byCategory: Record<string, GoogleFont[]> = {};
  for (const font of catalog) {
    const cat = font.category || "other";
    if (!byCategory[cat]) byCategory[cat] = [];
    byCategory[cat].push(font);
  }

  for (const [category, fonts] of Object.entries(byCategory).sort()) {
    console.log(`${CYAN}${category}${RESET} (${fonts.length}):`);
    const names = fonts
      .sort((a, b) => a.family.localeCompare(b.family))
      .slice(0, 10)
      .map((f) => f.family)
      .join(", ");
    console.log(`  ${names}${fonts.length > 10 ? ", ..." : ""}`);
    console.log();
  }
}

async function searchFonts(query: string): Promise<void> {
  const catalog = await fetchFontCatalog();
  const results = fuzzySearch(catalog, query, (f) => f.family, 0.2, 20);

  // Also search GitHub fonts
  const githubFontNames = Object.keys(GITHUB_FONTS);
  const githubResults = fuzzySearch(
    githubFontNames,
    query,
    (name) => name,
    0.2,
    10
  );

  if (results.length === 0 && githubResults.length === 0) {
    log.warn(`No fonts matching "${query}"`);
    return;
  }

  console.log(`\n${BOLD}Search results for "${query}"${RESET}\n`);

  // Show GitHub fonts first (marked as such)
  for (const result of githubResults) {
    const fontName = result.item;
    const score = ((result.score ?? 0) * 100).toFixed(0);
    console.log(
      `  ${GREEN}${fontName}${RESET} (monospace) - via GitHub [${score}% match]`
    );
  }

  // Then Google Fonts
  for (const result of results) {
    const font = result.item;
    const score = ((result.score ?? 0) * 100).toFixed(0);
    const variants = Array.isArray(font.variants) ? font.variants.length : "?";
    console.log(
      `  ${GREEN}${font.family}${RESET} (${font.category}) - ${variants} variants [${score}% match]`
    );
  }
  console.log();
}

async function installFromConfig(): Promise<void> {
  log.step("Loading font configuration...");
  const config = await loadConfig();

  log.step("Fetching font catalog...");
  const catalog = await fetchFontCatalog();

  // Ensure fonts directory exists
  if (!existsSync(FONTS_DIR)) {
    mkdirSync(FONTS_DIR, { recursive: true });
  }

  // Collect all fonts to install
  const fontsToInstall: string[] = [];
  for (const category of Object.values(config)) {
    if (category.primary) fontsToInstall.push(category.primary);
    if (category.fallback) fontsToInstall.push(category.fallback);
  }

  // Remove duplicates
  const uniqueFonts = [...new Set(fontsToInstall)];

  log.info(`Installing ${uniqueFonts.length} font families...`);
  console.log();

  let successCount = 0;
  let failCount = 0;

  for (const fontName of uniqueFonts) {
    let success: boolean;

    // Check if this is a GitHub-sourced font
    if (isGitHubFont(fontName)) {
      success = await installGitHubFont(fontName);
    } else {
      success = await installFont(fontName, catalog);
    }

    if (success) {
      successCount++;
    } else {
      failCount++;
    }
  }

  console.log();

  // Rebuild font cache
  log.step("Rebuilding font cache...");
  try {
    await $`fc-cache -f`.quiet();
    log.success("Font cache updated");
  } catch (e) {
    log.warn("Failed to update font cache (fc-cache not available?)");
  }

  // Summary
  console.log();
  console.log(`${BOLD}Summary${RESET}`);
  console.log(`  ${GREEN}Installed:${RESET} ${successCount}`);
  if (failCount > 0) {
    console.log(`  ${RED}Failed:${RESET} ${failCount}`);
  }
}

// ============================================================================
// Entry Point
// ============================================================================

async function main(): Promise<void> {
  const { values, positionals } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      list: { type: "boolean", short: "l" },
      search: { type: "string", short: "s" },
      help: { type: "boolean", short: "h" },
    },
    allowPositionals: true,
  });

  if (values.help) {
    console.log(`
${BOLD}Font Installer for Chezmoi${RESET}

${CYAN}Usage:${RESET}
  install-fonts.ts              Install fonts from config
  install-fonts.ts --list       List all available fonts
  install-fonts.ts --search <q> Search for fonts
  install-fonts.ts --help       Show this help

${CYAN}Config:${RESET} ${CONFIG_PATH}
${CYAN}Fonts:${RESET}  ${FONTS_DIR}
`);
    return;
  }

  if (values.list) {
    await listFonts();
    return;
  }

  if (values.search) {
    await searchFonts(values.search);
    return;
  }

  // Handle positional argument as search
  if (positionals.length > 0) {
    await searchFonts(positionals.join(" "));
    return;
  }

  // Default: install from config
  await installFromConfig();
}

main().catch((e) => {
  log.error(e.message);
  process.exit(1);
});
