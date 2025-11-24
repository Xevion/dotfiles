function sdk --description 'SDKMAN wrapper for Fish shell'
    # Run sdk commands in a Bash subshell
    bash -c "source '$HOME/.sdkman/bin/sdkman-init.sh' && sdk $argv"
end
