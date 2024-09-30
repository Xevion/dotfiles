# dotfiles

A [chezmoi][chezmoi]-activated dotfiles repository containing all settings related to any number of activities for me: Xevion.

This repository is not intended for others, but is kept public in the spirit of Open Source, as well as a test of my ability to build an encrypted config in the open.

> [!NOTE]
> If you're not used to Chezmoi, it's a dotfile manager for multiple machines, with special abilities like templating, encryption, secret management, scripting, and more.

> [!WARNING]
> While Chezmoi is a powerful tool with mass configurability, it is not a fully featured declarative configuration manager. This warning is more for myself than others: if you want a fully declarative OS, go install NixOS dumbass.

## Links

Documentation, references, etc.

- [www.chezmoi.io][chezmoi]

## Important Commands

Commands that are of chief importance for managing the repository, scripts, etc.

#### Installation/Bootstrap

```bash
# Install only
sh -c "$(curl -fsLS get.chezmoi.io)" -- init Xevion -b $HOME/.local/bin

# Bootstrap immediately (apply configurations, potentially overwriting existing files)
sh -c "$(curl -fsLS get.chezmoi.io)" -- init Xevion -b $HOME/.local/bin --apply
```

#### Basic

```bash
# Change to the dotfiles 'source' directory
chezmoi cd
# With a specific directory
chezmoi cd ~/.ssh/
```

#### Status

```bash
# Print all unmanaged files in the home directory (tree style, absolute path)
chezmoi unmanaged ~/ -t -p absolute
```

- Not yet developed
    - List all changed config files (source vs  )
    - Show diffing, search via `fzf` or something.
    - Show diffing, fuzzy matching

## Intended Goals

- Encrypted configurations for private details
- Bootstrapping for any Ubuntu-based system
    - WSL-support
        - Windows integration?
    - Device-specific configuration
- Automatic package/script install/setup
    - Neovim
    - VS Code
- Terminal startup for when configs changed, out of date, etc.
    - Ideally, this would run on a timer and only fetch the latest status upon terminal startup, keeping startup times low.
- GitHub Language Attributes Script
    - Automatically update the language attributes for GitHub repositories
    - Prehook for commits, updates .gitattributes
    - [Reference](https://github.com/github-linguist/linguist/blob/main/docs/overrides.md)

[chezmoi]: https://www.chezmoi.io/