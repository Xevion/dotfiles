# Interactive zoxide with fzf (initialization handled in config.fish)
function zi --description "zoxide interactive smart cd"
    # Configure fzf options for zoxide interactive mode
    set -gx _ZO_FZF_OPTS '
  --preview="lsd -1 --color=always --icon=always {2..}"
  --preview-window=down,30%
'

    if command -q zoxide
        __zoxide_zi $argv
    else
        echo "zoxide is not installed" >&2
        return 1
    end
end
