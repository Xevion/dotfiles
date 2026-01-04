# Lazy-load gh completions on first use
function gh --wraps gh
    # Initialize gh completions only once
    if not set -q __gh_initialized
        set -g __gh_initialized 1

        # Generate and source completions
        if command -q gh
            command gh completion -s fish | source
        else
            echo "gh is not installed" >&2
            return 1
        end
    end

    # Execute the actual gh command
    command gh $argv
end
