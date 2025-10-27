# Lazy-load pyenv on first use
function pyenv --wraps pyenv
    # Initialize pyenv only once
    if not set -q __pyenv_initialized
        set -g __pyenv_initialized 1

        # Add to PATH if not already there
        if test -d $PYENV_ROOT/bin; and not contains $PYENV_ROOT/bin $PATH
            set -gx PATH $PYENV_ROOT/bin $PATH
        end

        # Run pyenv init
        command pyenv init - fish | source

        # pyenv-virtualenv if available
        if command -q pyenv-virtualenv-init
            command pyenv virtualenv-init - fish | source
        end
    end

    # Execute the actual pyenv command
    command pyenv $argv
end
