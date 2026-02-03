// Filename generation

import { basename, extname } from "path";
import { nanoid } from "nanoid";
import { getExtensionForMime, normalizeFilename } from "../utils";
import type { ShareConfig } from "../types";

/**
 * Generate the final filename for upload
 */
export function generateFilename(
  originalFilename: string | undefined,
  mimeType: string,
  config: ShareConfig,
): string {
  const ext = getExtensionForMime(mimeType);

  if (config.randomFilename || !originalFilename) {
    return `${nanoid(8)}.${ext}`;
  }

  let base = basename(originalFilename, extname(originalFilename));

  if (config.normalizeFilename) {
    base = normalizeFilename(base);
  }

  return `${base}-${nanoid(8)}.${ext}`;
}
