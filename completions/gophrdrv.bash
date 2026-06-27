# bash completion for gophrdrv

_gophrdrv_completions()
{
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    
    # List of all options/flags (both single and double dash versions)
    opts="--root -root --port -port --host -host --read-timeout -read-timeout --write-timeout -write-timeout --max-upload -max-upload --private -private -r --qr -qr --version -version -v"

    case "$prev" in
        -root|--root)
            # Complete directories only
            local IFS=$'\n'
            COMPREPLY=( $(compgen -d -- "$cur") )
            return 0
            ;;
        -port|--port|-host|--host|-read-timeout|--read-timeout|-write-timeout|--write-timeout|-max-upload|--max-upload)
            # These expect specific non-path inputs; do not complete
            return 0
            ;;
    esac

    # Complete options if the current token starts with '-'
    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi
}

complete -F _gophrdrv_completions gophrdrv
