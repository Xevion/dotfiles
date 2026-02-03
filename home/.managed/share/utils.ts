// Utility functions and constants

import { platform } from "os";
import { extname } from "path";
import { $ } from "bun";
import { fileTypeFromBuffer } from "file-type";
import type { FormatOption, ResolutionOption, EncoderOption } from "./types";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const DOMAIN = "https://i.xevion.dev";

// Fetch R2 credentials from Doppler once and cache them
let cachedEnv: {
  endpoint: string;
  accessKeyId: string;
  secretAccessKey: string;
  bucket: string;
} | null = null;

async function fetchDopplerSecrets(): Promise<typeof cachedEnv> {
  try {
    const result = await $`doppler secrets download --format json --no-file`.json();
    return {
      endpoint: result.R2_ENDPOINT,
      accessKeyId: result.R2_ACCESS_KEY_ID,
      secretAccessKey: result.R2_SECRET_ACCESS_KEY,
      bucket: result.R2_BUCKET,
    };
  } catch (error) {
    throw new Error(`Failed to fetch Doppler secrets: ${error}`);
  }
}

export const ENV = await (async () => {
  if (!cachedEnv) {
    cachedEnv = await fetchDopplerSecrets();
  }
  return cachedEnv;
})();

export const TIMING = {
  requestTimeout: 30_000,
  uploadTimeout: 60_000,
};

export const SIZE_THRESHOLDS = {
  warnImage: 10 * 1024 * 1024,
  warnVideo: 50 * 1024 * 1024,
  warnOther: 100 * 1024 * 1024,
};

export const INCOMPATIBLE_VIDEO_CODECS = ["hevc", "h265", "av1"];

export const MIME_EXTENSIONS: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
  "image/bmp": "bmp",
  "image/tiff": "tiff",
  "image/heic": "heic",
  "image/heif": "heif",
  "image/avif": "avif",
  "video/mp4": "mp4",
  "video/webm": "webm",
  "video/quicktime": "mov",
  "video/x-matroska": "mkv",
  "text/plain": "txt",
  "application/json": "json",
  "application/pdf": "pdf",
};

export const EXTENSION_MIMES: Record<string, string> = {
  png: "image/png",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  gif: "image/gif",
  webp: "image/webp",
  bmp: "image/bmp",
  tiff: "image/tiff",
  heic: "image/heic",
  heif: "image/heif",
  avif: "image/avif",
  mp4: "video/mp4",
  webm: "video/webm",
  mov: "video/quicktime",
  mkv: "video/x-matroska",
  txt: "text/plain",
  json: "application/json",
  pdf: "application/pdf",
  js: "application/javascript",
  ts: "application/typescript",
  rs: "text/x-rust",
  py: "text/x-python",
  md: "text/markdown",
  html: "text/html",
  css: "text/css",
};

export const PROBE_DURATION = 8;
export const PROBE_CRF_VALUES = [16, 18, 20, 22, 24, 26, 28, 30];
export const DEFAULT_CRF = 23;

export const QUALITY_IMG: Record<"high" | "balanced" | "small", number> = {
  high: 95,
  balanced: 85,
  small: 70,
};

// ---------------------------------------------------------------------------
// Platform detection
// ---------------------------------------------------------------------------

export const IS_WSL = await (async () => {
  try {
    const text = await Bun.file("/proc/version").text();
    return text.toLowerCase().includes("microsoft");
  } catch {
    return false;
  }
})();

export const IS_WINDOWS = platform() === "win32";

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export function formatTime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}m ${secs}s`;
}

export function isImageMime(mime: string): boolean {
  return mime.startsWith("image/");
}

export function isVideoMime(mime: string): boolean {
  return mime.startsWith("video/");
}

export function isTextMime(mime: string): boolean {
  return (
    mime.startsWith("text/") ||
    mime === "application/json" ||
    mime === "application/javascript" ||
    mime === "application/typescript"
  );
}

export function getExtensionForMime(mime: string): string {
  return MIME_EXTENSIONS[mime] || "bin";
}

export function getMimeForFormat(format: FormatOption): string | null {
  switch (format) {
    case "png": return "image/png";
    case "jpeg": return "image/jpeg";
    case "webp": return "image/webp";
    case "mp4": return "video/mp4";
    case "webm": return "video/webm";
    case "gif": return "image/gif";
    default: return null;
  }
}

export function encodeUrlPath(path: string): string {
  return path.split("/").map(segment => encodeURIComponent(segment)).join("/");
}

export async function hasCommand(cmd: string): Promise<boolean> {
  try {
    await $`which ${cmd}`.quiet();
    return true;
  } catch {
    return false;
  }
}

export async function detectAvailableEncoders(): Promise<EncoderOption[]> {
  const available: EncoderOption[] = ["cpu"];

  if (!(await hasCommand("ffmpeg"))) return available;

  try {
    const result = await $`ffmpeg -hide_banner -encoders`.text();
    if (result.includes("h264_nvenc")) available.push("nvenc");
    if (result.includes("libsvtav1") || result.includes("libaom-av1")) {
      available.push("av1");
    }
  } catch {}

  return available;
}

export function normalizeFilename(filename: string): string {
  return filename
    .toLowerCase()
    .replace(/\s+/g, "-")
    .replace(/[^a-z0-9\-_.]/g, "")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

export async function detectMimeType(buffer: Buffer, filename?: string): Promise<string> {
  const result = await fileTypeFromBuffer(buffer);
  if (result) return result.mime;

  if (filename) {
    const ext = extname(filename).slice(1).toLowerCase();
    if (EXTENSION_MIMES[ext]) return EXTENSION_MIMES[ext];
  }

  return "application/octet-stream";
}

export function getFormatOptions(mime: string): Array<{ value: FormatOption; label: string; hint?: string }> {
  const options: Array<{ value: FormatOption; label: string; hint?: string }> = [
    { value: "original", label: "Keep original", hint: getExtensionForMime(mime).toUpperCase() },
  ];

  if (isImageMime(mime)) {
    if (mime !== "image/png") options.push({ value: "png", label: "PNG", hint: "lossless" });
    if (mime !== "image/jpeg") options.push({ value: "jpeg", label: "JPEG", hint: "smaller, lossy" });
    if (mime !== "image/webp") options.push({ value: "webp", label: "WebP", hint: "modern, efficient" });
  } else if (isVideoMime(mime)) {
    if (mime !== "video/mp4") options.push({ value: "mp4", label: "MP4 (H.264)", hint: "universal" });
    if (mime !== "video/webm") options.push({ value: "webm", label: "WebM", hint: "web-optimized" });
    options.push({ value: "gif", label: "GIF", hint: "animated, large files" });
  }

  return options;
}

export function getResolutionOptions(mime: string): Array<{ value: ResolutionOption; label: string }> {
  const options: Array<{ value: ResolutionOption; label: string }> = [
    { value: "original", label: "Keep original" },
  ];

  if (isImageMime(mime) || isVideoMime(mime)) {
    options.push(
      { value: "1080p", label: "1080p" },
      { value: "720p", label: "720p" },
      { value: "480p", label: "480p" },
      { value: "50%", label: "50% scale" },
      { value: "25%", label: "25% scale" },
    );
  }

  return options;
}

export function getDefaultImageFormat(mime: string): FormatOption {
  switch (mime) {
    case "image/jpeg": return "jpeg";
    case "image/webp": return "webp";
    case "image/gif": return "original";
    default: return "png";
  }
}
