function wtf --description "Create feature branch worktree"
    if test (count $argv) -lt 1
        echo "Usage: wtf <feature-name> [base-ref]" >&2
        return 1
    end
    
    set -l name $argv[1]
    set -l base $argv[2]
    
    wtb "feature/$name" $base
end
