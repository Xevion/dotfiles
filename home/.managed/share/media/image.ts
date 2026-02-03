// Image conversion and transformation functions

import { tmpdir } from "os";
import { join } from "path";
import { nanoid } from "nanoid";
import { writeTempFile, cleanupTempFiles } from "../io";
import { runMediaCommand } from "./ffmpeg";
import { probeImage } from "./probe";
import { QUALITY_IMG, getMimeForFormat, getDefaultImageFormat } from "../utils";
import type { FormatOption, ImageQuality, ShareConfig } from "../types";

/**
 * Convert image to JPEG format
 */
export async function convertToJpeg(buffer: Buffer, quality = 90): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-convert-${nanoid(8)}.jpg`);

  try {
    await runMediaCommand(["convert", inputPath, "-quality", String(quality), outputPath]);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Convert image to WebP format
 */
export async function convertToWebp(buffer: Buffer, quality = 85): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-convert-${nanoid(8)}.webp`);

  try {
    await runMediaCommand(["convert", inputPath, "-quality", String(quality), outputPath]);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Convert image to PNG format
 */
export async function convertToPng(buffer: Buffer): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-convert-${nanoid(8)}.png`);

  try {
    await runMediaCommand(["convert", inputPath, outputPath]);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Convert image with format, quality, and optional scaling
 */
export async function convert(
  buffer: Buffer,
  format: FormatOption,
  quality: ImageQuality,
  scale?: string,
): Promise<Buffer> {
  const inputPath = await writeTempFile(buffer);
  const ext = format === "jpeg" ? "jpg" : format;
  const outputPath = join(tmpdir(), `share-convert-${nanoid(8)}.${ext}`);

  try {
    const args = ["convert", inputPath];

    if (scale) {
      args.push("-resize", scale);
    }

    args.push("-quality", String(QUALITY_IMG[quality]), outputPath);
    await runMediaCommand(args);
    return Buffer.from(await Bun.file(outputPath).arrayBuffer());
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

/**
 * Process image according to configuration
 * Returns the processed buffer and MIME type
 */
export async function processImage(
  buffer: Buffer,
  originalMime: string,
  config: ShareConfig,
): Promise<{ buffer: Buffer; mimeType: string }> {
  const probe = await probeImage(buffer);
  const needsConvert = config.format !== "original" || config.resolution !== "original";

  // Skip processing if no changes needed
  if (!needsConvert && config.imageQuality === "balanced") {
    return { buffer, mimeType: originalMime };
  }

  // Calculate scale based on resolution
  let scale: string | undefined;
  if (config.resolution !== "original" && probe?.height) {
    switch (config.resolution) {
      case "1080p": scale = probe.height > 1080 ? `x1080` : undefined; break;
      case "720p": scale = probe.height > 720 ? `x720` : undefined; break;
      case "480p": scale = probe.height > 480 ? `x480` : undefined; break;
      case "50%": scale = "50%"; break;
      case "25%": scale = "25%"; break;
    }
  }

  // Determine target format
  const targetFormat = config.format === "original"
    ? getDefaultImageFormat(originalMime)
    : config.format;

  const result = await convert(buffer, targetFormat as FormatOption, config.imageQuality, scale);
  return {
    buffer: result,
    mimeType: getMimeForFormat(targetFormat as FormatOption) || originalMime,
  };
}
