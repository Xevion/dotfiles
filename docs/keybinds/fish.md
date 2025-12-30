# Fish Shell Keybindings

Complete reference for Fish shell keybindings, including built-in defaults, plugins, and custom configurations.

## Custom Bindings

Personal keybindings configured in this dotfiles repository.

| Keybinding | Action | Source |
|------------|--------|--------|
| Alt+a | `fzf_search_abbr` | `config.fish` - Search abbreviations/aliases with fzf |

## Quick Reference

### Most Useful "Hidden" Bindings

These are the bindings most users don't know about:

| Keybinding | What it does |
|------------|--------------|
| Alt+s | Toggle `sudo` prefix |
| Alt+. | Repeat last argument from previous command |
| Alt+e | Edit long commands in your editor |
| Alt+o | Preview file under cursor |
| Alt+l | Quick `ls` of path under cursor |
| Alt+w | `whatis` for command under cursor |
| Alt+p | Pipe output through pager automatically |
| Alt+# | Comment out command (save to history without running) |
| Alt+Up/Down | Search history for current token |
| F1 | Instant man page for current command |
| Ctrl+x / Ctrl+v | Native clipboard integration |
| Shift+Delete | Delete current history entry |

### Emacs-Style Navigation Cheatsheet

```
Ctrl+a ─────────────────────────────────────────────────── Ctrl+e
       │                                                 │
       └─ Ctrl+b ◄─── cursor ───► Ctrl+f ────────────────┘
                        │
                  Ctrl+Left/Right (by word)
                  Shift+Left/Right (by WORD)
```

### Kill Ring

Fish maintains a "kill ring" of deleted text:

1. `Ctrl+k` / `Ctrl+u` / `Alt+d` - Kill (cut) text
2. `Ctrl+y` - Yank (paste) most recent kill
3. `Alt+y` - Cycle through previous kills

## Plugin Bindings

Keybindings added by Fish plugins (Fisher).

### Atuin (History)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+r | `_atuin_search` | Search shell history (replaces built-in `history-pager`) |

### fzf.fish (PatrickF1/fzf.fish)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+Alt+f | `fzf_search_directory` | Search files in current directory |
| Ctrl+Alt+l | `fzf_search_git_log` | Search git log |
| Ctrl+Alt+s | `fzf_search_git_status` | Search git status |
| Ctrl+Alt+p | `fzf_search_processes` | Search running processes |
| Alt+Ctrl+v | `fzf_search_variables` | Search shell variables (remapped from Ctrl+v) |

> Note: `Ctrl+r` (history) is disabled in fzf.fish config to let Atuin handle it.

## Built-in Bindings

Default Fish shell keybindings (emacs mode).

### Execution & Completion

| Keybinding | Action | Description |
|------------|--------|-------------|
| Enter | `execute` | Execute current command |
| Ctrl+j | `execute` | Execute current command |
| Ctrl+m | `execute` | Execute current command |
| Ctrl+Enter | `execute` | Execute current command (prevents mistypes) |
| Tab | `complete` | Autocomplete |
| Ctrl+i | `complete` | Autocomplete (same as Tab) |
| Shift+Tab | `complete-and-search` | Complete and open search pager |
| Ctrl+s | `pager-toggle-search` | Toggle search in completion pager |

### Navigation (Line)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+a | `beginning-of-line` | Jump to start of line |
| Ctrl+e | `end-of-line` | Jump to end of line |
| Home | `beginning-of-line` | Jump to start of line |
| End | `end-of-line` | Jump to end of line |
| Ctrl+f | `forward-char` | Move right one character |
| Ctrl+b | `backward-char` | Move left one character |
| Right | `forward-char` | Move right one character |
| Left | `backward-char` | Move left one character |

### Navigation (Word)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+Right | `forward-word` | Jump forward one word |
| Ctrl+Left | `backward-word` | Jump backward one word |
| Shift+Right | `forward-bigword` | Jump forward one WORD (whitespace-delimited) |
| Shift+Left | `backward-bigword` | Jump backward one WORD |
| Alt+f | `nextd-or-forward-token` | Next directory OR forward token |
| Alt+b | `prevd-or-backward-word` | Previous directory OR backward word |
| Alt+Right | `nextd-or-forward-token` | Next directory OR forward token |
| Alt+Left | `prevd-or-backward-token` | Previous directory OR backward token |

### Navigation (Buffer)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Alt+< | `beginning-of-buffer` | Jump to start of buffer |
| Alt+> | `end-of-buffer` | Jump to end of buffer |

### Text Editing (Delete)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Backspace | `backward-delete-char` | Delete character before cursor |
| Shift+Backspace | `backward-delete-char` | Delete character before cursor |
| Ctrl+h | `backward-delete-char` | Delete character before cursor |
| Delete | `delete-char` | Delete character at cursor |
| Ctrl+d | `delete-or-exit` | Delete character or exit if line empty |
| Ctrl+w | `backward-kill-path-component` | Delete previous path component |
| Ctrl+u | `backward-kill-line` | Delete from cursor to start of line |
| Ctrl+k | `kill-line` | Delete from cursor to end of line |
| Alt+Backspace | `backward-kill-token` | Delete previous token |
| Ctrl+Alt+h | `backward-kill-token` | Delete previous token |
| Ctrl+Backspace | `backward-kill-word` | Delete previous word |
| Alt+Delete | `kill-token` | Delete token ahead |
| Ctrl+Delete | `kill-word` | Delete word ahead |
| Alt+d | `kill-word` / `dirh` | Delete word ahead OR show directory history if empty |

### Text Editing (Transform)

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+t | `transpose-chars` | Swap current and previous characters |
| Alt+t | `transpose-words` | Swap current and previous words |
| Alt+u | `upcase-word` | UPPERCASE the current word |
| Alt+c | `capitalize-word` | Capitalize the current word |

### Undo & Redo

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+z | `undo` | Undo last edit |
| Ctrl+/ | `undo` | Undo last edit |
| Ctrl+_ | `undo` | Undo last edit (XTerm) |
| Ctrl+Shift+z | `redo` | Redo last undone edit |
| Alt+/ | `redo` | Redo last undone edit |

### Clipboard & Yank

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+x | `fish_clipboard_copy` | Copy commandline to system clipboard |
| Ctrl+v | `fish_clipboard_paste` | Paste from system clipboard |
| Ctrl+y | `yank` | Yank (paste) last killed text |
| Alt+y | `yank-pop` | Cycle through kill ring |

### History Navigation

| Keybinding | Action | Description |
|------------|--------|-------------|
| Up | `up-or-search` | Previous history / prefix search |
| Down | `down-or-search` | Next history / prefix search |
| Ctrl+p | `up-or-search` | Previous history / prefix search |
| Ctrl+n | `down-or-search` | Next history / prefix search |
| PageUp | `beginning-of-history` | Jump to oldest history entry |
| PageDown | `end-of-history` | Jump to newest history entry |
| Shift+Delete | `history-delete` | Delete current history entry |

### History Token Search

| Keybinding | Action | Description |
|------------|--------|-------------|
| Alt+Up | `history-token-search-backward` | Search history for token under cursor (backward) |
| Alt+Down | `history-token-search-forward` | Search history for token under cursor (forward) |
| Alt+. | `history-token-search-backward` | Insert last argument from previous command |

### Special Functions

| Keybinding | Action | Description |
|------------|--------|-------------|
| Alt+s | `fish_commandline_prepend sudo` | Toggle `sudo` prefix on current line |
| Alt+e | `edit_command_buffer` | Open command in `$EDITOR` |
| Alt+v | `edit_command_buffer` | Open command in `$EDITOR` |
| Alt+p | `__fish_paginate` | Pipe command output through pager |
| Alt+# | `__fish_toggle_comment_commandline` | Toggle `#` comment on line |
| Alt+l | `__fish_list_current_token` | List/expand current token (like `ls` on path) |
| Alt+o | `__fish_preview_current_file` | Preview file under cursor |
| Alt+w | `__fish_whatis_current_token` | Show `whatis` for command under cursor |
| F1 | `__fish_man_page` | Open man page for current command |
| Alt+h | `__fish_man_page` | Open man page for current command |

### Line Control

| Keybinding | Action | Description |
|------------|--------|-------------|
| Ctrl+l | `clear-screen` | Clear screen (with scrollback push if supported) |
| Ctrl+c | `clear-commandline` | Clear current command line |
| Ctrl+g | `cancel` | Cancel current operation |
| Escape | `cancel` | Cancel / exit mode |
| Ctrl+[ | `cancel` | Cancel (escape sequence) |

### Text Insertion

| Keybinding | Action | Description |
|------------|--------|-------------|
| Space | `self-insert expand-abbr` | Insert space and expand abbreviations |
| ; | `self-insert expand-abbr` | Insert semicolon and expand abbreviations |
| \| | `self-insert expand-abbr` | Insert pipe and expand abbreviations |
| & | `self-insert expand-abbr` | Insert ampersand and expand abbreviations |
| > | `self-insert expand-abbr` | Insert redirect and expand abbreviations |
| < | `self-insert expand-abbr` | Insert redirect and expand abbreviations |
| ) | `self-insert expand-abbr` | Insert paren and expand abbreviations |
| Shift+Enter | `commandline -i \n` | Insert newline without executing |
| Alt+Enter | `commandline -i \n` | Insert newline without executing |
| Ctrl+Space | Insert literal space | Insert space without expanding abbreviations |
| Shift+Space | `self-insert expand-abbr` | Insert space (same as Space) |

## Configuration

Keybindings are configured in:

- **Built-in**: `fish_default_key_bindings` function (emacs mode)
- **Custom**: `~/.config/fish/config.fish` (this repo: `home/dot_config/fish/config.fish.tmpl`)
- **Plugins**: Managed via Fisher, configured in `config.fish`

### Checking Current Bindings

```fish
# List all bindings
bind

# List preset (built-in) bindings
bind --preset

# List user-defined bindings
bind --user

# Show what a specific key does
bind \ea  # Shows Alt+a binding
```

### Adding Custom Bindings

```fish
# In config.fish
bind \ek 'echo "Hello"'           # Alt+k
bind -M insert \ek 'echo "Hello"' # Also bind in insert mode (vi)
```
