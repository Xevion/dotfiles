# Directory finder starting from HOME with fd + fzf (shows all directories)
function _fzf_discover_home --description "Alt+Shift+D: Discover directories from HOME with fd+fzf"
    set -l selected (
        fd --type d --hidden --no-ignore --max-depth 5 \
            --exclude .git --exclude node_modules \
            . "$HOME" | \
        fzf --height=50% \
            --preview="lsd -1 --color=always --icon=always {}" \
            --preview-window=right,40% \
            --prompt="🏠 Home > " \
            --header="Searching from: $HOME (max depth 5, all dirs)"
    )

    if test -n "$selected"
        cd "$selected"
        commandline -f repaint
    end
end
