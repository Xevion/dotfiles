function chcode --description "Edit chezmoi files in VS Code"
    set -gx EDITOR "code --wait"

    # Check if --watch is in arguments
    set -l has_watch false
    set -l has_file false

    for arg in $argv
        if test "$arg" = "--watch"
            set has_watch true
        else if not string match -q -- '-*' $arg
            set has_file true
        end
    end

    # If --watch is provided but no file, show error
    if test $has_watch = true -a $has_file = false
        echo "--watch requires a file to be provided, directories aren't supported with watch mode"
        return 1
    end

    # Execute chezmoi edit with all arguments
    chezmoi edit $argv
end
