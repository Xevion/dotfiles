function wtr --description "FZF-based git worktree remover (multi-select)"
    set -l root (git rev-parse --show-toplevel 2>/dev/null)
    if test -z "$root"
        echo "Not in a git repository" >&2
        return 1
    end

    # Get the main worktree path to exclude it
    set -l main_wt (git worktree list --porcelain | grep '^worktree ' | head -n1 | string replace 'worktree ' '')
    
    # Select worktrees with fzf (excluding main)
    set -l selected (git worktree list | grep -v "^$main_wt " | fzf --multi --height=40% --reverse \
        --header="Select worktrees to remove (Tab to multi-select)" \
        --preview="git -C {1} log --oneline -5 2>/dev/null || echo 'No commits'")
    
    if test -z "$selected"
        echo "No worktrees selected"
        return 0
    end

    echo "Will remove the following worktrees:"
    for line in $selected
        set -l wt_path (echo "$line" | awk '{print $1}')
        echo "  - $wt_path"
    end
    echo ""
    
    read -P "Confirm removal? [y/N] " -n 1 confirm
    echo ""
    
    if string match -qi 'y' "$confirm"
        for line in $selected
            set -l wt_path (echo "$line" | awk '{print $1}')
            echo "Removing $wt_path..."
            git worktree remove "$wt_path"
        end
    else
        echo "Cancelled"
    end
end
