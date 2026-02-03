// Media processing orchestration

import { processVideo } from "../media/video";
import { processImage } from "../media/image";
import { isImageMime, isVideoMime, DEFAULT_CRF } from "../utils";
import type { ShareConfig, MediaIssue } from "../types";

export interface ProcessResult {
  buffer: Buffer;
  mimeType: string;
}

/**
 * Apply auto-fixes for detected issues
 */
export async function applyFixes(
  buffer: Buffer,
  mimeType: string,
  issues: MediaIssue[],
): Promise<{ buffer: Buffer; mimeType: string }> {
  let currentBuffer = buffer;
  let currentMime = mimeType;

  for (const issue of issues) {
    currentBuffer = await issue.fix(currentBuffer);

    // Update MIME type based on fix
    switch (issue.id) {
      case "heic-compat":
        currentMime = "image/jpeg";
        break;
      case "avif-compat":
      case "bmp-compat":
        currentMime = "image/webp";
        break;
    }
  }

  return { buffer: currentBuffer, mimeType: currentMime };
}

/**
 * Check if processing is needed based on config
 */
export function needsProcessing(config: ShareConfig, mimeType: string): boolean {
  if (isVideoMime(mimeType)) {
    return (
      config.format !== "original" ||
      config.resolution !== "original" ||
      config.fps !== "original" ||
      config.removeAudio ||
      config.crf !== DEFAULT_CRF
    );
  }

  if (isImageMime(mimeType)) {
    return (
      config.format !== "original" ||
      config.resolution !== "original" ||
      config.imageQuality !== "balanced"
    );
  }

  return false;
}

/**
 * Process media according to configuration
 */
export async function processMedia(
  buffer: Buffer,
  mimeType: string,
  config: ShareConfig,
  onProgress?: (percent: number) => void,
): Promise<ProcessResult> {
  if (!needsProcessing(config, mimeType)) {
    return { buffer, mimeType };
  }

  if (isVideoMime(mimeType)) {
    return processVideo(buffer, config, onProgress);
  }

  if (isImageMime(mimeType)) {
    return processImage(buffer, mimeType, config);
  }

  // Non-media files pass through unchanged
  return { buffer, mimeType };
}
