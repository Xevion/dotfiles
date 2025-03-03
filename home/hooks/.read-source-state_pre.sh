#!/bin/bash
set -eu

# Flag variables
LATE_FAIL_EXIT=false      # Set to true if any installation fails, but only raise an exit at the end
DOPPLER_UNAVAILABLE=false # Set to true if Doppler is not available

apt_update() {
    # Only run apt update once
    if [ -f /tmp/.apt-updated ]; then
        return
    fi

    sudo apt update >/dev/null

    if [ $? -ne 0 ]; then
        echo "chezmoi: Critical issue - failed to update apt!"
        exit 1
    else
        touch /tmp/.apt-updated
    fi
}

install_age() {
    # Test if age is installed
    command -v age >/dev/null 2>&1 && return

    # Install age
    apt_update
    sudo apt install age
}

# install_cargo_binstall() {
#     # Test if cargo binstall is installed
#     cargo binstall --help >/dev/null 2>&1
#     if [ $? -eq 0 ]; then
#         return
#     fi
#     curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash

#     # Test again
#     cargo binstall --help
#     if [ $? -ne 0 ]; then
#         echo "Failed to install cargo binstall"
#         exit 1
#     fi
# }

install_inotify() {
    if ! dpkg -l inotify-tools >/dev/null 2>&1; then
        echo "Installing inotify-tools"
        apt_update
        sudo apt install inotify-tools
    fi
}

install_doppler() {
    if ! command -v doppler >/dev/null 2>&1; then
        echo "You need to install the Doppler CLI manually (for security purposes)."
        echo "https://docs.doppler.com/docs/cli#installation"

        DOPPLER_UNAVAILABLE=true
        INSTALLATION_FAILED=true
    fi
}

require_doppler_login() {
    doppler me >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo "You need to login to Doppler."
        echo "https://docs.doppler.com/docs/cli#logging-in"
    fi
}

install_deno() {
    if ! command -v deno >/dev/null 2>&1; then
        echo "Installing Deno"
        curl -fsSL https://deno.land/install.sh | sh
    fi
}

install_inotify
install_age
# install_cargo_binstall
if [ $DOPPLER_UNAVAILABLE = false ]; then
    install_doppler
fi