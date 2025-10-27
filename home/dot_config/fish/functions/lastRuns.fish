function lastRuns --description "Show last N GitHub Actions runs"
    set -l runs 10
    if test (count $argv) -gt 0
        set runs $argv[1]
    end
    gh run list -L $runs --json name,url | jq -c '.[] | [.name, .url] | join(" ")' -r
end
