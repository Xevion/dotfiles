# Smart directory finder with fd + fzf (shows all directories including gitignored)
# Searches from git root if in repo, otherwise current directory
function _fzf_discover_directory --description "Alt+D: Discover directories with fd+fzf (no-ignore)"
    # Start from git root if in a git repo, otherwise current directory
    set -l search_root (git rev-parse --show-toplevel 2>/dev/null || pwd)

    set -l selected (
        fd --type d --hidden --no-ignore --exclude .git --exclude node_modules \
            . "$search_root" | \
        fzf --height=50% \
            --preview="lsd -1 --color=always --icon=always {}" \
            --preview-window=right,40% \
            --prompt="📁 Discover > " \
            --header="Searching from: $search_root (all dirs, no gitignore)"
    )

    if test -n "$selected"
        cd "$selected"
        commandline -f repaint
    end
end
