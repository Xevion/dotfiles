#!/bin/sh

type rbw >/dev/null 2>&1 && exit

apt_update() {
    # Only run apt update once
    if [ -f /tmp/.apt-updated ]; then
        return
    fi

    sudo apt update

    if [ $? -ne 0 ]; then
        echo "Failed to update apt"
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

install_cargo_binstall() {
    # Test if cargo binstall is installed
    cargo binstall --help
    if [ $? -eq 0 ]; then
        return
    fi
    curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash

    # Test again
    cargo binstall --help
    if [ $? -ne 0 ]; then
        echo "Failed to install cargo binstall"
        exit 1
    fi
}

install_rbw() {
    # Test if rbw is installed
    command -v rbw >/dev/null 2>&1 && return
    cargo binstall rbw --no-confirm

    # Test again
    command -v rbw >/dev/null 2>&1 && return
    echo "Failed to install rbw"
    exit 1
}

install_age
install_cargo_binstall
install_rbw
