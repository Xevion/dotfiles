function glf --description "Git log find by commit message"
    git log --all --grep="$argv"
end
