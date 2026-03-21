function chai --description "Interactive chezmoi apply with fzf diff preview"
    # Get status data first (before fzf takes over TTY)
    # Script outputs to stderr for "no changes" message, stdout for data
    set -l data (fzf-chezmoi-apply.ts 2>/dev/null)
    set -l script_status $status

    # Handle error or no changes case
    if test $script_status -ne 0
        return 1
    end

    if test -z "$data"
        echo "No changes to apply"
        return 0
    end

    # Run fzf separately so it gets proper TTY access
    set -l selected (printf '%s\n' $data | fzf \
        --ansi \
        --height=50% \
        --reverse \
        --delimiter='\t' \
        --with-nth=4 \
        --nth=1 \
        --prompt='Apply Changes > ' \
        --preview='chezmoi diff ~/{1} 2>/dev/null | bat -pp --color=always --language=diff' \
        --preview-window=right:60%:wrap \
        --multi \
        --bind='ctrl-a:toggle-all' \
        --marker='* ' \
        --pointer='>' \
        --header='Tab: toggle | Ctrl+A: all | Enter: apply')

    if test $status -ne 0 -o -z "$selected"
        echo "Cancelled"
        return 1
    end

    # Apply selected files
    set -l targets
    for line in $selected
        set -l target (string split \t $line)[1]
        set -a targets $target
    end

    set -l count (count $targets)
    echo "Applying $count file(s)..."

    for target in $targets
        echo "  $target"
        chezmoi apply ~/$target
    end

    echo "Applied $count file(s)"
end
