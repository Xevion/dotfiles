#!/bin/bash
# set -u

ROOT="$1"
SHELLCHECK_OPTIONS="--color=never --format=gcc"

# Function to invoke shellcheck or chezmoi execute-template based on file type
invoke_checker() {
    filepath="$1"

    # If the file is a .tmpl file, use chezmoi execute-template
    if [[ $filepath == *.tmpl ]]; then
        RENDERED_TEMPLATE=$(chezmoi execute-template <$filepath)
        if [ $? -eq 0 ]; then
            echo "$RENDERED_TEMPLATE" | shellcheck - $SHELLCHECK_OPTIONS | sed "s|^-|$filepath|"
        fi
    else
        # Otherwise, use shellcheck directly
        shellcheck "$filepath" $SHELLCHECK_OPTIONS
    fi
}

echo "inotify watcher started"

# Run an initial scan of all shell scripts
while IFS= read -rd $'\0' file; do
    invoke_checker "$file"
done < <(find "$ROOT" \( -name "*.sh" -o -name "*.sh.tmpl" \) -type f -print0)

echo "inotifywait invoking"
inotifywait -mr --quiet --timefmt '%d/%m/%y %H:%M:%S' --format '%T %w %f' -e modify $1 |
    while read -r date time dir file; do
        absolute_path=${dir}${file}

        # Check if the changed file ends with .sh or .sh.tmpl
        if [[ $absolute_path == *.sh || $absolute_path == *.sh.tmpl ]]; then
            invoke_checker $absolute_path
        fi

    done

echo "inotify watcher stopped"
