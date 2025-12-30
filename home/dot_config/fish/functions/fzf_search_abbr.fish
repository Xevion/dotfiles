function fzf_search_abbr --description "Search Fish abbreviations, aliases, and functions with fzf"
    # Use the Bun script to collect items
    # Output format: name\texpansion\ttype\tdisplay
    set -l result (fzf-abbr-search.ts | fzf \
        --ansi \
        --height=50% \
        --reverse \
        --delimiter='\t' \
        --with-nth=4 \
        --nth=1,2 \
        --prompt='Aliases/Abbrs > ' \
        --preview='echo {2}' \
        --preview-window=up:3:wrap \
        --expect='tab' \
        --header='Enter: insert name | Tab: insert expansion')
    
    # Handle cancellation - just repaint and return
    if test $status -ne 0 -o -z "$result"
        commandline -f repaint
        return
    end
    
    # First line is the key pressed, second line is the selected item
    set -l lines (string split \n $result)
    set -l key $lines[1]
    set -l selected $lines[2]
    
    if test -n "$selected"
        # Split by tab to get fields
        set -l fields (string split \t $selected)
        
        if test "$key" = "tab"
            # Insert expansion (field 2)
            commandline -i $fields[2]
        else
            # Insert name (field 1)
            commandline -i $fields[1]
        end
    end
    
    commandline -f repaint
end


