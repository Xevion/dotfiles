#!/usr/bin/env -S bun --install=fallback

/**
 * share - Interactive file uploader to R2 with TUI
 *
 * Usage:
 *   share                    # Upload clipboard content
 *   share file.png           # Upload specific file
 *   cat file.txt | share     # Upload from stdin
 */

import pc from "picocolors";
import { parseCliArgs, printHelp, isInteractive } from "./cli";
import { buildConfigFromCli, buildConfigInteractively, runNonInteractiveProbe } from "./config";
import { createInteractiveUI, createSilentUI } from "./ui";
import { readSource, detectIssues, runPipeline, generateFilename } from "./pipeline";
import { wasStdinUsed } from "./io";
import { ENV, SIZE_THRESHOLDS, isImageMime, isVideoMime, formatBytes, detectMimeType, getMimeForFormat } from "./utils";
import { setVerbose } from "./media/ffmpeg";

async function main() {
  const { cliArgs, source, showHelp } = parseCliArgs(Bun.argv.slice(2));

  if (showHelp) {
    printHelp();
    process.exit(0);
  }

  // Enable verbose logging if requested
  if (cliArgs.verbose) {
    setVerbose(true);
  }

  const interactive = isInteractive();
  const ui = interactive ? createInteractiveUI() : createSilentUI();

  // Credentials check
  if (!ENV.endpoint || !ENV.accessKeyId || !ENV.secretAccessKey || !ENV.bucket) {
    ui.error("Missing R2 credentials - chezmoi template may not have been processed correctly");
    process.exit(1);
  }

  ui.intro();

  // Step 1: Read source
  ui.spinnerStart("Reading source...");
  let uploadSource;
  try {
    uploadSource = await readSource(source);
    if (!uploadSource.mimeType) {
      uploadSource.mimeType = await detectMimeType(uploadSource.buffer, uploadSource.filename);
    }
    ui.spinnerStop(`Source: ${source.type}`);
  } catch (e) {
    ui.spinnerStop("Failed to read source");
    ui.error(e instanceof Error ? e.message : "Unknown error");
    process.exit(1);
  }

  // Show file info
  ui.showSourceInfo(uploadSource.mimeType, uploadSource.buffer.length);

  // Size warning
  const threshold = isImageMime(uploadSource.mimeType) ? SIZE_THRESHOLDS.warnImage
    : isVideoMime(uploadSource.mimeType) ? SIZE_THRESHOLDS.warnVideo
    : SIZE_THRESHOLDS.warnOther;

  if (uploadSource.buffer.length > threshold) {
    ui.warn(`File is large (${formatBytes(uploadSource.buffer.length)}). Consider reducing quality/resolution.`);
  }

  // Step 2: Detect issues
  const issues = await detectIssues(uploadSource.buffer, uploadSource.mimeType);
  if (issues.length > 0) {
    ui.showIssues(issues);
  }

  // Step 3: Build configuration
  let config;
  if (interactive) {
    config = await buildConfigInteractively(cliArgs, source, uploadSource, issues, ui);
  } else {
    config = buildConfigFromCli(cliArgs, source, issues);

    // Handle non-interactive probe
    const shouldProbe = cliArgs.probe && !cliArgs.crf && isVideoMime(uploadSource.mimeType);
    if (shouldProbe) {
      config = await runNonInteractiveProbe(config, uploadSource.buffer);
    }
  }

  // Step 4: Handle dry-run before pipeline
  if (config.dryRun) {
    // Predict the output MIME type based on format setting
    const outputMimeType = config.format === "original"
      ? uploadSource.mimeType
      : (getMimeForFormat(config.format) ?? uploadSource.mimeType);
    const filename = generateFilename(uploadSource.filename, outputMimeType, config);
    ui.showDryRunSummary(config, filename, uploadSource.buffer.length, outputMimeType);
    process.exit(0);
  }

  // Step 5: Run pipeline
  try {
    const result = await runPipeline(config, uploadSource, issues, ui.events);
    ui.showResult(result);
  } catch (e) {
    ui.error(e instanceof Error ? e.message : "Unknown error");
    process.exit(1);
  }

  // Cleanup
  if (wasStdinUsed()) {
    process.stdin.destroy();
  }
  process.exit(0);
}

main().catch((e) => {
  console.error(pc.red(e instanceof Error ? e.message : "Unknown error"));
  process.exit(1);
});
