import type { Plugin } from "@opencode-ai/plugin"

const CHEZMOI_DIR = "/home/xevion/.local/share/chezmoi"

interface ToolExecuteInput {
  tool: string
  sessionID: string
  callID: string
}

interface ToolExecuteOutputBefore {
  args: {
    filePath?: string
    [key: string]: unknown
  }
  [key: string]: unknown
}

interface ToolExecuteOutputAfter {
  output?: string
  [key: string]: unknown
}

export const ChezmoiContextPlugin: Plugin = async (ctx) => {
  // Store warnings by callID
  const warnings = new Map<string, string>()

  return {
    "tool.execute.before": async (
      input: ToolExecuteInput,
      output: ToolExecuteOutputBefore
    ): Promise<void> => {
      // Only check read, edit, and write tools
      if (input.tool !== "read" && input.tool !== "edit" && input.tool !== "write") {
        return
      }

      const filePath = output.args.filePath
      
      if (!filePath || typeof filePath !== "string") {
        return
      }

      // Resolve to absolute path
      const absolutePath = filePath.startsWith("/") 
        ? filePath 
        : `${ctx.directory}/${filePath}`

      // Check if outside Chezmoi directory
      if (!absolutePath.startsWith(CHEZMOI_DIR)) {
        const warning = `<system-reminder>
⚠️  CONTEXT WARNING: This file is OUTSIDE the Chezmoi managed directory.

File location: ${absolutePath}
Chezmoi directory: ${CHEZMOI_DIR}

This file is NOT part of your dotfiles configuration. Changes here will:
- NOT be tracked by Chezmoi
- NOT sync across machines  
- Affect the local system directly

If you intended to edit a dotfile, make sure you're working within ${CHEZMOI_DIR}.
</system-reminder>

`
        // Store warning for the after hook
        warnings.set(input.callID, warning)
      }
    },

    "tool.execute.after": async (
      input: ToolExecuteInput,
      output: ToolExecuteOutputAfter
    ): Promise<void> => {
      // Check if we stored a warning for this call
      const warning = warnings.get(input.callID)
      if (warning && typeof output.output === "string") {
        output.output = warning + output.output
        warnings.delete(input.callID)
      }
    },
  }
}
