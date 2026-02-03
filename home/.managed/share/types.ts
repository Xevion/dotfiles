// Types and interfaces for the share CLI tool

// ---------------------------------------------------------------------------
// Core data types
// ---------------------------------------------------------------------------

export interface UploadSource {
  buffer: Buffer;
  filename?: string;
  mimeType: string;
}

export interface UploadResult {
  url: string;
  key: string;
  size: number;
}

// ---------------------------------------------------------------------------
// Media probing
// ---------------------------------------------------------------------------

export interface VideoProbe {
  duration: number | null;
  codec: string | null;
  hasFaststart: boolean;
  width: number | null;
  height: number | null;
}

export interface ImageProbe {
  width: number | null;
  height: number | null;
}

// ---------------------------------------------------------------------------
// Media issues
// ---------------------------------------------------------------------------

export interface MediaIssue {
  id: string;
  description: string;
  severity: "error" | "warning";
  fix: (buffer: Buffer) => Promise<Buffer>;
}

// ---------------------------------------------------------------------------
// Option types
// ---------------------------------------------------------------------------

export type FormatOption = "original" | "png" | "jpeg" | "webp" | "mp4" | "webm" | "gif";
export type ResolutionOption = "original" | "1080p" | "720p" | "480p" | "50%" | "25%";
export type FpsOption = "original" | "60" | "30" | "24" | "15";
export type EncoderOption = "cpu" | "nvenc" | "av1";
export type ImageQuality = "high" | "balanced" | "small";

// ---------------------------------------------------------------------------
// CLI arguments (raw parsed from command line)
// ---------------------------------------------------------------------------

export interface CliArgs {
  dryRun: boolean;
  format?: FormatOption;
  resolution?: ResolutionOption;
  fps?: FpsOption;
  encoder?: EncoderOption;
  crf?: number;
  removeAudio?: boolean;
  imageQuality?: ImageQuality;
  randomFilename?: boolean;
  normalizeFilename?: boolean;
  autoFix?: boolean;
  gifFps?: number;
  gifWidth?: number | null;
  probe?: boolean;
  noProbe?: boolean;
  verbose?: boolean;
}

// ---------------------------------------------------------------------------
// Source configuration
// ---------------------------------------------------------------------------

export type SourceType = "file" | "stdin" | "clipboard";

export interface SourceConfig {
  type: SourceType;
  path?: string;
}

// ---------------------------------------------------------------------------
// Share configuration (fully resolved)
// ---------------------------------------------------------------------------

export interface ShareConfig {
  source: SourceConfig;
  dryRun: boolean;

  // Media options
  format: FormatOption;
  resolution: ResolutionOption;

  // Video options
  fps: FpsOption;
  removeAudio: boolean;
  encoder: EncoderOption;
  crf: number;

  // GIF options
  gifFps: number;
  gifWidth: number | null;

  // Image options
  imageQuality: ImageQuality;

  // Filename options
  randomFilename: boolean;
  normalizeFilename: boolean;

  // Auto-fix
  autoFix: boolean;
}

// ---------------------------------------------------------------------------
// Video encoding settings
// ---------------------------------------------------------------------------

export interface VideoEncodeSettings {
  format: FormatOption;
  resolution: ResolutionOption;
  fps: FpsOption;
  removeAudio: boolean;
  encoder: EncoderOption;
  crf: number;
}

export interface GifSettings {
  fps: number;
  width: number | null;
}

export interface ImageSettings {
  format: FormatOption;
  quality: ImageQuality;
  scale?: string;
}

// ---------------------------------------------------------------------------
// Quality probing
// ---------------------------------------------------------------------------

export interface QualityProbeResult {
  crf: number;
  sampleSize: number;
  sampleDuration: number;
  encodeTime: number;
  estimatedFullSize: number;
  estimatedFullTime: number;
}

export interface SampleResult {
  buffer: Buffer;
  actualDuration: number;
}

// ---------------------------------------------------------------------------
// Pipeline types
// ---------------------------------------------------------------------------

export interface PipelineResult {
  url: string;
  key: string;
  originalSize: number;
  processedSize: number;
  processingTime: number;
  mimeType: string;
}

export interface PipelineEvents {
  onSourceRead?: (source: UploadSource) => void;
  onIssuesDetected?: (issues: MediaIssue[]) => void;
  onProcessingStart?: () => void;
  onProcessingProgress?: (percent: number) => void;
  onProcessingComplete?: (size: number, time: number) => void;
  onUploadStart?: (filename: string) => void;
  onUploadProgress?: (loaded: number, total: number) => void;
  onUploadComplete?: (time: number) => void;
}

// ---------------------------------------------------------------------------
// UI types
// ---------------------------------------------------------------------------

export interface UI {
  intro(): void;
  showSourceInfo(mimeType: string, size: number): void;
  showIssues(issues: MediaIssue[]): void;
  showProbeResults(results: QualityProbeResult[]): void;
  showDryRunSummary(config: ShareConfig, filename: string, size: number, mimeType: string): void;
  showResult(result: PipelineResult): void;
  error(message: string): void;
  warn(message: string): void;
  info(message: string): void;
  cancel(message?: string): never;

  // Spinner operations
  spinnerStart(message: string): void;
  spinnerUpdate(message: string): void;
  spinnerStop(message: string): void;

  // Pipeline events for progress
  events: PipelineEvents;
}

// ---------------------------------------------------------------------------
// Prompt types (for interactive config building)
// ---------------------------------------------------------------------------

export interface FormatPromptOption {
  value: FormatOption;
  label: string;
  hint?: string;
}

export interface ResolutionPromptOption {
  value: ResolutionOption;
  label: string;
}

export interface EncoderPromptOption {
  value: EncoderOption;
  label: string;
  hint?: string;
}

export interface SizeTargetOption {
  value: number | "manual";
  label: string;
  hint?: string;
}

export interface CrfOption {
  value: number;
  label: string;
  hint?: string;
}
