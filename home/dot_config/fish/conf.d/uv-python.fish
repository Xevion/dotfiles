# Redirect pip/pip3 to uv to avoid Linuxbrew vs system Python confusion.
# uv resolves the right Python and isolates deps properly.

function pip --wraps pip --description "Redirect pip to uv pip"
    if test (count $argv) -ge 1 -a "$argv[1]" = "install"
        echo "tip: use 'uv tool install <pkg>' for CLI tools or 'uv pip install' for venv-scoped installs" >&2
        echo "     running: uv pip $argv" >&2
    end
    uv pip $argv
end

function pip3 --wraps pip3 --description "Redirect pip3 to uv pip"
    pip $argv
end
