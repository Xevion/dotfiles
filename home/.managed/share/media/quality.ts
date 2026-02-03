// Adaptive quality probing for video encoding

import { tmpdir } from "os";
import { join } from "path";
import { nanoid } from "nanoid";
import { writeTempFile, cleanupTempFiles } from "../io";
import { runMediaCommand, setVerbose } from "./ffmpeg";
import { probeVideo } from "./probe";
import { PROBE_DURATION, PROBE_CRF_VALUES } from "../utils";
import type { VideoEncodeSettings, QualityProbeResult, SampleResult } from "../types";

// Re-export setVerbose for external use
export { setVerbose };

/**
 * Extract a sample from the middle of a video for quality probing.
 * Re-encodes the sample to ensure codec compatibility (WebM codecs
 * cannot be copied into MP4 container).
 * 
 * Returns the path to the sample file - caller is responsible for cleanup.
 */
export async function extractSampleToPath(buffer: Buffer, duration: number): Promise<{ path: string; actualDuration: number; cleanup: () => Promise<void> }> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-sample-${nanoid(8)}.mp4`);

  const probe = await probeVideo(buffer);
  const totalDuration = probe?.duration ?? 30;
  const actualDuration = Math.min(duration, totalDuration);
  const startTime = Math.max(0, (totalDuration / 2) - (actualDuration / 2));

  // Re-encode sample to ensure codec compatibility
  // Scale filter ensures even dimensions (required by NVENC/libx264)
  await runMediaCommand([
    "ffmpeg", "-y", "-ss", String(startTime),
    "-i", inputPath, "-t", String(actualDuration),
    "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
    "-c:v", "libx264", "-crf", "23", "-preset", "ultrafast",
    "-an",  // No audio needed for probing
    "-movflags", "+faststart",
    outputPath,
  ]);

  // Clean up input, keep output for probing
  await cleanupTempFiles(inputPath);

  return {
    path: outputPath,
    actualDuration,
    cleanup: () => cleanupTempFiles(outputPath),
  };
}

/**
 * Legacy function for backwards compatibility
 */
export async function extractSample(buffer: Buffer, duration: number): Promise<SampleResult> {
  const { path, actualDuration, cleanup } = await extractSampleToPath(buffer, duration);
  try {
    return {
      buffer: Buffer.from(await Bun.file(path).arrayBuffer()),
      actualDuration,
    };
  } finally {
    await cleanup();
  }
}

/**
 * Encode a sample from a file path with specific settings.
 * Used for probing - always uses NVENC for speed.
 * Returns the output file size.
 */
async function encodeProbeFromPath(
  inputPath: string,
  crf: number,
  settings: Omit<VideoEncodeSettings, "crf" | "encoder">,
): Promise<{ size: number; encodeTime: number }> {
  const outputPath = join(tmpdir(), `share-probe-${nanoid(8)}.mp4`);
  const startTime = performance.now();

  try {
    const args = ["ffmpeg", "-y", "-i", inputPath];

    // Always use NVENC for probing - it's much faster
    // QP mode maps roughly to CRF (qp = crf + 1 for similar quality)
    args.push("-c:v", "h264_nvenc", "-qp", String(crf + 1), "-preset", "p1");

    // Resolution
    if (settings.resolution !== "original") {
      const scale = getScaleFilter(settings.resolution);
      if (scale) args.push("-vf", scale);
    }

    // No audio for probing
    args.push("-an");

    args.push("-movflags", "+faststart", outputPath);

    await runMediaCommand(args);
    
    const stat = await Bun.file(outputPath).stat();
    const encodeTime = (performance.now() - startTime) / 1000;
    
    return { size: stat?.size ?? 0, encodeTime };
  } finally {
    await cleanupTempFiles(outputPath);
  }
}

/**
 * Encode a sample with specific settings and measure the result
 */
export async function encodeWithSettings(
  buffer: Buffer,
  settings: VideoEncodeSettings,
): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-probe-${nanoid(8)}.mp4`);

  try {
    const args = ["ffmpeg", "-y", "-i", inputPath];

    // Encoder selection
    const useNvenc = settings.encoder === "nvenc";
    const useAv1 = settings.encoder === "av1";

    if (useAv1) {
      args.push("-c:v", "libsvtav1", "-crf", String(settings.crf), "-preset", "6");
    } else if (useNvenc) {
      args.push("-c:v", "h264_nvenc", "-qp", String(settings.crf + 1), "-preset", "p4");
    } else {
      // Use ultrafast preset for probing - we only care about relative size estimates
      args.push("-c:v", "libx264", "-crf", String(settings.crf), "-preset", "ultrafast");
    }

    // FPS
    if (settings.fps !== "original") {
      args.push("-r", settings.fps);
    }

    // Resolution (always ensure even dimensions for codec compatibility)
    if (settings.resolution !== "original") {
      const scale = getScaleFilter(settings.resolution);
      if (scale) args.push("-vf", scale);
    } else {
      // Ensure even dimensions even at original resolution
      args.push("-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2");
    }

    // Audio
    if (settings.removeAudio) {
      args.push("-an");
    } else {
      args.push("-c:a", "aac", "-b:a", "128k");
    }

    args.push("-movflags", "+faststart", outputPath);

    await runMediaCommand(args);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Probe a single quality option and return timing/size estimates
 * Uses file path for efficiency (avoids re-writing buffer to disk)
 */
async function probeQualityFromPath(
  samplePath: string,
  sampleDuration: number,
  fullDuration: number,
  crf: number,
  settings: Omit<VideoEncodeSettings, "crf" | "encoder">,
): Promise<QualityProbeResult> {
  const { size, encodeTime } = await encodeProbeFromPath(samplePath, crf, settings);
  const ratio = fullDuration / sampleDuration;

  return {
    crf,
    sampleSize: size,
    sampleDuration,
    encodeTime,
    estimatedFullSize: Math.round(size * ratio),
    estimatedFullTime: Math.round(encodeTime * ratio),
  };
}

/**
 * Probe a single quality option and return timing/size estimates
 * Legacy buffer-based version for backwards compatibility
 */
export async function probeQuality(
  sample: Buffer,
  sampleDuration: number,
  fullDuration: number,
  crf: number,
  settings: Omit<VideoEncodeSettings, "crf">,
): Promise<QualityProbeResult> {
  const startTime = Date.now();

  const encoded = await encodeWithSettings(sample, { ...settings, crf });

  const encodeTime = (Date.now() - startTime) / 1000;
  const ratio = fullDuration / sampleDuration;

  return {
    crf,
    sampleSize: encoded.length,
    sampleDuration,
    encodeTime,
    estimatedFullSize: Math.round(encoded.length * ratio),
    estimatedFullTime: Math.round(encodeTime * ratio),
  };
}

/**
 * Run adaptive quality probing across multiple CRF values.
 * Uses NVENC and runs probes in parallel for speed.
 */
export async function runAdaptiveProbe(
  buffer: Buffer,
  settings: Omit<VideoEncodeSettings, "crf">,
  onProgress?: (current: number, total: number) => void,
): Promise<QualityProbeResult[]> {
  const probe = await probeVideo(buffer);
  const fullDuration = probe?.duration ?? 30;

  const sampleDuration = Math.min(PROBE_DURATION, fullDuration / 3);
  
  // Extract sample once to a file path
  const { path: samplePath, actualDuration, cleanup } = await extractSampleToPath(buffer, sampleDuration);

  try {
    // Run all probes in parallel using NVENC
    const probePromises = PROBE_CRF_VALUES.map((crf, i) => 
      probeQualityFromPath(samplePath, actualDuration, fullDuration, crf, settings)
        .then(result => {
          onProgress?.(i + 1, PROBE_CRF_VALUES.length);
          return result;
        })
    );

    const results = await Promise.all(probePromises);
    
    // Sort by CRF to ensure consistent ordering
    return results.sort((a, b) => a.crf - b.crf);
  } finally {
    await cleanup();
  }
}

/**
 * Generate reasonable size targets based on probe results
 */
export function generateSizeTargets(results: QualityProbeResult[]): number[] {
  const sizes = results.map(r => r.estimatedFullSize);
  const minSize = Math.min(...sizes);
  const maxSize = Math.max(...sizes);

  const tiers = [1, 2, 3, 5, 8, 10, 15, 20, 30, 50, 75, 100];
  const targets: number[] = [];

  for (const mb of tiers) {
    const bytes = mb * 1024 * 1024;
    if (bytes >= minSize * 0.9 && bytes <= maxSize * 1.1) {
      targets.push(bytes);
    }
  }

  if (targets.length < 2) {
    return [minSize, maxSize];
  }

  return targets;
}

/**
 * Select the best CRF to achieve a target size
 */
export function selectCrfForTarget(targetSize: number, results: QualityProbeResult[]): number {
  const sorted = [...results].sort((a, b) => a.crf - b.crf);
  const fitsTarget = sorted.filter(r => r.estimatedFullSize <= targetSize);

  if (fitsTarget.length > 0) {
    return fitsTarget[0].crf;
  }

  return sorted[sorted.length - 1].crf;
}

// Helper to get scale filter string
function getScaleFilter(resolution: string): string | undefined {
  switch (resolution) {
    case "1080p": return "scale=-2:1080";
    case "720p": return "scale=-2:720";
    case "480p": return "scale=-2:480";
    case "50%": return "scale=trunc(iw/4)*2:trunc(ih/4)*2";
    case "25%": return "scale=trunc(iw/8)*2:trunc(ih/8)*2";
    default: return undefined;
  }
}
