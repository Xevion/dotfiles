# Lazy-load thefuck on first use
function fuck --description "Correct your previous console command"
    # Initialize thefuck only once
    if not set -q __thefuck_initialized
        set -g __thefuck_initialized 1

        # Run thefuck alias generation
        if command -q thefuck
            thefuck --alias | source
        else
            echo "thefuck is not installed" >&2
            return 1
        end
    end

    # Execute the fuck command (which thefuck creates)
    # After initialization, the actual fuck function will be defined
    fuck $argv
end
