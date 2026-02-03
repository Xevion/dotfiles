// R2 upload logic

import { S3Client } from "@aws-sdk/client-s3";
import { Upload } from "@aws-sdk/lib-storage";
import { ENV, DOMAIN, TIMING, encodeUrlPath } from "../utils";
import type { UploadResult } from "../types";

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
    credentials: {
      accessKeyId: ENV.accessKeyId,
      secretAccessKey: ENV.secretAccessKey,
    },
    requestHandler: { requestTimeout: TIMING.requestTimeout },
  });

  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const key = `${year}/${month}/${filename}`;

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

    upload.on("httpUploadProgress", (progress) => {
      if (progress.loaded && onProgress) {
        onProgress(progress.loaded, buffer.length);
      }
    });

    await Promise.race([
      upload.done(),
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error("Upload timeout")), TIMING.uploadTimeout)
      ),
    ]);

    upload.removeAllListeners();

    const encodedKey = encodeUrlPath(key);
    const url = `${DOMAIN}/${encodedKey}`;
    return { url, key, size: buffer.length };
  } finally {
    client.destroy();
  }
}
