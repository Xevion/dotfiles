#!/usr/bin/env -S bun --install=fallback

/**
 * share - Upload files to R2 and copy the URL to clipboard
 *
 * Usage:
 *   share                    # Upload clipboard content
 *   share file.png           # Upload specific file
 *   cat file.txt | share     # Upload from stdin
 *   share -c video.mov       # Convert then upload
 *
 * Environment Variables:
 *   R2_ENDPOINT              S3-compatible endpoint URL
 *   R2_ACCESS_KEY_ID         Access key ID
 *   R2_SECRET_ACCESS_KEY     Secret access key
 *   R2_BUCKET                Bucket name
 */

import { existsSync, fstatSync } from 'fs';
import { tmpdir, platform } from 'os';
import { join, basename, extname } from 'path';
import { parseArgs } from 'util';
import chalk from 'chalk';
import { S3Client } from '@aws-sdk/client-s3';
import { Upload } from '@aws-sdk/lib-storage';
import { nanoid } from 'nanoid';
import { fileTypeFromBuffer } from 'file-type';
import { $ } from 'bun';

interface UploadSource {
  buffer: Buffer;
  filename?: string;
  mimeType?: string;
}

interface UploadResult {
  url: string;
  key: string;
  size: number;
}

type FixMode = 'never' | 'prompt' | 'always';

interface MediaIssue {
  id: string;
  description: string;
  severity: 'error' | 'warning';
  autoFix: boolean; // true = instant/lossless, false = slow/lossy
  fix: (buffer: Buffer) => Promise<Buffer>;
}

interface ProbeResult {
  issues: MediaIssue[];
  metadata: Record<string, unknown>;
}

interface VideoProbe {
  duration: number | null;
  codec: string | null;
  hasFaststart: boolean;
}

type TimeoutHandle = ReturnType<typeof setTimeout>;
type IntervalHandle = ReturnType<typeof setInterval>;

const DOMAIN = 'https://i.xevion.dev';

const REQUIRED_ENV = ['R2_ENDPOINT', 'R2_ACCESS_KEY_ID', 'R2_SECRET_ACCESS_KEY', 'R2_BUCKET'];
const ENV = {
  endpoint: process.env.R2_ENDPOINT || '',
  accessKeyId: process.env.R2_ACCESS_KEY_ID || '',
  secretAccessKey: process.env.R2_SECRET_ACCESS_KEY || '',
  bucket: process.env.R2_BUCKET || '',
};

const TIMING = {
  spinnerFrame: 80,
  requestTimeout: 30_000,
  uploadTimeout: 60_000,
};

const SIZE_THRESHOLDS = {
  image: 10 * 1024 * 1024,
  video: 50 * 1024 * 1024,
  binary: 100 * 1024 * 1024,
  reencodeAuto: 10 * 1024 * 1024, // Auto re-encode incompatible codecs below this size
};

const INCOMPATIBLE_VIDEO_CODECS = ['hevc', 'h265', 'av1'];

const COLORS = {
  spinner: ['#A5D8DD', '#9DCCB4', '#B8D99A', '#E8D4A2', '#F4B8A4', '#F5A6A6'],
  success: '#9DCCB4',
  error: '#E89999',
  label: '#6B7280',
  dim: '#9CA3AF',
  progressFilled: '#A5D8DD',
  progressEmpty: '#374151',
};

const MIME_EXTENSIONS: Record<string, string> = {
  'image/png': 'png',
  'image/jpeg': 'jpg',
  'image/jpg': 'jpg',
  'image/gif': 'gif',
  'image/webp': 'webp',
  'image/bmp': 'bmp',
  'image/tiff': 'tiff',
  'image/heic': 'heic',
  'image/heif': 'heif',
  'image/avif': 'avif',
  'video/mp4': 'mp4',
  'video/webm': 'webm',
  'video/quicktime': 'mov',
  'video/x-matroska': 'mkv',
  'text/plain': 'txt',
  'application/json': 'json',
  'application/pdf': 'pdf',
};

const EXTENSION_MIMES: Record<string, string> = {
  'png': 'image/png',
  'jpg': 'image/jpeg',
  'jpeg': 'image/jpeg',
  'gif': 'image/gif',
  'webp': 'image/webp',
  'bmp': 'image/bmp',
  'tiff': 'image/tiff',
  'heic': 'image/heic',
  'heif': 'image/heif',
  'avif': 'image/avif',
  'mp4': 'video/mp4',
  'webm': 'video/webm',
  'mov': 'video/quicktime',
  'mkv': 'video/x-matroska',
  'txt': 'text/plain',
  'json': 'application/json',
  'pdf': 'application/pdf',
  'js': 'application/javascript',
  'ts': 'application/typescript',
  'rs': 'text/x-rust',
  'py': 'text/x-python',
  'md': 'text/markdown',
  'html': 'text/html',
  'css': 'text/css',
};

const TRUTHY = ['y', 'yes', 'true', 't', 'Y', 'YES', 'TRUE', 'T'];
const FALSY = ['n', 'no', 'false', 'f', 'N', 'NO', 'FALSE', 'F', 'deny'];

let VERBOSE = false;
let usedStdin = false;

const IS_WSL = await (async () => {
  try {
    const text = await Bun.file('/proc/version').text();
    return text.toLowerCase().includes('microsoft');
  } catch {
    return false;
  }
})();

const IS_WINDOWS = platform() === 'win32';

class Spinner {
  private frames = ['⣾', '⣽', '⣻', '⢿', '⡿', '⣟', '⣯', '⣷'];
  private colors = COLORS.spinner;
  private index = 0;
  private interval: IntervalHandle | null = null;

  start(message: string) {
    process.stdout.write('\x1B[?25l');

    this.interval = setInterval(() => {
      const colorIndex = this.index % this.colors.length;
      const frame = chalk.hex(this.colors[colorIndex])(this.frames[this.index]);
      process.stdout.write(`\r${frame} ${chalk.hex(COLORS.dim)(message)}`);
      this.index = (this.index + 1) % this.frames.length;
    }, TIMING.spinnerFrame);
  }

  stop(clearLine = true) {
    if (this.interval) {
      clearInterval(this.interval);
      if (clearLine) {
        process.stdout.write('\r\x1B[K');
      }
      process.stdout.write('\x1B[?25h');
    }
  }
}

function restoreCursor() {
  process.stdout.write('\x1B[?25h');
}

process.on('SIGINT', () => {
  restoreCursor();
  process.exit(130);
});

process.on('SIGTERM', () => {
  restoreCursor();
  process.exit(143);
});

process.on('uncaughtException', (err) => {
  restoreCursor();
  console.error(err);
  process.exit(1);
});

class ProgressBar {
  private total: number;
  private barWidth = 30;

  constructor(total: number) {
    this.total = Math.max(1, total);
  }

  update(loaded: number) {
    const percentage = Math.min(100, Math.floor((loaded / this.total) * 100));
    const filled = Math.floor((loaded / this.total) * this.barWidth);
    const empty = this.barWidth - filled;

    const bar =
      chalk.hex(COLORS.progressFilled)('█'.repeat(filled)) +
      chalk.hex(COLORS.progressEmpty)('░'.repeat(empty));

    const stats = `${formatBytes(loaded)}/${formatBytes(this.total)}`;

    process.stdout.write(`\r${bar} ${percentage}% · ${stats}`);
  }

  finish() {
    process.stdout.write('\r\x1B[K');
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function log(message: string, type: 'success' | 'error' | 'info' | 'dim' = 'info') {
  const icons = {
    success: chalk.hex(COLORS.success)('✓'),
    error: chalk.hex(COLORS.error)('✗'),
    info: chalk.hex(COLORS.dim)('→'),
    dim: chalk.hex(COLORS.dim)('·'),
  };
  console.log(`${icons[type]} ${message}`);
}

function debug(message: string, data?: unknown) {
  if (!VERBOSE) return;
  console.error(chalk.hex(COLORS.label)('[debug]'), message);
  if (data !== undefined) {
    console.error(chalk.hex(COLORS.dim)(JSON.stringify(data, null, 2)));
  }
}

async function detectMimeType(buffer: Buffer, filename?: string): Promise<string> {
  const result = await fileTypeFromBuffer(buffer);
  if (result) {
    debug('Detected MIME via file-type', { mime: result.mime });
    return result.mime;
  }

  if (filename) {
    const ext = extname(filename).slice(1).toLowerCase();
    if (EXTENSION_MIMES[ext]) {
      debug('Detected MIME via extension', { ext, mime: EXTENSION_MIMES[ext] });
      return EXTENSION_MIMES[ext];
    }
  }

  debug('Defaulting to application/octet-stream');
  return 'application/octet-stream';
}

function getExtensionForMime(mime: string): string {
  return MIME_EXTENSIONS[mime] || 'bin';
}

function getMimeForExtension(ext: string): string {
  const normalized = ext.replace(/^\.+/, '');
  return EXTENSION_MIMES[normalized] || 'application/octet-stream';
}

function isTextMime(mime: string): boolean {
  return mime.startsWith('text/') ||
    mime === 'application/json' ||
    mime === 'application/javascript' ||
    mime === 'application/typescript';
}

function isImageMime(mime: string): boolean {
  return mime.startsWith('image/');
}

function isVideoMime(mime: string): boolean {
  return mime.startsWith('video/');
}

async function readFromFile(path: string): Promise<UploadSource> {
  const spinner = new Spinner();
  spinner.start('Reading file...');

  try {
    if (!existsSync(path)) {
      throw new Error(`File not found: ${path}`);
    }

    const buffer = Buffer.from(await Bun.file(path).arrayBuffer());
    const filename = basename(path);
    const mimeType = await detectMimeType(buffer, filename);

    spinner.stop();
    debug('Read file', { path, size: buffer.length, mimeType });

    return { buffer, filename, mimeType };
  } catch (e) {
    spinner.stop();
    throw e;
  }
}

function hasStdinData(): boolean {
  if (Bun.stdin.isTTY === true) {
    return false;
  }

  try {
    const stats = fstatSync(0);
    return stats.isFIFO() || stats.isSocket();
  } catch (error) {
    debug('fstat stdin detection failed', error instanceof Error ? error.message : String(error));
    return false;
  }
}

async function readFromStdin(): Promise<UploadSource> {
  usedStdin = true;
  const spinner = new Spinner();
  spinner.start('Reading from stdin...');

  try {
    const chunks: Buffer[] = [];

    for await (const chunk of Bun.stdin.stream()) {
      chunks.push(Buffer.from(chunk));
    }

    const buffer = Buffer.concat(chunks);

    if (buffer.length === 0) {
      throw new Error('No data received from stdin');
    }

    const mimeType = await detectMimeType(buffer);
    spinner.stop();
    debug('Read stdin', { size: buffer.length, mimeType });

    return { buffer, mimeType };
  } catch (e) {
    spinner.stop();
    throw e;
  }
}

async function copyToClipboard(text: string): Promise<void> {
  if (IS_WSL || IS_WINDOWS) {
    const proc = Bun.spawn(['clip.exe'], { stdin: 'pipe' });
    proc.stdin.write(text);
    proc.stdin.end();
    await proc.exited;
  } else {
    const proc = Bun.spawn(['xclip', '-selection', 'clipboard'], { stdin: 'pipe' });
    proc.stdin.write(text);
    proc.stdin.end();
    await proc.exited;
  }
}

async function readFromClipboard(): Promise<UploadSource> {
  const spinner = new Spinner();
  spinner.start('Reading clipboard...');

  try {
    if (IS_WSL || IS_WINDOWS) {
      const result = await $`powershell.exe -NoProfile -Command "Get-Clipboard -Raw"`.text();
      const text = result.trim();

      if (!text) {
        throw new Error('Clipboard is empty');
      }

      if (text.startsWith('/') && existsSync(text)) {
        spinner.stop();
        const shouldUpload = await confirm(`Clipboard contains a file path: ${text}\nUpload this file?`);
        if (shouldUpload) {
          return readFromFile(text);
        }
        throw new Error('Upload cancelled');
      }

      spinner.stop();
      const buffer = Buffer.from(text, 'utf-8');
      debug('Read clipboard text (WSL)', { size: buffer.length });

      return { buffer, mimeType: 'text/plain' };
    }

    const targets = await $`xclip -selection clipboard -t TARGETS -o`.text();
    const targetList = targets.trim().split('\n');
    debug('Clipboard targets', targetList);

    if (targetList.includes('text/uri-list') || targetList.includes('x-special/gnome-copied-files')) {
      const uris = await $`xclip -selection clipboard -t text/uri-list -o`.text();
      const filePath = uris.split('\n')[0].replace(/^file:\/\//, '').trim();

      if (filePath && existsSync(filePath)) {
        spinner.stop();

        const shouldUpload = await confirm(`Clipboard contains a file path: ${filePath}\nUpload this file?`);
        if (shouldUpload) {
          return readFromFile(filePath);
        } else {
          throw new Error('Upload cancelled');
        }
      }
    }

    const imageTarget = targetList.find(t => t.startsWith('image/'));
    if (imageTarget) {
      const isBmp = imageTarget === 'image/bmp';
      const buffer = Buffer.from(await $`xclip -selection clipboard -t ${imageTarget} -o`.arrayBuffer());

      spinner.stop();
      debug('Read clipboard image', { target: imageTarget, size: buffer.length });

      if (isBmp) {
        log('Converting BMP to PNG...', 'info');
        return { buffer, mimeType: 'image/bmp', filename: 'paste.bmp' };
      }

      return { buffer, mimeType: imageTarget, filename: `paste.${getExtensionForMime(imageTarget)}` };
    }

    const text = await $`xclip -selection clipboard -o`.text();
    if (!text.trim()) {
      throw new Error('Clipboard is empty or contains unsupported data');
    }

    const trimmedText = text.trim();
    if (trimmedText.startsWith('/') && existsSync(trimmedText)) {
      spinner.stop();
      const shouldUpload = await confirm(`Clipboard contains a file path: ${trimmedText}\nUpload this file?`);
      if (shouldUpload) {
        return readFromFile(trimmedText);
      }
    }

    spinner.stop();
    const buffer = Buffer.from(text, 'utf-8');
    debug('Read clipboard text', { size: buffer.length });

    return { buffer, mimeType: 'text/plain' };
  } catch (e) {
    spinner.stop();
    throw e;
  }
}

async function confirm(message: string): Promise<boolean> {
  process.stdout.write(`${chalk.hex(COLORS.label)('?')} ${message} ${chalk.hex(COLORS.dim)('(y/n)')}: `);

  const input = await new Promise<string>((resolve) => {
    process.stdin.resume();
    process.stdin.once('data', (data) => {
      process.stdin.pause();
      resolve(data.toString().trim());
    });
  });

  return TRUTHY.includes(input);
}

async function promptExtension(): Promise<string | null> {
  while (true) {
    process.stdout.write(`${chalk.hex(COLORS.label)('?')} File extension ${chalk.hex(COLORS.dim)('(default: .txt)')}: `);

    const input = await new Promise<string>((resolve) => {
      process.stdin.resume();
      process.stdin.once('data', (data) => {
        process.stdin.pause();
        resolve(data.toString().trim());
      });
    });

    if (!input) {
      const useTxt = await confirm('Upload as .txt?');
      if (useTxt) return 'txt';
      return null;
    }

    if (!input.startsWith('.')) {
      if (TRUTHY.includes(input)) return 'txt';
      if (FALSY.includes(input)) return null;
    }

    let ext = input.startsWith('.') ? input.slice(1) : input;

    if (ext.startsWith('.') || ext.includes('..')) {
      log('Invalid extension format. Use format like "txt", ".txt", or ".ts.map"', 'error');
      continue;
    }

    return ext;
  }
}

async function cleanupTempFiles(...paths: string[]): Promise<void> {
  for (const path of paths) {
    try {
      await $`rm -f ${path}`.quiet();
    } catch {
      debug('Failed to cleanup temp file', path);
    }
  }
}

// ============================================================================
// Media Validation
// ============================================================================

async function hasCommand(cmd: string): Promise<boolean> {
  try {
    await $`which ${cmd}`.quiet();
    return true;
  } catch {
    return false;
  }
}

async function writeTempFile(buffer: Buffer, ext = ''): Promise<string> {
  const path = join(tmpdir(), `share-probe-${nanoid(8)}${ext}`);
  await Bun.write(path, buffer);
  return path;
}

function parseFixMode(value: string | undefined): FixMode {
  if (!value) return 'prompt';
  const v = value.toLowerCase();
  if (v.startsWith('n')) return 'never';
  if (v.startsWith('a')) return 'always';
  return 'prompt';
}

function isAudioMime(mime: string): boolean {
  return mime.startsWith('audio/');
}

async function probeVideoMetadata(buffer: Buffer): Promise<VideoProbe | null> {
  if (!await hasCommand('ffprobe')) {
    debug('ffprobe not found, skipping video validation');
    return null;
  }

  const tmpPath = await writeTempFile(buffer);
  try {
    const result = await $`ffprobe -v error -show_format -show_streams -of json ${tmpPath}`.json();
    const format = result.format || {};
    const videoStream = result.streams?.find((s: { codec_type: string }) => s.codec_type === 'video');

    // Check for faststart by looking at format tags or probing atom order
    // A proper check would require parsing the file, but we can infer from tags
    const hasFaststart = format.tags?.major_brand === 'isom' ||
      format.format_name?.includes('mov') && format.tags?.compatible_brands?.includes('isom');

    return {
      duration: format.duration ? parseFloat(format.duration) : null,
      codec: videoStream?.codec_name?.toLowerCase() || null,
      hasFaststart: !!hasFaststart,
    };
  } catch (e) {
    debug('ffprobe failed', e instanceof Error ? e.message : String(e));
    return null;
  } finally {
    await cleanupTempFiles(tmpPath);
  }
}

async function probeVideo(buffer: Buffer): Promise<ProbeResult | null> {
  const probe = await probeVideoMetadata(buffer);
  if (!probe) return null;

  const issues: MediaIssue[] = [];

  // Missing duration metadata - remux fixes this and also adds faststart
  if (probe.duration === null) {
    issues.push({
      id: 'missing-duration',
      description: 'Missing duration metadata',
      severity: 'error',
      autoFix: true,
      fix: remuxVideo,
    });
    // Skip faststart check since remux handles it
  } else if (!probe.hasFaststart) {
    // Only check faststart if duration exists (otherwise remux already handles it)
    issues.push({
      id: 'missing-faststart',
      description: 'MP4 not optimized for streaming',
      severity: 'warning',
      autoFix: true,
      fix: addFaststart,
    });
  }

  // Incompatible codec - this is separate since it requires re-encoding
  if (probe.codec && INCOMPATIBLE_VIDEO_CODECS.includes(probe.codec)) {
    const isSmall = buffer.length < SIZE_THRESHOLDS.reencodeAuto;
    issues.push({
      id: 'incompatible-codec',
      description: `Codec '${probe.codec}' has limited browser/Discord support`,
      severity: 'warning',
      autoFix: isSmall,
      fix: reencodeVideo,
    });
  }

  return { issues, metadata: probe };
}

async function probeImage(_buffer: Buffer, mime: string): Promise<ProbeResult | null> {
  const issues: MediaIssue[] = [];

  if (mime === 'image/heic' || mime === 'image/heif') {
    issues.push({
      id: 'heic-compat',
      description: 'HEIC not supported in browsers',
      severity: 'error',
      autoFix: true,
      fix: convertImageToJpeg,
    });
  }

  if (mime === 'image/avif') {
    issues.push({
      id: 'avif-compat',
      description: 'AVIF has limited browser support',
      severity: 'warning',
      autoFix: true,
      fix: convertImageToWebp,
    });
  }

  return issues.length > 0 ? { issues, metadata: { originalMime: mime } } : null;
}

async function probeAudio(buffer: Buffer): Promise<ProbeResult | null> {
  if (!await hasCommand('ffprobe')) {
    debug('ffprobe not found, skipping audio validation');
    return null;
  }

  const tmpPath = await writeTempFile(buffer);
  try {
    const result = await $`ffprobe -v error -show_format -of json ${tmpPath}`.json();
    const format = result.format || {};
    const issues: MediaIssue[] = [];

    if (!format.duration || format.duration === 'N/A') {
      issues.push({
        id: 'missing-duration',
        description: 'Missing duration metadata',
        severity: 'error',
        autoFix: true,
        fix: remuxAudio,
      });
    }

    return issues.length > 0 ? { issues, metadata: format } : null;
  } catch (e) {
    debug('ffprobe failed for audio', e instanceof Error ? e.message : String(e));
    return null;
  } finally {
    await cleanupTempFiles(tmpPath);
  }
}

async function validateMedia(buffer: Buffer, mimeType: string): Promise<ProbeResult | null> {
  if (isVideoMime(mimeType)) {
    return probeVideo(buffer);
  } else if (isImageMime(mimeType)) {
    return probeImage(buffer, mimeType);
  } else if (isAudioMime(mimeType)) {
    return probeAudio(buffer);
  }
  return null;
}

async function applyFixes(
  buffer: Buffer,
  mimeType: string,
  issues: MediaIssue[],
  mode: FixMode,
  skipPrompts: boolean
): Promise<{ buffer: Buffer; mimeType: string; applied: string[] }> {
  if (mode === 'never' || issues.length === 0) {
    for (const issue of issues) {
      log(`Skipped: ${issue.description}`, 'dim');
    }
    return { buffer, mimeType, applied: [] };
  }

  const applied: string[] = [];

  for (const issue of issues) {
    const shouldAuto = mode === 'always' || (mode === 'prompt' && issue.autoFix);

    if (shouldAuto) {
      log(`Fixing: ${issue.description}`, 'info');
      buffer = await issue.fix(buffer);
      applied.push(issue.id);
    } else if (mode === 'prompt') {
      if (skipPrompts) {
        log(`Skipped: ${issue.description}`, 'dim');
        continue;
      }

      const proceed = await confirm(`Fix: ${issue.description}?`);
      if (proceed) {
        buffer = await issue.fix(buffer);
        applied.push(issue.id);
      }
    }
  }

  // Update mimeType based on applied fixes
  if (applied.includes('heic-compat')) {
    mimeType = 'image/jpeg';
  } else if (applied.includes('avif-compat')) {
    mimeType = 'image/webp';
  } else if (applied.includes('incompatible-codec')) {
    mimeType = 'video/mp4';
  }

  return { buffer, mimeType, applied };
}

// ============================================================================
// Media Fix Functions
// ============================================================================

async function remuxVideo(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Remuxing video...');

  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-remux-${nanoid(8)}.webm`);

  try {
    // Remux without re-encoding, adding faststart for MP4
    await $`ffmpeg -y -i ${inputPath} -c copy -fflags +genpts ${outputPath}`.quiet();

    const result = Buffer.from(await Bun.file(outputPath).arrayBuffer());
    spinner.stop();
    log('Fixed video metadata', 'success');
    return result;
  } catch (e) {
    spinner.stop();
    throw new Error(`Video remux failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function addFaststart(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Optimizing for streaming...');

  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-faststart-${nanoid(8)}.mp4`);

  try {
    await $`ffmpeg -y -i ${inputPath} -c copy -movflags +faststart ${outputPath}`.quiet();

    const result = Buffer.from(await Bun.file(outputPath).arrayBuffer());
    spinner.stop();
    log('Optimized for streaming', 'success');
    return result;
  } catch (e) {
    spinner.stop();
    throw new Error(`Faststart optimization failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function reencodeVideo(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Re-encoding video for compatibility...');

  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-reencode-${nanoid(8)}.mp4`);

  try {
    await $`ffmpeg -y -i ${inputPath} -c:v libx264 -crf 23 -preset medium -c:a aac -b:a 128k -movflags +faststart ${outputPath}`.quiet();

    const result = Buffer.from(await Bun.file(outputPath).arrayBuffer());
    spinner.stop();
    log('Re-encoded to H.264', 'success');
    return result;
  } catch (e) {
    spinner.stop();
    throw new Error(`Video re-encode failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function remuxAudio(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Fixing audio metadata...');

  const inputPath = await writeTempFile(buffer);
  // Detect format and use same extension
  const outputPath = join(tmpdir(), `share-audio-${nanoid(8)}.mka`);

  try {
    await $`ffmpeg -y -i ${inputPath} -c copy ${outputPath}`.quiet();

    const result = Buffer.from(await Bun.file(outputPath).arrayBuffer());
    spinner.stop();
    log('Fixed audio metadata', 'success');
    return result;
  } catch (e) {
    spinner.stop();
    throw new Error(`Audio remux failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function convertImageToJpeg(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Converting to JPEG...');

  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-convert-${nanoid(8)}.jpg`);

  try {
    await $`convert ${inputPath} -quality 90 ${outputPath}`.quiet();

    const result = Buffer.from(await Bun.file(outputPath).arrayBuffer());
    spinner.stop();
    log('Converted to JPEG', 'success');
    return result;
  } catch (e) {
    spinner.stop();
    throw new Error(`JPEG conversion failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function convertImageToWebp(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Converting to WebP...');

  const inputPath = await writeTempFile(buffer);
  const outputPath = join(tmpdir(), `share-convert-${nanoid(8)}.webp`);

  try {
    await $`convert ${inputPath} -quality 85 ${outputPath}`.quiet();

    const result = Buffer.from(await Bun.file(outputPath).arrayBuffer());
    spinner.stop();
    log('Converted to WebP', 'success');
    return result;
  } catch (e) {
    spinner.stop();
    throw new Error(`WebP conversion failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

// ============================================================================
// Original Conversion Functions (kept for -c flag compatibility)
// ============================================================================

async function convertImage(buffer: Buffer, fromMime: string, shouldConvert: boolean): Promise<Buffer> {
  const needsConversion = fromMime === 'image/bmp' ||
    (shouldConvert && (fromMime === 'image/heic' || fromMime === 'image/heif' || fromMime === 'image/tiff'));

  if (!needsConversion) return buffer;

  const spinner = new Spinner();
  spinner.start('Converting image...');

  const inputPath = join(tmpdir(), `share-input-${nanoid(8)}`);
  const outputPath = join(tmpdir(), `share-output-${nanoid(8)}.png`);

  try {
    await Bun.write(inputPath, buffer);

    if (fromMime === 'image/heic' || fromMime === 'image/heif') {
      await $`convert ${inputPath} -quality 90 ${outputPath}`.quiet();
    } else {
      await $`convert ${inputPath} ${outputPath}`.quiet();
    }

    const converted = Buffer.from(await Bun.file(outputPath).arrayBuffer());

    spinner.stop();
    log(`Converted to ${fromMime === 'image/bmp' ? 'PNG' : 'JPEG'}`, 'success');

    return converted;
  } catch (e) {
    spinner.stop();
    throw new Error(`Image conversion failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function convertVideo(buffer: Buffer): Promise<Buffer> {
  const spinner = new Spinner();
  spinner.start('Converting video (this may take a while)...');

  const inputPath = join(tmpdir(), `share-input-${nanoid(8)}`);
  const outputPath = join(tmpdir(), `share-output-${nanoid(8)}.mp4`);

  try {
    await Bun.write(inputPath, buffer);

    await $`ffmpeg -i ${inputPath} -c:v libx264 -crf 28 -preset slow -vf "scale=-2:min(720,ih)" -c:a aac -b:a 128k -movflags +faststart ${outputPath}`.quiet();

    const converted = Buffer.from(await Bun.file(outputPath).arrayBuffer());

    spinner.stop();
    log('Converted to web-optimized MP4', 'success');

    return converted;
  } catch (e) {
    spinner.stop();
    throw new Error(`Video conversion failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    await cleanupTempFiles(inputPath, outputPath);
  }
}

async function uploadToR2(
  buffer: Buffer,
  filename: string,
  mimeType: string
): Promise<UploadResult> {
  if (!ENV.endpoint || !ENV.accessKeyId || !ENV.secretAccessKey || !ENV.bucket) {
    throw new Error('Missing R2 credentials - ensure all R2_* environment variables are set');
  }

  debug('S3 Client Config', {
    endpoint: ENV.endpoint,
    bucket: ENV.bucket,
    accessKeyIdLength: ENV.accessKeyId.length,
  });

  const client = new S3Client({
    region: 'auto',
    endpoint: ENV.endpoint,
    credentials: {
      accessKeyId: ENV.accessKeyId,
      secretAccessKey: ENV.secretAccessKey,
    },
    requestHandler: {
      requestTimeout: TIMING.requestTimeout,
    },
  });

  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const key = `${year}/${month}/${filename}`;

  debug('Uploading to R2', { key, size: buffer.length, mimeType });

  const progressBar = new ProgressBar(buffer.length);

  try {
    const upload = new Upload({
      client,
      params: {
        Bucket: ENV.bucket,
        Key: key,
        Body: buffer,
        ContentType: mimeType,
      },
    });

    upload.on('httpUploadProgress', (progress) => {
      if (progress.loaded) {
        progressBar.update(progress.loaded);
      }
    });

    console.log(`Uploading ${chalk.hex(COLORS.dim)(filename)}`);

    let timeoutId: TimeoutHandle;
    const uploadPromise = upload.done();
    const timeoutPromise = new Promise((_, reject) => {
      timeoutId = setTimeout(() => reject(new Error('Upload timeout after 60 seconds')), TIMING.uploadTimeout);
    });

    debug('Waiting for upload to complete...');
    try {
      await Promise.race([uploadPromise, timeoutPromise]);
      debug('Upload promise resolved');
    } finally {
      clearTimeout(timeoutId!);
    }
    progressBar.finish();
    debug('Progress bar finished');

    upload.removeAllListeners();
    debug('Event listeners removed');

    const url = `${DOMAIN}/${key}`;
    debug('Returning result');
    return { url, key, size: buffer.length };
  } catch (e) {
    progressBar.finish();
    if (VERBOSE && e instanceof Error) {
      debug('Upload error details', { error: e.message, stack: e.stack });
    }
    throw new Error(`Upload failed: ${e instanceof Error ? e.message : String(e)}`);
  } finally {
    client.destroy();
  }
}

function hasNanoidSuffix(filename: string): boolean {
  // nanoid default alphabet: A-Za-z0-9_-
  return /-[A-Za-z0-9_-]{8}\.[^.]+$/.test(filename);
}

async function main() {
  const { values, positionals } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      verbose: { type: 'boolean', short: 'v' },
      convert: { type: 'boolean', short: 'c' },
      fix: { type: 'string', short: 'F' },
      yes: { type: 'boolean', short: 'y' },
      name: { type: 'string', short: 'n' },
      help: { type: 'boolean', short: 'h' },
    },
    allowPositionals: true,
  });

  if (values.help) {
    console.log(`
${chalk.hex(COLORS.success)('share')} - Upload files to R2 and copy the URL to clipboard

${chalk.hex(COLORS.label)('Usage:')}
  share [options] [file]

${chalk.hex(COLORS.label)('Arguments:')}
  file                Optional file path to upload

${chalk.hex(COLORS.label)('Options:')}
  -v, --verbose       Enable debug output
  -c, --convert       Convert media before upload (re-encode video, etc.)
  -F, --fix <mode>    Fix media issues: n=never, p=prompt (default), a=always
  -y, --yes           Skip confirmation prompts
  -n, --name <name>   Custom filename (without extension)
  -h, --help          Show this help

${chalk.hex(COLORS.label)('Fix Modes:')}
  -Fn, --fix=never    Skip all fixes, upload as-is
  -Fp, --fix=prompt   Auto-fix quick/lossless, prompt for slow/lossy (default)
  -Fa, --fix=always   Apply all fixes automatically

${chalk.hex(COLORS.label)('Media Fixes (auto-applied by default):')}
  • Missing video duration/metadata (remux)
  • MP4 streaming optimization (faststart)
  • HEIC/AVIF browser compatibility (convert to JPEG/WebP)

${chalk.hex(COLORS.label)('Media Fixes (prompted or with -Fa):')}
  • Video re-encoding for codec compatibility (H.265/AV1 → H.264)
  • Large file optimizations

${chalk.hex(COLORS.label)('Examples:')}
  share                      # Upload clipboard content
  share screenshot.png       # Upload specific file
  cat file.txt | share       # Upload from stdin
  share -c large-video.mov   # Convert/re-encode then upload
  share -Fa video.webm       # Fix all issues automatically

${chalk.hex(COLORS.label)('Environment Variables:')}
  R2_ENDPOINT              S3-compatible endpoint URL
  R2_ACCESS_KEY_ID         Access key ID
  R2_SECRET_ACCESS_KEY     Secret access key
  R2_BUCKET                Bucket name
`);
    return;
  }

  VERBOSE = values.verbose || false;

  const missing = REQUIRED_ENV.filter(key => !process.env[key]);
  if (missing.length > 0) {
    log(`Missing environment variables: ${missing.join(', ')}`, 'error');
    process.exit(1);
  }

  try {
    let source: UploadSource;

    if (positionals[0]) {
      source = await readFromFile(positionals[0]);
    } else if (hasStdinData()) {
      source = await readFromStdin();
    } else {
      source = await readFromClipboard();
    }

    let { buffer, filename, mimeType } = source;

    if (!mimeType) {
      mimeType = await detectMimeType(buffer, filename);
    }

    log(`Detected: ${mimeType} (${formatBytes(buffer.length)})`, 'success');

    if (isTextMime(mimeType) && !values.yes) {
      const ext = await promptExtension();
      if (!ext) {
        log('Upload cancelled', 'error');
        process.exit(0);
      }

      mimeType = getMimeForExtension(ext);
      const baseName = values.name || (filename ? basename(filename, extname(filename)) : 'text');
      filename = `${baseName}-${nanoid(8)}.${ext}`;
    }

    // Determine fix mode: -c implies -Fa (always fix), otherwise parse -F flag
    const fixMode = values.convert ? 'always' : parseFixMode(values.fix);

    // Validate and fix media issues
    if (fixMode !== 'never') {
      const probeResult = await validateMedia(buffer, mimeType);

      if (probeResult && probeResult.issues.length > 0) {
        debug('Media issues detected', probeResult.metadata);

        const fixResult = await applyFixes(
          buffer,
          mimeType,
          probeResult.issues,
          fixMode,
          values.yes || false
        );

        buffer = fixResult.buffer;
        mimeType = fixResult.mimeType;

        if (fixResult.applied.length > 0) {
          debug('Applied fixes', fixResult.applied);
        }
      }
    }

    if (!values.yes) {
      let needsConfirm = false;
      let reason = '';

      if (isImageMime(mimeType) && buffer.length > SIZE_THRESHOLDS.image) {
        needsConfirm = true;
        reason = `image is larger than ${formatBytes(SIZE_THRESHOLDS.image)}`;
      } else if (isVideoMime(mimeType) && buffer.length > SIZE_THRESHOLDS.video) {
        needsConfirm = true;
        reason = `video is larger than ${formatBytes(SIZE_THRESHOLDS.video)}`;
      } else if (!isImageMime(mimeType) && !isVideoMime(mimeType) && buffer.length > SIZE_THRESHOLDS.binary) {
        needsConfirm = true;
        reason = `file is larger than ${formatBytes(SIZE_THRESHOLDS.binary)}`;
      }

      if (needsConfirm) {
        const shouldContinue = await confirm(`Upload large file? (${reason})`);
        if (!shouldContinue) {
          log('Upload cancelled', 'error');
          process.exit(0);
        }
      }
    }

    // Legacy conversion path: BMP always converts, -c triggers full re-encode
    if (isImageMime(mimeType) && mimeType === 'image/bmp') {
      buffer = await convertImage(buffer, mimeType, false);
      mimeType = 'image/png';
    } else if (isVideoMime(mimeType) && values.convert) {
      // Full video conversion (re-encode with scaling, etc.) only with -c
      buffer = await convertVideo(buffer);
      mimeType = 'video/mp4';
    }

    if (!filename) {
      const ext = getExtensionForMime(mimeType);
      const baseName = values.name || 'upload';
      filename = `${baseName}-${nanoid(8)}.${ext}`;
    } else if (!hasNanoidSuffix(filename)) {
      const ext = extname(filename);
      const base = basename(filename, ext);
      const finalName = values.name || base;
      filename = `${finalName}-${nanoid(8)}${ext}`;
    }

    debug('Calling uploadToR2...');
    const result = await uploadToR2(buffer, filename, mimeType);
    debug('uploadToR2 returned', { url: result.url });

    debug('Copying to clipboard...');
    await copyToClipboard(result.url);
    debug('Clipboard copy complete');

    log(result.url, 'success');
    log('Copied to clipboard', 'success');

    debug('About to exit...');
    if (usedStdin) {
      process.stdin.destroy();
    }
    process.exit(0);
  } catch (e) {
    if (e instanceof Error) {
      log(e.message, 'error');
    } else {
      log('Unknown error occurred', 'error');
    }
    process.exit(1);
  }
}

main();
