// Issue detection for media files

import { probeVideo } from "../media/probe";
import { remux, addFaststart, encode } from "../media/video";
import { convertToJpeg, convertToWebp, convertToPng } from "../media/image";
import { isImageMime, isVideoMime, INCOMPATIBLE_VIDEO_CODECS, DEFAULT_CRF } from "../utils";
import type { MediaIssue, VideoEncodeSettings } from "../types";

/**
 * Detect issues with video files
 */
export async function detectVideoIssues(buffer: Buffer): Promise<MediaIssue[]> {
  const probe = await probeVideo(buffer);
  if (!probe) return [];

  const issues: MediaIssue[] = [];

  if (probe.duration === null) {
    issues.push({
      id: "missing-duration",
      description: "Missing duration metadata",
      severity: "error",
      fix: remux,
    });
  } else if (!probe.hasFaststart) {
    issues.push({
      id: "missing-faststart",
      description: "Not optimized for streaming",
      severity: "warning",
      fix: addFaststart,
    });
  }

  if (probe.codec && INCOMPATIBLE_VIDEO_CODECS.includes(probe.codec)) {
    issues.push({
      id: "incompatible-codec",
      description: `Codec '${probe.codec}' has limited browser support`,
      severity: "warning",
      fix: async (buf) => {
        const settings: VideoEncodeSettings = {
          format: "mp4",
          resolution: "original",
          fps: "original",
          removeAudio: false,
          encoder: "cpu",
          crf: DEFAULT_CRF,
        };
        return encode(buf, settings);
      },
    });
  }

  return issues;
}

/**
 * Detect issues with image files
 */
export async function detectImageIssues(mime: string): Promise<MediaIssue[]> {
  const issues: MediaIssue[] = [];

  if (mime === "image/heic" || mime === "image/heif") {
    issues.push({
      id: "heic-compat",
      description: "HEIC not supported in browsers",
      severity: "error",
      fix: convertToJpeg,
    });
  }

  if (mime === "image/avif") {
    issues.push({
      id: "avif-compat",
      description: "AVIF has limited browser support",
      severity: "warning",
      fix: convertToWebp,
    });
  }

  if (mime === "image/bmp") {
    issues.push({
      id: "bmp-compat",
      description: "BMP is inefficient for web",
      severity: "warning",
      fix: convertToPng,
    });
  }

  return issues;
}

/**
 * Detect all issues for a media file based on MIME type
 */
export async function detectIssues(buffer: Buffer, mimeType: string): Promise<MediaIssue[]> {
  if (isVideoMime(mimeType)) {
    return detectVideoIssues(buffer);
  }

  if (isImageMime(mimeType)) {
    return detectImageIssues(mimeType);
  }

  return [];
}
