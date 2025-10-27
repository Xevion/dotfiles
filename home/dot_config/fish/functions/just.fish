# Lazy-load just completions on first use
function just --wraps just
    # Initialize just completions only once
    if not set -q __just_initialized
        set -g __just_initialized 1

        # Generate and source completions
        if command -q just
            command just --completions fish | source
        else
            echo "just is not installed" >&2
            return 1
        end
    end

    # Execute the actual just command
    command just $argv
end
