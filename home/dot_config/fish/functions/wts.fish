function wts --description "Show git status for all worktrees"
    set -l root (git rev-parse --show-toplevel 2>/dev/null)
    if test -z "$root"
        echo "Not in a git repository" >&2
        return 1
    end

    for wt_path in (git worktree list --porcelain | grep '^worktree ' | string replace 'worktree ' '')
        set -l branch (git -C "$wt_path" symbolic-ref --short HEAD 2>/dev/null; or echo "detached")
        
        # Print header with box drawing
        set_color --bold blue
        echo "╭─────────────────────────────────────────────────────────────╮"
        echo -n "│ "
        set_color --bold yellow
        echo -n "$wt_path"
        set_color normal
        echo ""
        set_color --bold blue
        echo -n "│ "
        set_color cyan
        echo "[$branch]"
        set_color --bold blue
        echo "╰─────────────────────────────────────────────────────────────╯"
        set_color normal
        
        # Run git status
        git -C "$wt_path" status --short
        echo ""
    end
end
