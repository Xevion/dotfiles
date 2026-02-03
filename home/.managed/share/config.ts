// Configuration building for interactive and non-interactive modes

import * as p from "@clack/prompts";
import {
  DEFAULT_CRF,
  isImageMime,
  isVideoMime,
  getFormatOptions,
  getResolutionOptions,
  detectAvailableEncoders,
  formatBytes,
  formatTime,
} from "./utils";
import { runAdaptiveProbe, generateSizeTargets, selectCrfForTarget } from "./media/quality";
import type {
  CliArgs,
  ShareConfig,
  SourceConfig,
  UploadSource,
  MediaIssue,
  FormatOption,
  ResolutionOption,
  FpsOption,
  EncoderOption,
  ImageQuality,
  QualityProbeResult,
  UI,
} from "./types";

function isCancel(value: unknown): value is symbol {
  return p.isCancel(value);
}

/**
 * Build configuration from CLI arguments only (non-interactive mode)
 */
export function buildConfigFromCli(
  cliArgs: CliArgs,
  source: SourceConfig,
  issues: MediaIssue[],
): ShareConfig {
  return {
    source,
    dryRun: cliArgs.dryRun,
    format: cliArgs.format ?? "original",
    resolution: cliArgs.resolution ?? "original",
    fps: cliArgs.fps ?? "original",
    removeAudio: cliArgs.removeAudio ?? false,
    encoder: cliArgs.encoder ?? "cpu",
    crf: cliArgs.crf ?? DEFAULT_CRF,
    gifFps: cliArgs.gifFps ?? 15,
    gifWidth: cliArgs.gifWidth ?? null,
    imageQuality: cliArgs.imageQuality ?? "balanced",
    randomFilename: cliArgs.randomFilename ?? false,
    normalizeFilename: cliArgs.normalizeFilename ?? false,
    autoFix: cliArgs.autoFix ?? (issues.length > 0),
  };
}

/**
 * Build configuration interactively with prompts
 * CLI args serve as defaults for prompts
 */
export async function buildConfigInteractively(
  cliArgs: CliArgs,
  source: SourceConfig,
  uploadSource: UploadSource,
  issues: MediaIssue[],
  ui: UI,
): Promise<ShareConfig> {
  const { mimeType } = uploadSource;
  const isImage = isImageMime(mimeType);
  const isVideo = isVideoMime(mimeType);

  // Start with CLI defaults
  let config: ShareConfig = buildConfigFromCli(cliArgs, source, issues);

  // Only prompt for media files
  if (!isImage && !isVideo) {
    return promptToggleOptions(config, cliArgs, issues, ui);
  }

  // Format selection
  const formatOptions = getFormatOptions(mimeType);
  if (formatOptions.length > 1 && !cliArgs.format) {
    const format = await p.select({
      message: "Format",
      options: formatOptions,
    });
    if (isCancel(format)) ui.cancel();
    config.format = format as FormatOption;
  }

  // GIF-specific options
  if (config.format === "gif") {
    config = await promptGifOptions(config, cliArgs, ui);
  }

  // Resolution selection (skip for GIF)
  if (config.format !== "gif" && !cliArgs.resolution) {
    const resOptions = getResolutionOptions(mimeType);
    if (resOptions.length > 1) {
      const resolution = await p.select({
        message: "Resolution",
        options: resOptions,
      });
      if (isCancel(resolution)) ui.cancel();
      config.resolution = resolution as ResolutionOption;
    }
  }

  // Video-specific options
  if (isVideo && config.format !== "gif") {
    config = await promptVideoOptions(config, cliArgs, uploadSource, ui);
  }

  // Image-specific options
  if (isImage && !cliArgs.imageQuality) {
    const quality = await p.select({
      message: "Quality",
      options: [
        { value: "high" as const, label: "High quality", hint: "larger file" },
        { value: "balanced" as const, label: "Balanced" },
        { value: "small" as const, label: "Smaller file", hint: "lower quality" },
      ],
    });
    if (isCancel(quality)) ui.cancel();
    config.imageQuality = quality as ImageQuality;
  }

  // Toggle options
  return promptToggleOptions(config, cliArgs, issues, ui);
}

async function promptGifOptions(
  config: ShareConfig,
  cliArgs: CliArgs,
  ui: UI,
): Promise<ShareConfig> {
  if (!cliArgs.gifFps) {
    const gifFps = await p.select({
      message: "GIF frame rate",
      options: [
        { value: 15, label: "15 fps", hint: "smaller file" },
        { value: 10, label: "10 fps", hint: "smallest" },
        { value: 24, label: "24 fps", hint: "smoother, larger" },
      ],
    });
    if (isCancel(gifFps)) ui.cancel();
    config.gifFps = gifFps as number;
  }

  if (cliArgs.gifWidth === undefined) {
    const gifWidth = await p.select({
      message: "GIF width",
      options: [
        { value: null, label: "Original" },
        { value: 640, label: "640px" },
        { value: 480, label: "480px" },
        { value: 320, label: "320px", hint: "smallest" },
      ],
    });
    if (isCancel(gifWidth)) ui.cancel();
    config.gifWidth = gifWidth as number | null;
  }

  return config;
}

async function promptVideoOptions(
  config: ShareConfig,
  cliArgs: CliArgs,
  uploadSource: UploadSource,
  ui: UI,
): Promise<ShareConfig> {
  // FPS selection
  if (!cliArgs.fps) {
    const fps = await p.select({
      message: "Frame rate",
      options: [
        { value: "original", label: "Keep original" },
        { value: "30", label: "30 fps", hint: "recommended for UI" },
        { value: "24", label: "24 fps", hint: "cinematic" },
        { value: "15", label: "15 fps", hint: "smallest size" },
      ],
    });
    if (isCancel(fps)) ui.cancel();
    config.fps = fps as FpsOption;
  }

  // Audio removal
  if (cliArgs.removeAudio === undefined) {
    const removeAudio = await p.confirm({
      message: "Remove audio track?",
      initialValue: true,
    });
    if (isCancel(removeAudio)) ui.cancel();
    config.removeAudio = removeAudio;
  }

  // Encoder selection
  if (!cliArgs.encoder) {
    const availableEncoders = await detectAvailableEncoders();
    const encoderOptions: Array<{ value: EncoderOption; label: string; hint?: string }> = [
      { value: "cpu", label: "CPU (H.264)", hint: "universal compatibility" },
    ];

    if (availableEncoders.includes("nvenc")) {
      encoderOptions.unshift({ value: "nvenc", label: "NVENC (GPU)", hint: "10-50x faster" });
    }

    if (availableEncoders.includes("av1")) {
      encoderOptions.push({ value: "av1", label: "AV1", hint: "30-50% smaller, slow encode" });
    }

    if (encoderOptions.length > 1) {
      const encoder = await p.select({
        message: "Encoder",
        options: encoderOptions,
      });
      if (isCancel(encoder)) ui.cancel();
      config.encoder = encoder as EncoderOption;
    }
  }

  // Adaptive quality probing
  const shouldProbe = cliArgs.probe || (!cliArgs.crf && !cliArgs.noProbe);

  if (shouldProbe) {
    config = await runQualityProbe(config, uploadSource.buffer, ui);
  }

  return config;
}

async function runQualityProbe(
  config: ShareConfig,
  buffer: Buffer,
  ui: UI,
): Promise<ShareConfig> {
  ui.spinnerStart("Analyzing quality options...");

  let probeResults: QualityProbeResult[] = [];

  try {
    probeResults = await runAdaptiveProbe(buffer, {
      format: config.format,
      resolution: config.resolution,
      fps: config.fps,
      removeAudio: config.removeAudio,
      encoder: config.encoder,
    }, (current, total) => {
      ui.spinnerUpdate(`Analyzing quality options... (${current}/${total})`);
    });
    ui.spinnerStop("Analysis complete");
  } catch (e) {
    ui.spinnerStop("Analysis failed, using defaults");
    ui.warn(e instanceof Error ? e.message : "Unknown error");
    return config;
  }

  if (probeResults.length === 0) {
    return config;
  }

  // Display results
  ui.showProbeResults(probeResults);

  // Generate size targets
  const sizeTargets = generateSizeTargets(probeResults);

  const targetOptions: Array<{ value: number | "manual"; label: string; hint?: string }> = sizeTargets.map(t => {
    const selectedCrf = selectCrfForTarget(t, probeResults);
    const selected = probeResults.find(r => r.crf === selectedCrf)!;
    return {
      value: t,
      label: `Under ${formatBytes(t)}`,
      hint: `CRF ${selectedCrf}`,
    };
  });
  targetOptions.push({ value: "manual", label: "Pick CRF manually" });

  const selection = await p.select({
    message: "Target size",
    options: targetOptions,
  });
  if (isCancel(selection)) {
    p.cancel("Cancelled.");
    process.exit(1);
  }

  if (selection === "manual") {
    const crfOptions = probeResults.map(r => ({
      value: r.crf,
      label: `CRF ${r.crf}`,
      hint: `~${formatBytes(r.estimatedFullSize)}`,
    }));

    const crf = await p.select({
      message: "Quality (CRF)",
      options: crfOptions,
    });
    if (isCancel(crf)) {
      p.cancel("Cancelled.");
      process.exit(1);
    }
    config.crf = crf as number;
  } else {
    config.crf = selectCrfForTarget(selection as number, probeResults);
  }

  return config;
}

async function promptToggleOptions(
  config: ShareConfig,
  cliArgs: CliArgs,
  issues: MediaIssue[],
  ui: UI,
): Promise<ShareConfig> {
  // Skip if all options already set via CLI
  if (
    cliArgs.autoFix !== undefined &&
    cliArgs.randomFilename !== undefined &&
    cliArgs.normalizeFilename !== undefined
  ) {
    return config;
  }

  const toggleOptions: Array<{ value: string; label: string; hint?: string }> = [
    { value: "autoFix", label: "Auto-fix issues", hint: issues.length > 0 ? `${issues.length} issue(s)` : "none" },
    { value: "randomFilename", label: "Random filename" },
    { value: "normalizeFilename", label: "Normalize filename", hint: "lowercase, no spaces" },
  ];

  const initialValues: string[] = [];
  if (config.autoFix) initialValues.push("autoFix");
  if (config.randomFilename) initialValues.push("randomFilename");
  if (config.normalizeFilename) initialValues.push("normalizeFilename");

  const toggles = await p.multiselect({
    message: "Options",
    options: toggleOptions,
    initialValues,
    required: false,
  });
  if (isCancel(toggles)) ui.cancel();

  config.autoFix = (toggles as string[]).includes("autoFix");
  config.randomFilename = (toggles as string[]).includes("randomFilename");
  config.normalizeFilename = (toggles as string[]).includes("normalizeFilename");

  return config;
}

/**
 * Run quality probe in non-interactive mode
 * Returns updated config with selected CRF
 */
export async function runNonInteractiveProbe(
  config: ShareConfig,
  buffer: Buffer,
): Promise<ShareConfig> {
  console.log("Analyzing quality options...");

  let probeResults: QualityProbeResult[] = [];

  try {
    probeResults = await runAdaptiveProbe(buffer, {
      format: config.format,
      resolution: config.resolution,
      fps: config.fps,
      removeAudio: config.removeAudio,
      encoder: config.encoder,
    }, (current, total) => {
      console.log(`  Probing ${current}/${total}...`);
    });
    console.log("Analysis complete");
  } catch (e) {
    console.log("Analysis failed, using defaults");
    console.error("Error:", e instanceof Error ? e.message : "Unknown error");
    return config;
  }

  if (probeResults.length === 0) {
    return config;
  }

  // Display results
  console.log("\nQuality probe results:");
  console.log("  CRF    Size");
  for (const r of probeResults) {
    const size = formatBytes(r.estimatedFullSize).padStart(10);
    console.log(`   ${String(r.crf).padStart(2)}   ${size}`);
  }

  // Non-interactive: pick middle CRF (balanced quality)
  const middleIndex = Math.floor(probeResults.length / 2);
  config.crf = probeResults[middleIndex].crf;
  console.log(`Selected CRF ${config.crf} (balanced quality)`);

  return config;
}
