function _wt_clone_ignored --description "Clone gitignored files from source to destination worktree"
    set -l src_path $argv[1]
    set -l dst_path $argv[2]
    
    # Find gitignored files (excluding .git and .worktrees)
    set -l ignored_files (git -C "$src_path" ls-files --others --ignored --exclude-standard 2>/dev/null | \
        grep -v '^\.git' | grep -v '^\.worktrees')
    
    if test -z "$ignored_files"
        return 0
    end

    echo ""
    echo "Found gitignored files to copy..."
    
    # Measure size with timeout
    set -l size_bytes 0
    set -l size_human "unknown"
    
    set -l size_output (timeout 10s du -sb "$src_path" --exclude='.git' --exclude='.worktrees' 2>/dev/null | head -n1)
    if test $status -eq 0 -a -n "$size_output"
        set size_bytes (echo "$size_output" | awk '{print $1}')
        set size_human (numfmt --to=iec --suffix=B "$size_bytes" 2>/dev/null; or echo "$size_bytes bytes")
    end

    # 100MB threshold
    set -l threshold (math "100 * 1024 * 1024")
    
    if test "$size_bytes" -gt "$threshold" -o "$size_human" = "unknown"
        echo "Gitignored files size: $size_human"
        read -P "Copy these files to new worktree? [y/N] " -n 1 confirm
        echo ""
        if not string match -qi 'y' "$confirm"
            echo "Skipping gitignored file copy"
            return 0
        end
    end

    echo "Copying gitignored files..."
    for file in $ignored_files
        set -l src "$src_path/$file"
        set -l dst "$dst_path/$file"
        if test -e "$src"
            mkdir -p (dirname "$dst")
            cp -r "$src" "$dst" 2>/dev/null
        end
    end
    
    echo "Done copying gitignored files"
end
