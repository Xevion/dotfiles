function suggest --description "Suggest a command using GitHub Copilot"
    argparse --name=suggest \
        'd/debug' \
        'h/help' \
        'hostname=' \
        't/target=' \
        -- $argv
    or return 1

    if set -q _flag_help
        echo "Wrapper around \`copilot\` to suggest a command."
        echo "Places suggested command on the command line for review/editing."
        echo
        echo "USAGE"
        echo "  suggest [flags] <prompt>"
        echo
        echo "FLAGS"
        echo "  -d, --debug            Enable debugging"
        echo "  -h, --help             Display help usage"
        echo "      --hostname HOST    GitHub host for authentication"
        echo "  -t, --target TARGET    Target: shell (default), gh, git"
        echo
        echo "EXAMPLES"
        echo "  suggest"
        echo "  suggest -t git 'Undo the most recent local commits'"
        echo "  suggest -t gh 'Create pull request'"
        echo "  suggest 'Convert MOV to animated PNG'"
        return 0
    end

    copilot -p "suggest: $argv" --allow-all-tools
end
