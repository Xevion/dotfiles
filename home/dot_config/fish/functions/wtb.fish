function wtb --description "Add git worktree with new branch"
    if test (count $argv) -lt 1
        echo "Usage: wtb <branch-name> [base-ref]" >&2
        return 1
    end

    set -l root (git rev-parse --show-toplevel 2>/dev/null)
    if test -z "$root"
        echo "Not in a git repository" >&2
        return 1
    end

    set -l branch $argv[1]
    
    # Get base ref: argument, or current branch tip
    set -l base
    if test (count $argv) -ge 2
        set base $argv[2]
    else
        # Get current branch name (not HEAD if detached)
        set base (git symbolic-ref --short HEAD 2>/dev/null)
        if test -z "$base"
            set base (git rev-parse HEAD)
        end
    end

    # Sanitize branch name for directory (replace / with -)
    set -l dir_name (string replace --all '/' '-' "$branch")
    set -l wt_path "$root/.worktrees/$dir_name"

    # Create .worktrees directory if needed
    mkdir -p "$root/.worktrees"

    # Add to .gitignore if not already there
    if not grep -q '^\.worktrees/?$' "$root/.gitignore" 2>/dev/null
        echo ".worktrees/" >> "$root/.gitignore"
        echo "Added .worktrees/ to .gitignore"
    end

    echo "Creating worktree at $wt_path"
    echo "  Branch: $branch"
    echo "  Base: $base"
    
    if git worktree add -b "$branch" "$wt_path" "$base"
        echo ""
        echo "Worktree created: $wt_path"
        
        # Clone gitignored files from current worktree
        set -l current_wt (pwd)
        _wt_clone_ignored "$current_wt" "$wt_path"
        
        cd "$wt_path"
    end
end
