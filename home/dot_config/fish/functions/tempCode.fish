function tempCode --description "Create a temporary file and open in VS Code"
    if test (count $argv) -eq 0
        echo "Must provide filetype argument (ex: py, .xml, html)"
        return 1
    end

    # Remove preceding dot, then re-add to support both '.py' and 'py' as arguments
    set -l extension (string replace -r '^\.' '' $argv[1])
    set -l temp_file (mktemp /tmp/XXXXXXXXXXXX_(uuidgen).$extension)

    echo "Temporary $argv[1] file created at $temp_file"
    code --file-uri "file://$temp_file"
end
