function wth --description "Create hotfix branch worktree"
    if test (count $argv) -lt 1
        echo "Usage: wth <hotfix-name> [base-ref]" >&2
        return 1
    end
    
    set -l name $argv[1]
    set -l base $argv[2]
    
    wtb "hotfix/$name" $base
end
