// Source reading for the pipeline

import { readFromFile, readFromStdin, readFromClipboard } from "../io";
import type { SourceConfig, UploadSource } from "../types";

/**
 * Read source data based on configuration
 */
export async function readSource(source: SourceConfig): Promise<UploadSource> {
  switch (source.type) {
    case "file":
      if (!source.path) {
        throw new Error("File path required for file source");
      }
      return readFromFile(source.path);

    case "stdin":
      return readFromStdin();

    case "clipboard":
      return readFromClipboard();

    default:
      throw new Error(`Unknown source type: ${source.type}`);
  }
}
