// CLI argument parsing

import type { CliArgs, FormatOption, ResolutionOption, FpsOption, EncoderOption, ImageQuality, SourceConfig } from "./types";

const HELP_TEXT = `
share - Interactive file uploader to R2 with TUI

Usage:
  share [options] [file]

Options:
  --dry-run              Skip upload step (test processing only)
  --format=<fmt>         Format: original, png, jpeg, webp, mp4, webm, gif
  --resolution=<res>     Resolution: original, 1080p, 720p, 480p, 50%, 25%
  --fps=<fps>            Frame rate: original, 60, 30, 24, 15
  --encoder=<enc>        Encoder: cpu, nvenc, av1
  --crf=<num>            CRF quality (lower = higher quality, 0-51)
  --probe                Force quality probing (analyze multiple CRF values)
  --no-probe             Skip quality probing (use default CRF)
  --remove-audio         Remove audio track
  --keep-audio           Keep audio track
  --quality=<q>          Image quality: high, balanced, small
  --random-filename      Generate random filename
  --normalize-filename   Normalize filename (lowercase, no spaces)
  --auto-fix             Auto-fix detected issues
  --no-auto-fix          Don't auto-fix issues
  --gif-fps=<fps>        GIF frame rate (10, 15, 24)
  --gif-width=<px>       GIF width in pixels
  --verbose, -v          Show detailed logging (ffmpeg commands, timing)
  -h, --help             Show this help

Examples:
  share video.mp4 --dry-run --format=webm --resolution=720p
  share screenshot.png --format=webp --quality=small
  cat video.mkv | share --encoder=nvenc --crf=23 --remove-audio
`;

export interface ParseResult {
  cliArgs: CliArgs;
  source: SourceConfig;
  showHelp: boolean;
}

export function parseCliArgs(args: string[]): ParseResult {
  const cliArgs: CliArgs = { dryRun: false };
  let filePath: string | undefined;
  let showHelp = false;

  for (const arg of args) {
    if (arg === "--dry-run") {
      cliArgs.dryRun = true;
    } else if (arg === "--help" || arg === "-h") {
      showHelp = true;
    } else if (arg.startsWith("--format=")) {
      cliArgs.format = arg.split("=")[1] as FormatOption;
    } else if (arg.startsWith("--resolution=")) {
      cliArgs.resolution = arg.split("=")[1] as ResolutionOption;
    } else if (arg.startsWith("--fps=")) {
      cliArgs.fps = arg.split("=")[1] as FpsOption;
    } else if (arg.startsWith("--encoder=")) {
      cliArgs.encoder = arg.split("=")[1] as EncoderOption;
    } else if (arg.startsWith("--crf=")) {
      cliArgs.crf = parseInt(arg.split("=")[1], 10);
    } else if (arg === "--remove-audio") {
      cliArgs.removeAudio = true;
    } else if (arg === "--keep-audio") {
      cliArgs.removeAudio = false;
    } else if (arg.startsWith("--quality=")) {
      cliArgs.imageQuality = arg.split("=")[1] as ImageQuality;
    } else if (arg === "--random-filename") {
      cliArgs.randomFilename = true;
    } else if (arg === "--normalize-filename") {
      cliArgs.normalizeFilename = true;
    } else if (arg === "--auto-fix") {
      cliArgs.autoFix = true;
    } else if (arg === "--no-auto-fix") {
      cliArgs.autoFix = false;
    } else if (arg === "--probe") {
      cliArgs.probe = true;
    } else if (arg === "--no-probe") {
      cliArgs.noProbe = true;
    } else if (arg.startsWith("--gif-fps=")) {
      cliArgs.gifFps = parseInt(arg.split("=")[1], 10);
    } else if (arg.startsWith("--gif-width=")) {
      cliArgs.gifWidth = parseInt(arg.split("=")[1], 10);
    } else if (arg === "--verbose" || arg === "-v") {
      cliArgs.verbose = true;
    } else if (!arg.startsWith("-")) {
      filePath = arg;
    }
  }

  // Determine source type
  const source: SourceConfig = filePath
    ? { type: "file", path: filePath }
    : hasStdinData()
      ? { type: "stdin" }
      : { type: "clipboard" };

  return { cliArgs, source, showHelp };
}

export function printHelp(): void {
  console.log(HELP_TEXT);
}

export function isInteractive(): boolean {
  return process.stdout.isTTY === true;
}

// Check if stdin has data (without importing io.ts to avoid circular deps)
function hasStdinData(): boolean {
  if (Bun.stdin.isTTY === true) return false;

  try {
    const { fstatSync } = require("fs");
    const stats = fstatSync(0);
    return stats.isFIFO() || stats.isSocket();
  } catch {
    return false;
  }
}
