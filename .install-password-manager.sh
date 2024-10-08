#!/bin/sh

type rbw >/dev/null 2>&1 && exit

install_cargo_binstall() {
    # Test if cargo binstall is installed
    cargo binstall
    if [ $? -eq 0 ]; then
        return
    fi
    curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash

    # Test again
    cargo binstall
    if [ $? -ne 0 ]; then
        echo "Failed to install cargo binstall"
        exit 1
    fi
}

install_rbw() {
    # Test if rbw is installed
    command -v rbw >/dev/null 2>&1 && return
    cargo binstall rbw

    # Test again
    command -v rbw >/dev/null 2>&1
}

install_cargo_binstall
install_rbw
