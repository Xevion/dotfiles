#!/bin/bash
# set -u

ROOT="$1"
SHELLCHECK_OPTIONS="--color=never --format=gcc"

# Function to invoke shellcheck or chezmoi execute-template based on file type
invoke_checker() {
    filepath="$1"

    # If the file is a .tmpl file, use chezmoi execute-template
    if [[ $filepath == *.tmpl ]]; then
        # TODO: This still doesn't work, for some reason 'sed' just refuses to replace the 'stdin' placeholder with execute-template
        if ! RENDERED_TEMPLATE=$(cat $filepath | chezmoi execute-template | sed -E "s|stdin|$filepath|"); then
            # since stdin is used for this, the filepath appears as '-', and thus must be replaced
            echo "$RENDERED_TEMPLATE" | shellcheck - $SHELLCHECK_OPTIONS | sed "s|^-|$filepath|"
        else
            echo $filepath
        fi
    else
        # Otherwise, use shellcheck directly
        shellcheck "$filepath" $SHELLCHECK_OPTIONS
    fi
}

# chek that 'shellcheck' is available
if ! command -v shellcheck &> /dev/null; then
    echo "shellcheck could not be found"
    exit 1
fi

echo "initial shellcheck started"

# Run an initial scan of all shell scripts
while IFS= read -rd $'\0' file; do
    invoke_checker "$file"
done < <(find "$ROOT" \( -name "*.sh" -o -name "*.sh.tmpl" \) -type f -print0)

echo "inotifywait started"
inotifywait -mr --quiet --format ' %w %f' -e modify $1 |
    while read -r dir file; do
        absolute_path=${dir}${file}

        # Check if the changed file ends with .sh or .sh.tmpl
        if [[ $absolute_path == *.sh || $absolute_path == *.sh.tmpl ]]; then
            invoke_checker $absolute_path
        fi

    done

echo "inotify watcher stopped"
