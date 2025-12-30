/**
 * Shared utilities for fzf-based tools
 */

// Standardized color scheme for all fzf tools
export const colors = {
  name: "\x1b[36m",      // Cyan - primary item/name
  arrow: "\x1b[90m",     // Gray - separators/arrows
  expansion: "\x1b[32m", // Green - positive/expansion/success
  type: "\x1b[33m",      // Yellow - type/category labels
  add: "\x1b[32m",       // Green - additions
  modify: "\x1b[33m",    // Yellow - modifications
  delete: "\x1b[91m",    // Red - deletions/removals
  script: "\x1b[35m",    // Purple - special actions/scripts
  reset: "\x1b[0m",
};

// Type indicators for chezmoi files (max 3 shown)
export const typeIcons = {
  encrypted: "🔒",
  private: "🔐",
  template: "📝",
  executable: "⚡",
  symlink: "🔗",
  script: "▶️",
};

export interface ChezmoiFileFlags {
  isEncrypted: boolean;
  isPrivate: boolean;
  isTemplate: boolean;
  isExecutable: boolean;
  isSymlink: boolean;
  isScript: boolean;
}

/**
 * Parse a chezmoi source filename to extract type flags
 */
export function parseSourceFile(source: string): ChezmoiFileFlags {
  const basename = source.split("/").pop() || source;
  
  return {
    isEncrypted: basename.includes("encrypted_"),
    isPrivate: basename.includes("private_"),
    isTemplate: basename.endsWith(".tmpl"),
    isExecutable: basename.includes("executable_"),
    isSymlink: basename.includes("symlink_"),
    isScript: basename.startsWith("run_"),
  };
}

/**
 * Get emoji indicators for file type flags (max 3)
 */
export function getTypeIndicators(flags: ChezmoiFileFlags): string {
  const indicators: string[] = [];
  
  // Priority order for showing indicators
  if (flags.isScript) indicators.push(typeIcons.script);
  if (flags.isEncrypted) indicators.push(typeIcons.encrypted);
  if (flags.isPrivate && !flags.isEncrypted) indicators.push(typeIcons.private);
  if (flags.isExecutable) indicators.push(typeIcons.executable);
  if (flags.isSymlink) indicators.push(typeIcons.symlink);
  if (flags.isTemplate) indicators.push(typeIcons.template);
  
  // Limit to 3 indicators
  return indicators.slice(0, 3).join("");
}

/**
 * Get the primary color for a file based on its type
 */
export function getFileColor(flags: ChezmoiFileFlags): string {
  if (flags.isScript) return colors.script;
  if (flags.isEncrypted) return colors.delete; // Red for encrypted (sensitive)
  if (flags.isPrivate) return colors.modify;   // Yellow for private
  if (flags.isExecutable) return colors.add;   // Green for executable
  if (flags.isSymlink) return colors.name;     // Cyan for symlink
  if (flags.isTemplate) return colors.script;  // Purple for template
  return colors.reset;
}

/**
 * Pad a string to a fixed width (for column alignment)
 */
export function padRight(str: string, width: number): string {
  // Account for emoji width (most are 2 chars wide in terminals)
  const emojiCount = (str.match(/[\u{1F300}-\u{1F9FF}]|[\u{2600}-\u{26FF}]/gu) || []).length;
  const visualWidth = str.length + emojiCount;
  const padding = Math.max(0, width - visualWidth);
  return str + " ".repeat(padding);
}

/**
 * Format an error message for display
 */
export function formatError(message: string): string {
  return `${colors.delete}Error:${colors.reset} ${message}`;
}
