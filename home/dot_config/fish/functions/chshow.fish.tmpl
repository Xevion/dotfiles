function chshow --description "Browse chezmoi managed files with fzf preview"
    # Get data from script first (before fzf takes over TTY)
    set -l data (fzf-chezmoi-show.ts)
    if test $status -ne 0 -o -z "$data"
        return 1
    end

    # Run fzf separately so it gets proper TTY access
    set -l result (printf '%s\n' $data | fzf \
        --ansi \
        --height=50% \
        --reverse \
        --delimiter='\t' \
        --with-nth=4 \
        --nth=1,2 \
        --prompt='Chezmoi Files > ' \
        --preview='chezmoi cat {1} 2>/dev/null | bat -pp --color=always --file-name={1} 2>/dev/null || echo "Preview unavailable"' \
        --preview-window=right:60%:wrap \
        --expect='ctrl-e' \
        --header='Enter: view | Ctrl+E: edit source')

    if test $status -ne 0 -o -z "$result"
        return
    end

    # Parse result: first line is key, second is selected item
    set -l lines (string split \n $result)
    set -l key $lines[1]
    set -l selected $lines[2]

    if test -n "$selected"
        set -l target (string split \t $selected)[1]

        if test "$key" = "ctrl-e"
            chezmoi edit ~/$target
        else
            echo "─── Rendered: $target ───"
            chezmoi cat $target 2>/dev/null || echo "Cannot render file (may be binary or encrypted)"
        end
    end
end
