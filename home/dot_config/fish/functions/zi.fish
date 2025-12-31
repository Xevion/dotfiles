# Lazy-load zoxide on first use of zi command
function zi --description "zoxide interactive smart cd"
    # Initialize zoxide only once (shared flag with z.fish)
    if not set -q __zoxide_initialized
        set -g __zoxide_initialized 1

        # Configure fzf options for zoxide interactive mode
        set -gx _ZO_FZF_OPTS '
  --preview="lsd -1 --color=always --icon=always {2..}"
  --preview-window=down,30%
'

        # Run zoxide init
        if command -q zoxide
            zoxide init fish | source
        else
            echo "zoxide is not installed" >&2
            return 1
        end
    end

    # Execute the actual zi command (defined by zoxide init)
    __zoxide_zi $argv
end
