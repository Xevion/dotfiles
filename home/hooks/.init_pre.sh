#!/bin/bash

# note: CHEZMOI_UPDATE will be set if this was invoked indirectly by 'chezmoi update --init'
set -eu

# While key.txt is managed by Chezmoi, it's required for encrypted operations and needed to bootstrap other operations.
if [ ! -f ~/key.txt ]; then
    rbw get "key.txt (age)" --field notes >>~/key.txt
    rbw get "key.txt (age)" --field password >>~/key.txt
    echo "key.txt bootstrapped"
fi
