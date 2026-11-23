#!/bin/bash

install_dir=@INSTALL_DIR@

case "${1:-}" in
    --cli)
        bash "$install_dir/scripts/to-do_cli.sh"
        ;;
    --tui)
        #bash $install_dir/scripts/to-do-tui.sh
        echo 'TUI is in development. So, use CLI --cli'
	;;
    -h|--help)
	printf -- "--cli for Command Line Interface\n\n--tui for Terminal User Interface\n"
	;;
    *)
        #bash $install_dir/scripts/to-do-tui.sh
        echo 'TUI is in devlopment. So, use CLI --cli'
        ;;
esac
