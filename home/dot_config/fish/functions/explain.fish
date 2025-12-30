function explain --description "Explain a command using GitHub Copilot"
    argparse --name=explain \
        'd/debug' \
        'h/help' \
        'hostname=' \
        -- $argv
    or return 1

    if set -q _flag_help
        echo "Wrapper around \`copilot\` to explain a command."
        echo
        echo "USAGE"
        echo "  explain [flags] <command>"
        echo
        echo "FLAGS"
        echo "  -d, --debug            Enable debugging"
        echo "  -h, --help             Display help usage"
        echo "      --hostname HOST    GitHub host for authentication"
        echo
        echo "EXAMPLES"
        echo "  explain 'du -sh | sort -h'"
        echo "  explain 'git log --oneline --graph --decorate --all'"
        echo "  explain 'bfg --strip-blobs-bigger-than 50M'"
        return 0
    end

    copilot -p "explain: $argv" --allow-all-tools
end
