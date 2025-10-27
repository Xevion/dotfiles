# Lazy-load mise on first use
function mise --wraps mise
    # Initialize mise only once
    if not set -q __mise_initialized
        set -g __mise_initialized 1
        command mise activate fish | source
    end

    # Execute the actual mise command
    command mise $argv
end
