# Override fzf.fish default bindings before it loads
# This file loads before fzf.fish (alphabetically) to prevent CTRL+R binding
# We want Atuin to handle CTRL+R instead of fzf

# Store the original fzf_configure_bindings function
functions -c fzf_configure_bindings _fzf_configure_bindings_original 2>/dev/null

# Override with our version that disables history binding
function fzf_configure_bindings --wraps=_fzf_configure_bindings_original
    # If called without arguments (from fzf.fish), add --history= to disable CTRL+R
    if test (count $argv) -eq 0
        _fzf_configure_bindings_original --history=
    else
        # Otherwise pass through arguments as-is
        _fzf_configure_bindings_original $argv
    end
end
