# Lazy-load zoxide on first use of z command
function z --description "zoxide smart cd"
    # Initialize zoxide only once
    if not set -q __zoxide_initialized
        set -g __zoxide_initialized 1

        # Run zoxide init
        if command -q zoxide
            zoxide init fish | source
        else
            echo "zoxide is not installed" >&2
            return 1
        end
    end

    # Execute the actual z command (defined by zoxide init)
    __zoxide_z $argv
end
