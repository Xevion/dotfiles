function chfix --description "chmod +x the last command (if it's a file)"
    # Get the last command from Fish history
    set -l last_command (history --max 1 | string trim)

    if test -f "$last_command"
        chmod +x "$last_command"
        echo "Made executable: $last_command"
    else
        echo "Error: $last_command is not a valid file"
        return 1
    end
end
