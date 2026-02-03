// Pipeline orchestrator

import { copyToClipboard } from "../io";
import { readSource } from "./source";
import { detectIssues } from "./detect";
import { applyFixes, processMedia } from "./process";
import { uploadToR2 } from "./upload";
import { generateFilename } from "./filename";
import type { ShareConfig, PipelineResult, PipelineEvents, UploadSource, MediaIssue } from "../types";

export { readSource } from "./source";
export { detectIssues, detectVideoIssues, detectImageIssues } from "./detect";
export { applyFixes, processMedia, needsProcessing } from "./process";
export { uploadToR2 } from "./upload";
export { generateFilename } from "./filename";

/**
 * Run the complete upload pipeline
 *
 * Pipeline steps:
 * 1. Read source (already done - passed in)
 * 2. Apply auto-fixes if enabled
 * 3. Process media (encode, resize, etc.)
 * 4. Generate filename
 * 5. Upload to R2 (unless dry-run)
 * 6. Copy URL to clipboard
 */
export async function runPipeline(
  config: ShareConfig,
  source: UploadSource,
  issues: MediaIssue[],
  events?: PipelineEvents,
): Promise<PipelineResult> {
  const startTime = Date.now();
  let { buffer, mimeType, filename } = source;

  // Step 1: Apply auto-fixes
  if (config.autoFix && issues.length > 0) {
    const fixed = await applyFixes(buffer, mimeType, issues);
    buffer = fixed.buffer;
    mimeType = fixed.mimeType;
  }

  // Step 2: Process media
  events?.onProcessingStart?.();
  const processStartTime = Date.now();

  const processed = await processMedia(buffer, mimeType, config, events?.onProcessingProgress);
  buffer = processed.buffer;
  mimeType = processed.mimeType;

  const processingTime = (Date.now() - processStartTime) / 1000;
  events?.onProcessingComplete?.(buffer.length, processingTime);

  // Step 3: Generate filename
  const finalFilename = generateFilename(filename, mimeType, config);

  // Step 4: Handle dry-run
  if (config.dryRun) {
    return {
      url: "(dry-run)",
      key: finalFilename,
      originalSize: source.buffer.length,
      processedSize: buffer.length,
      processingTime,
      mimeType,
    };
  }

  // Step 5: Upload
  events?.onUploadStart?.(finalFilename);
  const uploadStartTime = Date.now();

  const result = await uploadToR2(buffer, finalFilename, mimeType, events?.onUploadProgress);

  const uploadTime = (Date.now() - uploadStartTime) / 1000;
  events?.onUploadComplete?.(uploadTime);

  // Step 6: Copy to clipboard
  await copyToClipboard(result.url);

  return {
    url: result.url,
    key: result.key,
    originalSize: source.buffer.length,
    processedSize: result.size,
    processingTime,
    mimeType,
  };
}
