function wtcd --description "FZF-based worktree picker - cd into selected"
    set -l root (git rev-parse --show-toplevel 2>/dev/null)
    if test -z "$root"
        echo "Not in a git repository" >&2
        return 1
    end

    set -l selected (git worktree list | fzf --height=40% --reverse \
        --header="Select worktree" \
        --preview="git -C {1} log --oneline -5 2>/dev/null; echo ''; git -C {1} status --short 2>/dev/null")
    
    if test -n "$selected"
        set -l wt_path (echo "$selected" | awk '{print $1}')
        cd "$wt_path"
    end
end
