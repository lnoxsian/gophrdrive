# fish completion for gophrdrv

# Disable file completion by default
complete -c gophrdrv -f

# -root / --root
complete -c gophrdrv -o root -l root -r -a "(__fish_complete_directories)" -d "Filesystem root directory"

# -port / --port
complete -c gophrdrv -o port -l port -r -d "Port to listen on"

# -host / --host
complete -c gophrdrv -o host -l host -r -d "Host to bind to"

# -read-timeout / --read-timeout
complete -c gophrdrv -o read-timeout -l read-timeout -r -d "Read timeout duration"

# -write-timeout / --write-timeout
complete -c gophrdrv -o write-timeout -l write-timeout -r -d "Write timeout duration"

# -max-upload / --max-upload
complete -c gophrdrv -o max-upload -l max-upload -r -d "Maximum upload size (e.g. 100MB, 1GB)"

# -private / --private
complete -c gophrdrv -o private -l private -d "Enable private mode with password protection"

# -r
complete -c gophrdrv -s r -d "Generate a random 6-digit password for private mode"

# -qr / --qr
complete -c gophrdrv -o qr -l qr -d "Show QR code for the server URL in the terminal"

# -version / --version
complete -c gophrdrv -o version -l version -d "Print version and exit"

# -v
complete -c gophrdrv -s v -d "Print version and exit"
