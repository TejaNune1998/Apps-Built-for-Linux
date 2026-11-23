#!/bin/bash

work_dir=$PWD
install_dir=$HOME/.To-Do
systemd_flg=`ls -l $HOME/.config/systemd/user/to-do.{service,timer} 2>/dev/null |wc -l` 

dependencies() {

}


setup() {

}

uninstall() {
		echo 'Stopping, disabling and removing the enable symlink'
		systemctl --user disable --now to-do.timer to-do.timer
		echo 'Deleting the installed and systemd files'
		rm ~/.config/systemd/user/to-do.{service,timer}
		rm ~/.config/systemd/user/to-do.{service,timer}
		rm -rf ~/.Take-a-Break

		echo 'Making systemd forget them'
		systemctl --user daemon-reload
		systemctl --user reset-failed

		echo 'Verifying uninstall..'
		flg=`systemctl --user list-timers | grep -i break | wc -l`

		if [[ $flg == "0" ]]
		then
			echo 'Successfully uninstalled Take-a-Break'
		else
			echo 'Uninstalled failed. Please do below manually.'

			echo 'systemctl --user disable --now to-do.timer to-do.timer'
			echo 'rm ~/.config/systemd/user/to-do.{service,timer}'
			echo 'rm -rf ~/.Take-a-Break'
			echo 'systemctl --user daemon-reload'
			echo 'systemctl --user reset-failed'
		fi
}

restore_files() {
	echo 'Restoring Files..'
}

if [ -d $install_dir ] && [ $systemd_flg != 0 ]
then
else
fi
