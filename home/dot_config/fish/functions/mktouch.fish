function mktouch --description "Touch a file while creating parent directories"
    mkdir -p (dirname $argv[1])
    touch $argv[1]
end
