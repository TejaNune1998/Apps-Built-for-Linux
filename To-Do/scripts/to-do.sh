#!/bin/bash

install_dir=@INSTALL_DIR@

case "${1:-}" in
    --cli)
        bash "$install_dir/scripts/to-do_cli.sh"
        ;;
     --tui)
        "$install_dir/tui/to-do-tui" "$data_dir"
    -h|--help)
	printf -- "--cli for Command Line Interface\n\n--tui for Terminal User Interface\n"
	;;
    -v|--version)
	cat $install_dir/assets/version.txt
	;;
    *)
        "$install_dir/tui/to-do-tui" "$data_dir"
        ;;
esac
