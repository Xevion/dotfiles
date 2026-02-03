// UI abstraction for interactive and non-interactive modes

import * as p from "@clack/prompts";
import pc from "picocolors";
import { formatBytes, formatTime } from "./utils";
import type { UI, PipelineEvents, PipelineResult, ShareConfig, MediaIssue, QualityProbeResult } from "./types";

/**
 * Create an interactive UI using @clack/prompts
 */
export function createInteractiveUI(): UI {
  let spinner: ReturnType<typeof p.spinner> | null = null;

  const events: PipelineEvents = {
    onProcessingStart: () => {
      spinner = p.spinner();
      spinner.start("Processing...");
    },
    onProcessingProgress: (percent) => {
      spinner?.message(`Processing... ${percent}%`);
    },
    onProcessingComplete: (size, time) => {
      spinner?.stop(`Processed ${pc.dim(`(${formatBytes(size)}, ${time.toFixed(1)}s)`)}`);
      spinner = null;
    },
    onUploadStart: (filename) => {
      spinner = p.spinner();
      spinner.start(`Uploading ${pc.dim(filename)}...`);
    },
    onUploadProgress: (loaded, total) => {
      const percent = Math.floor((loaded / total) * 100);
      spinner?.message(`Uploading... ${percent}%`);
    },
    onUploadComplete: (time) => {
      spinner?.stop(`Uploaded ${pc.dim(`(${time.toFixed(1)}s)`)}`);
      spinner = null;
    },
  };

  return {
    intro() {
      p.intro(pc.bgCyan(pc.black(" share ")));
    },

    showSourceInfo(mimeType, size) {
      const ext = mimeType.split("/")[1]?.toUpperCase() || "FILE";
      p.log.info(`${pc.cyan(ext)} ${pc.dim(`(${formatBytes(size)})`)}`);
    },

    showIssues(issues) {
      for (const issue of issues) {
        const icon = issue.severity === "error" ? pc.red("!") : pc.yellow("!");
        p.log.warn(`${icon} ${issue.description}`);
      }
    },

    showProbeResults(results) {
      p.log.info(pc.dim("-".repeat(24)));
      p.log.info(pc.dim("  CRF    Size"));
      p.log.info(pc.dim("-".repeat(24)));
      for (const r of results) {
        const size = formatBytes(r.estimatedFullSize).padStart(10);
        p.log.info(`   ${String(r.crf).padStart(2)}   ${size}`);
      }
      p.log.info(pc.dim("-".repeat(24)));
    },

    showDryRunSummary(config, filename, size, mimeType) {
      p.log.success(pc.yellow("Dry-run mode - skipping upload"));
      p.log.info(pc.dim("-".repeat(40)));
      console.log(pc.cyan("Configuration:"));
      console.log(`  Filename: ${filename}`);
      console.log(`  Size: ${formatBytes(size)}`);
      console.log(`  MIME: ${mimeType}`);
      console.log(`  Format: ${config.format}`);
      console.log(`  Resolution: ${config.resolution}`);
      if (config.fps !== "original") console.log(`  FPS: ${config.fps}`);
      if (config.removeAudio) console.log(`  Audio: removed`);
      if (config.encoder !== "cpu") console.log(`  Encoder: ${config.encoder}`);
      if (config.crf !== 23) console.log(`  CRF: ${config.crf}`);
      p.outro(pc.green("Done! (dry-run)"));
    },

    showResult(result) {
      p.log.success(pc.cyan(result.url));
      p.log.info(pc.dim("Copied to clipboard"));
      p.outro(pc.green("Done!"));
    },

    error(message) {
      p.log.error(message);
    },

    warn(message) {
      p.log.warn(message);
    },

    info(message) {
      p.log.info(message);
    },

    cancel(message) {
      p.cancel(message ?? "Cancelled.");
      process.exit(1);
    },

    spinnerStart(message) {
      spinner = p.spinner();
      spinner.start(message);
    },

    spinnerUpdate(message) {
      spinner?.message(message);
    },

    spinnerStop(message) {
      spinner?.stop(message);
      spinner = null;
    },

    events,
  };
}

/**
 * Create a silent UI for non-interactive/piped usage
 */
export function createSilentUI(): UI {
  const events: PipelineEvents = {
    onProcessingProgress: (percent) => {
      if (percent % 20 === 0) {
        console.log(`Processing... ${percent}%`);
      }
    },
    onProcessingComplete: (size, time) => {
      console.log(`Processed: ${formatBytes(size)} in ${time.toFixed(1)}s`);
    },
    onUploadStart: (filename) => {
      console.log(`Uploading ${filename}...`);
    },
    onUploadComplete: (time) => {
      console.log(`Uploaded in ${time.toFixed(1)}s`);
    },
  };

  return {
    intro() {
      console.log(pc.cyan("share") + pc.dim(" (non-interactive mode)"));
    },

    showSourceInfo(mimeType, size) {
      console.log(`Source: ${mimeType} (${formatBytes(size)})`);
    },

    showIssues(issues) {
      for (const issue of issues) {
        const prefix = issue.severity === "error" ? "ERROR" : "WARN";
        console.log(`[${prefix}] ${issue.description}`);
      }
    },

    showProbeResults(results) {
      console.log("\nQuality probe results:");
      console.log("  CRF    Size");
      for (const r of results) {
        const size = formatBytes(r.estimatedFullSize).padStart(10);
        console.log(`   ${String(r.crf).padStart(2)}   ${size}`);
      }
    },

    showDryRunSummary(config, filename, size, mimeType) {
      console.log("\n--- DRY RUN ---");
      console.log(`Filename: ${filename}`);
      console.log(`Size: ${formatBytes(size)}`);
      console.log(`MIME: ${mimeType}`);
      console.log(`Format: ${config.format}`);
      console.log(`Resolution: ${config.resolution}`);
    },

    showResult(result) {
      console.log(result.url);
    },

    error(message) {
      console.error(pc.red(`Error: ${message}`));
    },

    warn(message) {
      console.warn(pc.yellow(`Warning: ${message}`));
    },

    info(message) {
      console.log(message);
    },

    cancel(message) {
      console.error(message ?? "Cancelled.");
      process.exit(1);
    },

    spinnerStart(message) {
      console.log(message);
    },

    spinnerUpdate(message) {
      // Silent mode doesn't update spinners
    },

    spinnerStop(message) {
      console.log(message);
    },

    events,
  };
}
