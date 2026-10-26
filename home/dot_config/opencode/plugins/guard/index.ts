import { spawnSync } from "node:child_process"
import { homedir } from "node:os"
import { join } from "node:path"
import { Plugin } from "@opencode/plugin"

type Input = Record<string, unknown>

interface Verdict {
  blocked: boolean
  message: string
  command?: string
  context?: string
}

const EDIT_TOOLS = new Set(["edit", "write"])

function runGuard(binary: string, payload: object, timeout: number): Verdict | undefined {
  const proc = spawnSync(binary, ["hook"], { input: JSON.stringify(payload), encoding: "utf8", timeout })
  if (proc.error || proc.status === null) return undefined

  const verdict: Verdict = { blocked: proc.status === 2, message: proc.stderr.trim() }
  try {
    const out = JSON.parse(proc.stdout || "{}").hookSpecificOutput ?? {}
    verdict.command = out.updatedInput?.command
    verdict.context = out.additionalContext
  } catch {
    // stdout is advisory; a malformed body must not mask the exit status
  }
  return verdict
}

function append(result: { content?: unknown } | undefined, text: string) {
  if (result && typeof result.content === "string") result.content += `\n\n${text}`
}

export default Plugin.define({
  id: "guard",
  async setup(ctx) {
    const binary = typeof ctx.options.binary === "string" ? ctx.options.binary : join(homedir(), ".claude/hooks/guard")
    const cwd = ctx.location.directory
    // Advisory context from a rewrite, keyed by the command that actually runs.
    const pendingContext = new Map<string, string>()

    await ctx.tool.hook("execute.before", (event) => {
      if (event.tool !== "bash") return
      const input = event.input as Input
      const verdict = runGuard(
        binary,
        { hook_event_name: "PreToolUse", tool_name: "bash", cwd, tool_input: input },
        3000,
      )
      if (!verdict) return
      if (verdict.blocked) throw new Error(verdict.message || "Blocked by guard")

      const command = verdict.command ?? (input.command as string | undefined)
      if (verdict.command) event.input = { ...input, command: verdict.command }
      if (verdict.context && command) pendingContext.set(command, verdict.context)
    })

    await ctx.tool.hook("execute.after", (event) => {
      if (event.status !== "completed") return
      const input = event.input as Input | undefined
      const result = event.result as { content?: unknown } | undefined

      if (event.tool === "bash") {
        const command = input?.command as string | undefined
        const context = command ? pendingContext.get(command) : undefined
        if (command) pendingContext.delete(command)
        if (context) append(result, context)
        return
      }
      if (!EDIT_TOOLS.has(event.tool) || !input) return

      const verdict = runGuard(
        binary,
        { hook_event_name: "PostToolUse", tool_name: event.tool, cwd, tool_input: input },
        5000,
      )
      if (!verdict) return
      const note = verdict.blocked ? verdict.message : verdict.context
      if (note) append(result, note)
    })
  },
})
