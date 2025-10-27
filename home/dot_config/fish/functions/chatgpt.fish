# Lazy-load chatgpt completions on first use
function chatgpt --wraps chatgpt
    # Initialize chatgpt completions only once
    if not set -q __chatgpt_initialized
        set -g __chatgpt_initialized 1

        # Generate and source completions
        if command -q chatgpt
            command chatgpt --set-completions fish | source
        else
            echo "chatgpt is not installed" >&2
            return 1
        end
    end

    # Execute the actual chatgpt command
    command chatgpt $argv
end
