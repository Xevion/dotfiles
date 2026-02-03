// FFmpeg command execution utilities

// Global verbose flag - set via setVerbose()
let verboseMode = false;

export function setVerbose(enabled: boolean): void {
  verboseMode = enabled;
}

function log(...args: unknown[]): void {
  if (verboseMode) {
    console.log("[verbose]", ...args);
  }
}

/**
 * Parse FFmpeg time string (HH:MM:SS.ms) to seconds
 */
export function parseTimeToSeconds(time: string): number {
  const match = time.match(/(\d+):(\d+):(\d+(?:\.\d+)?)/);
  if (!match) return 0;
  const [, hours, minutes, seconds] = match;
  return parseInt(hours) * 3600 + parseInt(minutes) * 60 + parseFloat(seconds);
}

/**
 * Run a media command (ffmpeg, convert, etc.) and throw on failure
 */
export async function runMediaCommand(cmd: string[]): Promise<void> {
  const startTime = performance.now();
  log("Running:", cmd.join(" "));
  
  const proc = Bun.spawn(cmd, { stdout: "ignore", stderr: "pipe" });
  const exitCode = await proc.exited;

  const elapsed = ((performance.now() - startTime) / 1000).toFixed(2);
  
  if (exitCode !== 0) {
    const stderr = await new Response(proc.stderr).text();
    log(`Command failed after ${elapsed}s`);
    throw new Error(`Command failed: ${stderr.trim().split("\n").slice(-3).join("\n")}`);
  }
  
  log(`Completed in ${elapsed}s`);
}

/**
 * Run FFmpeg with progress tracking
 */
export async function runFfmpegWithProgress(
  cmd: string[],
  durationSeconds: number,
  onProgress: (percent: number) => void,
): Promise<void> {
  const startTime = performance.now();
  log("Running (with progress):", cmd.join(" "));
  
  return new Promise((resolve, reject) => {
    const proc = Bun.spawn(cmd, {
      stdout: "ignore",
      stderr: "pipe",
    });

    const reader = proc.stderr.getReader();
    let buffer = "";

    const readStderr = async () => {
      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += new TextDecoder().decode(value);

          // Parse time= from ffmpeg output
          const timeMatch = buffer.match(/time=(\d+:\d+:\d+\.\d+|\d+:\d+\.\d+)/g);
          if (timeMatch) {
            const lastTime = timeMatch[timeMatch.length - 1].replace("time=", "");
            // Handle both HH:MM:SS.ms and MM:SS.ms formats
            const normalizedTime = lastTime.split(":").length === 2 ? `00:${lastTime}` : lastTime;
            const currentSeconds = parseTimeToSeconds(normalizedTime);

            if (durationSeconds > 0) {
              const percent = Math.min(99, Math.floor((currentSeconds / durationSeconds) * 100));
              onProgress(percent);
            }
          }

          // Keep buffer small
          if (buffer.length > 4096) {
            buffer = buffer.slice(-2048);
          }
        }
      } catch {
        // Ignore read errors on stderr
      }
    };

    readStderr();

    proc.exited.then((exitCode) => {
      const elapsed = ((performance.now() - startTime) / 1000).toFixed(2);
      if (exitCode === 0) {
        log(`Completed in ${elapsed}s`);
        onProgress(100);
        resolve();
      } else {
        log(`Command failed after ${elapsed}s`);
        reject(new Error(`ffmpeg exited with code ${exitCode}`));
      }
    }).catch(reject);
  });
}

/**
 * Get the scale filter string for a resolution option
 */
export function getScaleFilter(resolution: string): string | undefined {
  switch (resolution) {
    case "1080p": return "scale=-2:1080";
    case "720p": return "scale=-2:720";
    case "480p": return "scale=-2:480";
    case "50%": return "scale=trunc(iw/4)*2:trunc(ih/4)*2";
    case "25%": return "scale=trunc(iw/8)*2:trunc(ih/8)*2";
    default: return undefined;
  }
}
