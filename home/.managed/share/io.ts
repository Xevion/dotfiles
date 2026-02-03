// I/O operations for reading from various sources

import { existsSync, fstatSync } from "fs";
import { tmpdir } from "os";
import { join, basename } from "path";
import { nanoid } from "nanoid";
import { $ } from "bun";
import { IS_WSL, IS_WINDOWS, getExtensionForMime, detectMimeType } from "./utils";
import type { UploadSource } from "./types";

let usedStdin = false;

export function wasStdinUsed(): boolean {
  return usedStdin;
}

// ---------------------------------------------------------------------------
// Source reading
// ---------------------------------------------------------------------------

export async function readFromFile(path: string): Promise<UploadSource> {
  if (!existsSync(path)) {
    throw new Error(`File not found: ${path}`);
  }

  const buffer = Buffer.from(await Bun.file(path).arrayBuffer());
  const filename = basename(path);
  const mimeType = await detectMimeType(buffer, filename);

  return { buffer, filename, mimeType };
}

export function hasStdinData(): boolean {
  if (Bun.stdin.isTTY === true) return false;

  try {
    const stats = fstatSync(0);
    return stats.isFIFO() || stats.isSocket();
  } catch {
    return false;
  }
}

export async function readFromStdin(): Promise<UploadSource> {
  usedStdin = true;
  const chunks: Buffer[] = [];

  for await (const chunk of Bun.stdin.stream()) {
    chunks.push(Buffer.from(chunk));
  }

  const buffer = Buffer.concat(chunks);

  if (buffer.length === 0) {
    throw new Error("No data received from stdin");
  }

  const mimeType = await detectMimeType(buffer);
  return { buffer, mimeType };
}

export async function readFromClipboard(): Promise<UploadSource> {
  if (IS_WSL || IS_WINDOWS) {
    const result = await $`powershell.exe -NoProfile -Command "Get-Clipboard -Raw"`.text();
    const text = result.trim();

    if (!text) {
      throw new Error("Clipboard is empty");
    }

    // Check if clipboard contains a file path
    if (text.startsWith("/") && existsSync(text)) {
      return readFromFile(text);
    }

    const buffer = Buffer.from(text, "utf-8");
    return { buffer, mimeType: "text/plain" };
  }

  // Linux with xclip
  const targets = await $`xclip -selection clipboard -t TARGETS -o`.text();
  const targetList = targets.trim().split("\n");

  // Check for file URIs
  if (targetList.includes("text/uri-list") || targetList.includes("x-special/gnome-copied-files")) {
    const uris = await $`xclip -selection clipboard -t text/uri-list -o`.text();
    const filePath = uris.split("\n")[0].replace(/^file:\/\//, "").trim();

    if (filePath && existsSync(filePath)) {
      return readFromFile(filePath);
    }
  }

  // Check for images
  const imageTarget = targetList.find((t) => t.startsWith("image/"));
  if (imageTarget) {
    const buffer = Buffer.from(
      await $`xclip -selection clipboard -t ${imageTarget} -o`.arrayBuffer()
    );

    // BMP from clipboard needs conversion
    if (imageTarget === "image/bmp") {
      return { buffer, mimeType: "image/bmp", filename: "paste.bmp" };
    }

    return {
      buffer,
      mimeType: imageTarget,
      filename: `paste.${getExtensionForMime(imageTarget)}`,
    };
  }

  // Fall back to text
  const text = await $`xclip -selection clipboard -o`.text();
  if (!text.trim()) {
    throw new Error("Clipboard is empty or contains unsupported data");
  }

  // Check if text is a file path
  const trimmedText = text.trim();
  if (trimmedText.startsWith("/") && existsSync(trimmedText)) {
    return readFromFile(trimmedText);
  }

  const buffer = Buffer.from(text, "utf-8");
  return { buffer, mimeType: "text/plain" };
}

// ---------------------------------------------------------------------------
// Clipboard output
// ---------------------------------------------------------------------------

export async function copyToClipboard(text: string): Promise<void> {
  if (IS_WSL || IS_WINDOWS) {
    const proc = Bun.spawn(["clip.exe"], { stdin: "pipe" });
    proc.stdin.write(text);
    proc.stdin.end();
    await proc.exited;
  } else {
    const proc = Bun.spawn(["xclip", "-selection", "clipboard"], { stdin: "pipe" });
    proc.stdin.write(text);
    proc.stdin.end();
    await proc.exited;
  }
}

// ---------------------------------------------------------------------------
// Temp files
// ---------------------------------------------------------------------------

export async function writeTempFile(buffer: Buffer, ext = ""): Promise<string> {
  const path = join(tmpdir(), `share-${nanoid(8)}${ext}`);
  await Bun.write(path, buffer);
  return path;
}

export async function cleanupTempFiles(...paths: string[]): Promise<void> {
  for (const path of paths) {
    try {
      await $`rm -f ${path}`.quiet();
    } catch { /* ignore */ }
  }
}
