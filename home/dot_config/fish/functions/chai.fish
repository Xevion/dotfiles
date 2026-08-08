function chai --description "Interactive chezmoi apply with fzf diff preview"
    # Get status data first (before fzf takes over TTY).
    # Stderr is left attached so chezmoi's own errors reach the terminal.
    set -l data (fzf-chezmoi-apply.ts)
    set -l script_status $status

    if test $script_status -ne 0
        echo "chai: fzf-chezmoi-apply.ts failed (exit $script_status)" >&2
        return $script_status
    end

    if test -z "$data"
        echo "No changes to apply - target is in sync with source"
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
        --preview='chezmoi diff ~/{1} 2>&1 | bat -pp --color=always --language=diff' \
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

    set -l failed 0
    for target in $targets
        echo "  $target"
        if not chezmoi apply ~/$target
            set failed (math $failed + 1)
            echo "  ! failed: $target" >&2
        end
    end

    if test $failed -gt 0
        echo "Applied "(math $count - $failed)" file(s), $failed failed" >&2
        return 1
    end

    echo "Applied $count file(s)"
end
