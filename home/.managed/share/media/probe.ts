// Media probing functions for video and image metadata extraction

import { $ } from "bun";
import { writeTempFile, cleanupTempFiles } from "../io";
import { hasCommand } from "../utils";
import type { VideoProbe, ImageProbe } from "../types";

/**
 * Probe video metadata using ffprobe
 */
export async function probeVideo(buffer: Buffer): Promise<VideoProbe | null> {
  if (!(await hasCommand("ffprobe"))) return null;

  const tmpPath = await writeTempFile(buffer);
  try {
    const result = await $`ffprobe -v error -show_format -show_streams -of json ${tmpPath}`.json();
    const format = result.format || {};
    const videoStream = result.streams?.find((s: { codec_type: string }) => s.codec_type === "video");

    const hasFaststart =
      format.tags?.major_brand === "isom" ||
      (format.format_name?.includes("mov") && format.tags?.compatible_brands?.includes("isom"));

    return {
      duration: format.duration ? parseFloat(format.duration) : null,
      codec: videoStream?.codec_name?.toLowerCase() || null,
      hasFaststart: !!hasFaststart,
      width: videoStream?.width || null,
      height: videoStream?.height || null,
    };
  } catch {
    return null;
  } finally {
    await cleanupTempFiles(tmpPath);
  }
}

/**
 * Probe image metadata using ImageMagick identify
 */
export async function probeImage(buffer: Buffer): Promise<ImageProbe | null> {
  if (!(await hasCommand("identify"))) return null;

  const tmpPath = await writeTempFile(buffer);
  try {
    const result = await $`identify -format "%w %h" ${tmpPath}`.text();
    const [width, height] = result.trim().split(" ").map(Number);
    return { width: width || null, height: height || null };
  } catch {
    return null;
  } finally {
    await cleanupTempFiles(tmpPath);
  }
}
