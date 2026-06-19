// R2 upload logic

import { S3Client } from "bun";
import { ENV, DOMAIN, TIMING, encodeUrlPath } from "../utils";
import type { UploadResult } from "../types";

// Multipart part size. Files at or below this upload in a single request;
// larger files stream in chunks so progress can be reported per part.
const PART_SIZE = 5 * 1024 * 1024;

/**
 * Upload a file to R2 storage
 */
export async function uploadToR2(
  buffer: Buffer,
  filename: string,
  mimeType: string,
  onProgress?: (loaded: number, total: number) => void,
): Promise<UploadResult> {
  if (!ENV.endpoint || !ENV.accessKeyId || !ENV.secretAccessKey || !ENV.bucket) {
    throw new Error("Missing R2 credentials");
  }

  const client = new S3Client({
    region: "auto",
    endpoint: ENV.endpoint,
    accessKeyId: ENV.accessKeyId,
    secretAccessKey: ENV.secretAccessKey,
    bucket: ENV.bucket,
  });

  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const key = `${year}/${month}/${filename}`;

  const file = client.file(key);

  const upload = async () => {
    // Small payloads (the common case: screenshots, clipboard) go in one PUT.
    if (buffer.length <= PART_SIZE) {
      await file.write(buffer, { type: mimeType });
      onProgress?.(buffer.length, buffer.length);
      return;
    }

    // Larger payloads stream as multipart so we can surface upload progress.
    const writer = file.writer({
      type: mimeType,
      retry: 3,
      queueSize: 10,
      partSize: PART_SIZE,
    });

    let written = 0;
    for (let offset = 0; offset < buffer.length; offset += PART_SIZE) {
      const chunk = buffer.subarray(offset, Math.min(offset + PART_SIZE, buffer.length));
      writer.write(chunk);
      await writer.flush();
      written += chunk.length;
      onProgress?.(written, buffer.length);
    }
    await writer.end();
  };

  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error("Upload timeout")), TIMING.uploadTimeout);
  });

  try {
    await Promise.race([upload(), timeout]);
  } finally {
    clearTimeout(timer);
  }

  const encodedKey = encodeUrlPath(key);
  const url = `${DOMAIN}/${encodedKey}`;
  return { url, key, size: buffer.length };
}
