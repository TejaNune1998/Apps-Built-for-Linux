#!/bin/bash

work_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
install_dir=$HOME/.To-Do

install_dependencies() {
    local PKGS=(bash gum coreutils findutils grep sed gawk systemd)
    local MISSING=()

    if command -v apt >/dev/null 2>&1; then
        PKGS+=(golang libnotify-bin)
        for pkg in "${PKGS[@]}"; do
            dpkg-query -W -f='${Status}' "$pkg" 2>/dev/null | grep -q "install ok installed" || MISSING+=("$pkg")
        done
        if ((${#MISSING[@]} > 0)); then
            sudo apt update && sudo apt install -y "${MISSING[@]}"
        fi

    elif command -v dnf >/dev/null 2>&1; then
        PKGS+=(golang libnotify)
        for pkg in "${PKGS[@]}"; do
            rpm -q "$pkg" >/dev/null 2>&1 || MISSING+=("$pkg")
        done
        if ((${#MISSING[@]} > 0)); then
            sudo dnf install -y "${MISSING[@]}"
        fi

    elif command -v pacman >/dev/null 2>&1; then
        PKGS+=(go libnotify)
        for pkg in "${PKGS[@]}"; do
            pacman -Q "$pkg" >/dev/null 2>&1 || MISSING+=("$pkg")
        done
        if ((${#MISSING[@]} > 0)); then
	    sudo pacman -S --needed --noconfirm "${MISSING[@]}"
        fi

    elif command -v zypper >/dev/null 2>&1; then
        PKGS+=(go libnotify-tools)
        for pkg in "${PKGS[@]}"; do
            rpm -q "$pkg" >/dev/null 2>&1 || MISSING+=("$pkg")
        done
        if ((${#MISSING[@]} > 0)); then
            sudo zypper install -y "${MISSING[@]}"
        fi

    elif command -v apk >/dev/null 2>&1; then
        PKGS+=(go libnotify-tools)
        for pkg in "${PKGS[@]}"; do
            apk info -e "$pkg" >/dev/null 2>&1 || MISSING+=("$pkg")
        done
        if ((${#MISSING[@]} > 0)); then
            apk add --no-cache "${MISSING[@]}"
        fi

    else
        echo "Unsupported package manager."
        echo "Install manually: ${PKGS[*]} go libnotify"
        return 1
    fi

    if ((${#MISSING[@]} == 0)); then
        echo "All dependencies are already installed."
    fi
}

setup() {
	mkdir -p "$install_dir"
	cp -ap "$work_dir/." "$install_dir"
	data_dir=$(gum input --placeholder "To-Do Task Directory Path")
	data_dir="$data_dir/To-Do"
	mkdir -p "$data_dir"
	
	sed -i "s|@DATA_DIR@|$data_dir|g" "$install_dir/scripts/to-do_cli.sh"
	sed -i "s|@DATA_DIR@|$data_dir|g" "$install_dir/scripts/reminder.sh"
	sed -i "s|@INSTALL_DIR@|$install_dir|g" "$install_dir/scripts/systemd/to-do.service"
	sed -i "s|@INSTALL_DIR@|$install_dir|g" "$install_dir/scripts/to-do.sh"
	cp -ap "$install_dir/scripts/systemd/to-do.service" "$HOME/.config/systemd/user/"
	cp -ap "$install_dir/scripts/systemd/to-do.timer" "$HOME/.config/systemd/user/"

	systemctl --user daemon-reload
	systemctl --user enable --now to-do.timer
	systemctl --user is-enabled to-do.timer
	echo 'alias to-do="$HOME/.To-Do/scripts/to-do.sh"' >> ~/.bashrc
	source ~/.bashrc
}

delete_install_dir() {
	rm -rf $install_dir
}
delete_systemd() {
		echo 'Stopping, disabling and removing the enable symlink'
		systemctl --user disable --now to-do.timer to-do.timer
		echo 'Deleting the installed and systemd files'
		rm ~/.config/systemd/user/to-do.{service,timer}
		rm ~/.config/systemd/user/to-do.{service,timer}

		echo 'Making systemd forget them'
		systemctl --user daemon-reload
		systemctl --user reset-failed

		echo 'Verifying systemd files..'
		flg=`systemctl --user list-timers | grep -i break | wc -l`

		if [[ $flg != "0" ]]
		then
			echo 'Failed.....Please do below manually.'

			echo 'systemctl --user disable --now to-do.timer to-do.timer'
			echo 'rm ~/.config/systemd/user/to-do.{service,timer}'
			echo 'rm -rf ~/.To-Do'
			echo 'systemctl --user daemon-reload'
			echo 'systemctl --user reset-failed'
			sed -i '/\<to-do.sh\>/d' ~/.bashrc
			source ~/.bashrc
		fi
}

case "${1:-}" in
    --install)
        install_dependencies
	setup
        ;;
    --remove-systemctl)
        delete_install_dir
	delete_systemd
        ;;
    *)
        echo "Please execute this script as either below"
	echo 'To Install : ./setup --install'
	echo 'To Remove systemctl : ./setup --remove-systemctl'
	sleep 2s
        exit 1
        ;;
esac
