// Video encoding and transformation functions

import { tmpdir } from "os";
import { join } from "path";
import { nanoid } from "nanoid";
import { writeTempFile, cleanupTempFiles } from "../io";
import { runMediaCommand, runFfmpegWithProgress, getScaleFilter } from "./ffmpeg";
import { probeVideo } from "./probe";
import { DEFAULT_CRF, INCOMPATIBLE_VIDEO_CODECS } from "../utils";
import type { VideoEncodeSettings, GifSettings, ShareConfig } from "../types";

/**
 * Remux video to fix container issues (missing duration, etc.)
 */
export async function remux(buffer: Buffer): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-remux-${nanoid(8)}.webm`);

  try {
    await runMediaCommand([
      "ffmpeg", "-y", "-i", inputPath,
      "-c", "copy", "-fflags", "+genpts",
      outputPath,
    ]);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Add faststart flag to MP4 for streaming optimization
 */
export async function addFaststart(buffer: Buffer): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-faststart-${nanoid(8)}.mp4`);

  try {
    await runMediaCommand([
      "ffmpeg", "-y", "-i", inputPath,
      "-c", "copy", "-movflags", "+faststart",
      outputPath,
    ]);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Re-encode video with specific encoder and settings
 */
export async function encodeWithEncoder(
  buffer: Buffer,
  settings: VideoEncodeSettings,
  encoder: "cpu" | "nvenc" | "av1",
  durationSeconds?: number,
  onProgress?: (percent: number) => void,
): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);

  // Determine output format and extension
  const outputFormat = settings.format === "webm" ? "webm" : "mp4";
  const outputPath = join(tmpdir(), `share-reencode-${nanoid(8)}.${outputFormat}`);

  try {
    const args = ["ffmpeg", "-y", "-i", inputPath];

    // Encoder and quality
    const useNvenc = encoder === "nvenc";
    const useAv1 = encoder === "av1";
    const isWebm = outputFormat === "webm";

    if (isWebm) {
      // WebM uses VP9
      args.push("-c:v", "libvpx-vp9", "-crf", String(settings.crf), "-b:v", "0");
    } else if (useAv1) {
      args.push("-c:v", "libsvtav1", "-crf", String(settings.crf), "-preset", "6");
    } else if (useNvenc) {
      args.push("-c:v", "h264_nvenc", "-qp", String(settings.crf + 1), "-preset", "p4");
    } else {
      args.push("-c:v", "libx264", "-crf", String(settings.crf), "-preset", "medium");
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
      if (isWebm) {
        args.push("-c:a", "libopus", "-b:a", "128k");
      } else {
        args.push("-c:a", "aac", "-b:a", "128k");
      }
    }

    // Container-specific flags
    if (!isWebm) {
      args.push("-movflags", "+faststart");
    }

    args.push(outputPath);

    if (onProgress && durationSeconds && durationSeconds > 0) {
      await runFfmpegWithProgress(args, durationSeconds, onProgress);
    } else {
      await runMediaCommand(args);
    }

    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Re-encode video with automatic encoder fallback
 */
export async function encode(
  buffer: Buffer,
  settings: VideoEncodeSettings,
  durationSeconds?: number,
  onProgress?: (percent: number) => void,
): Promise<Buffer> {
  if (settings.encoder === "nvenc") {
    try {
      return await encodeWithEncoder(buffer, settings, "nvenc", durationSeconds, onProgress);
    } catch {
      // Fall back to CPU
      return await encodeWithEncoder(buffer, settings, "cpu", durationSeconds, onProgress);
    }
  }

  if (settings.encoder === "av1") {
    try {
      return await encodeWithEncoder(buffer, settings, "av1", durationSeconds, onProgress);
    } catch {
      // Fall back to CPU
      return await encodeWithEncoder(buffer, settings, "cpu", durationSeconds, onProgress);
    }
  }

  return await encodeWithEncoder(buffer, settings, "cpu", durationSeconds, onProgress);
}

/**
 * Convert video to GIF with palette optimization
 */
export async function convertToGif(buffer: Buffer, settings: GifSettings): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const palettePath = join(tmpdir(), `share-palette-${nanoid(8)}.png`);
  const outputPath = join(tmpdir(), `share-gif-${nanoid(8)}.gif`);

  try {
    const scaleFilter = settings.width
      ? `scale=${settings.width}:-1:flags=lanczos`
      : "scale=iw:ih:flags=lanczos";

    // Pass 1: Generate palette
    await runMediaCommand([
      "ffmpeg", "-y", "-i", inputPath,
      "-vf", `fps=${settings.fps},${scaleFilter},palettegen`,
      palettePath,
    ]);

    // Pass 2: Apply palette
    await runMediaCommand([
      "ffmpeg", "-y", "-i", inputPath, "-i", palettePath,
      "-lavfi", `fps=${settings.fps},${scaleFilter}[x];[x][1:v]paletteuse`,
      outputPath,
    ]);

    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, palettePath, outputPath);
  }
}

/**
 * Process video according to configuration
 * Returns the processed buffer and MIME type
 */
export async function processVideo(
  buffer: Buffer,
  config: ShareConfig,
  onProgress?: (percent: number) => void,
): Promise<{ buffer: Buffer; mimeType: string }> {
  // Handle GIF output separately
  if (config.format === "gif") {
    const result = await convertToGif(buffer, {
      fps: config.gifFps,
      width: config.gifWidth,
    });
    return { buffer: result, mimeType: "image/gif" };
  }

  const probe = await probeVideo(buffer);
  const settings: VideoEncodeSettings = {
    format: config.format,
    resolution: config.resolution,
    fps: config.fps,
    removeAudio: config.removeAudio,
    encoder: config.encoder,
    crf: config.crf,
  };

  const needsReencode =
    config.format !== "original" ||
    config.resolution !== "original" ||
    config.fps !== "original" ||
    config.removeAudio ||
    config.crf !== DEFAULT_CRF;

  // Check if we can skip encoding
  if (!needsReencode && probe?.hasFaststart && probe.duration !== null) {
    if (!probe.codec || !INCOMPATIBLE_VIDEO_CODECS.includes(probe.codec)) {
      return { buffer, mimeType: "video/mp4" };
    }
  }

  const duration = probe?.duration ?? undefined;
  const result = await encode(buffer, settings, duration, onProgress);

  // Return correct MIME type based on format
  const mimeType = config.format === "webm" ? "video/webm" : "video/mp4";
  return { buffer: result, mimeType };
}
