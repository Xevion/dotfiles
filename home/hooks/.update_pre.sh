#!/bin/bash

# chezmoi update --init does not invoke the 'hooks.init.pre' hook, so we do it ourselves
if grep -q 'init' <<<$CHEZMOI_ARGS; then
    # CHEZMOI_UPDATE is just a hint in case we need to know if we're updating
    CHEZMOI_UPDATE=1 $(dirname $0)/.init_pre.ts
fi
