#!/usr/bin/env bun

/**
 * fzf-abbr-search - Search shell abbreviations, aliases, and functions
 * Output format: name\texpansion\ttype\tdisplay
 */

import { $ } from "bun";

// ANSI color codes
const colors = {
  name: "\x1b[36m",      // Cyan
  arrow: "\x1b[90m",     // Gray
  expansion: "\x1b[32m", // Green
  type: "\x1b[33m",      // Yellow
  reset: "\x1b[0m",
};

interface Item {
  name: string;
  expansion: string;
  type: "abbr" | "alias" | "func";
}

async function detectShell(): Promise<string> {
  const shell = process.env.SHELL || "";
  if (shell.includes("fish")) return "fish";
  if (shell.includes("zsh")) return "zsh";
  if (shell.includes("bash")) return "bash";
  return "bash"; // default
}

async function getAllFishItems(): Promise<Item[]> {
  const items: Item[] = [];
  
  try {
    // Combine all Fish queries into one script for efficiency
    const script = `
      # Output abbreviations
      for line in (abbr --show)
        echo "ABBR|$line"
      end
      
      # Output aliases
      for line in (alias)
        echo "ALIAS|$line"
      end
      
      # Output functions with descriptions
      for func in (functions -n | string match -v '_*')
        set -l desc (functions -D -v $func 2>/dev/null | tail -n1)
        if test -n "$desc"
          echo "FUNC|$func|$desc"
        else
          echo "FUNC|$func|$func"
        end
      end
    `;
    
    const result = await $`fish -c ${script}`.quiet();
    const lines = result.text().trim().split("\n");
    
    for (const line of lines) {
      if (!line) continue;
      
      if (line.startsWith("ABBR|")) {
        const abbrLine = line.slice(5);
        const match = abbrLine.match(/^abbr -a -- (\S+) (.+)$/);
        if (match) {
          items.push({
            name: match[1],
            expansion: match[2].replace(/^'|'$/g, ""),
            type: "abbr",
          });
        }
      } else if (line.startsWith("ALIAS|")) {
        const aliasLine = line.slice(6);
        const match = aliasLine.match(/^alias (\S+) (.+)$/);
        if (match) {
          items.push({
            name: match[1],
            expansion: match[2].replace(/^'|'$/g, ""),
            type: "alias",
          });
        }
      } else if (line.startsWith("FUNC|")) {
        const parts = line.slice(5).split("|", 2);
        if (parts.length === 2) {
          items.push({
            name: parts[0],
            expansion: parts[1],
            type: "func",
          });
        }
      }
    }
  } catch (e) {
    // Fish not available
  }
  
  return items;
}

async function getBashZshAliases(): Promise<Item[]> {
  const items: Item[] = [];
  const shell = await detectShell();
  
  if (shell === "fish") return []; // Already handled by getFishAliases
  
  try {
    const cmd = shell === "zsh" ? "zsh" : "bash";
    const result = await $`${cmd} -i -c 'alias'`.quiet();
    const lines = result.text().trim().split("\n");
    
    for (const line of lines) {
      // Format: alias name='expansion' or alias name=expansion
      const match = line.match(/^alias (\S+)=['"]?(.+?)['"]?$/);
      if (match) {
        items.push({
          name: match[1],
          expansion: match[2],
          type: "alias",
        });
      }
    }
  } catch (e) {
    // Shell not available or no aliases
  }
  
  return items;
}

async function getBashZshFunctions(): Promise<Item[]> {
  const items: Item[] = [];
  const shell = await detectShell();
  
  if (shell === "fish") return []; // Already handled by getFishFunctions
  
  try {
    const cmd = shell === "zsh" ? "zsh" : "bash";
    // List all functions (excluding internal ones starting with _)
    const result = await $`${cmd} -i -c 'declare -F'`.quiet();
    const lines = result.text().trim().split("\n");
    
    for (const line of lines) {
      const match = line.match(/^declare -f (\S+)$/);
      if (match && !match[1].startsWith("_")) {
        items.push({
          name: match[1],
          expansion: match[1], // Functions just show their name
          type: "func",
        });
      }
    }
  } catch (e) {
    // Shell not available
  }
  
  return items;
}

async function collectAllItems(): Promise<Item[]> {
  const shell = await detectShell();
  
  if (shell === "fish") {
    return await getAllFishItems();
  } else {
    const [aliases, funcs] = await Promise.all([
      getBashZshAliases(),
      getBashZshFunctions(),
    ]);
    return [...aliases, ...funcs];
  }
}

function formatDisplay(item: Item): string {
  const { name, expansion, type } = item;
  return (
    `${colors.name}${name}${colors.reset} ` +
    `${colors.arrow}=>${colors.reset} ` +
    `${colors.expansion}${expansion}${colors.reset} ` +
    `${colors.type}(${type})${colors.reset}`
  );
}

async function main() {
  const items = await collectAllItems();
  
  // Output: name\texpansion\ttype\tdisplay
  for (const item of items) {
    const display = formatDisplay(item);
    console.log(`${item.name}\t${item.expansion}\t${item.type}\t${display}`);
  }
}

main();
