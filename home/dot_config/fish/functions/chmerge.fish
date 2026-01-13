function chmerge --description "Interactively merge chezmoi conflicts using fzf"
    # Get files with merge conflicts (MM = modified in both source and target)
    set -l conflict_files (chezmoi status | grep '^MM ' | awk '{print $2}')

    if test (count $conflict_files) -eq 0
        echo "No merge conflicts found."
        return 0
    end

    # Use fzf for selection with multi-select enabled
    set -l selected (printf '%s\n' $conflict_files | fzf \
        --multi \
        --preview 'chezmoi diff {}' \
        --preview-window 'right:70%:wrap' \
        --header 'Select files to merge (Tab for multi-select, Enter to confirm)' \
        --bind 'ctrl-/:toggle-preview')

    if test (count $selected) -eq 0
        echo "No files selected."
        return 0
    end

    # Convert to absolute target paths and merge
    for file in $selected
        # Expand ~ to home directory for chezmoi merge
        set -l target_path (string replace -r '^~' $HOME $file)
        echo "Merging: $file"
        chezmoi merge $target_path
    end
end
