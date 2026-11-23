#!/bin/bash

work_dir=$PWD
install_dir=$HOME/.To-Do
systemd_flg=`ls -l $HOME/.config/systemd/user/to-do.{service,timer} 2>/dev/null |wc -l` 

dependencies() {

}

ask_time() {
	local ans out
	while true
	do
		read -p "$1" ans
		if [ -n "$ans" ] && out=`date -d "${ans^^}" +%H:%M 2>/dev/null`
		then
			echo "$out"
			return
		fi
		echo "Could not read that time. Try e.g. 09:00 AM or 13:00." >&2
	done
}

setup() {

	read -p "What is your Favorite Beverage ? [Coffee/Tea]" bev
	d_bev=`echo "${bev,,}"`
	
	if [[ -z "$d_bev" ]]
	then
		echo 'Please enter either coffee or tea'
		sleep 2s
		exit 1
	else
		if [ "$d_bev" != "coffee" ] && [ "$d_bev" != "tea" ]
		then
        		echo 'Please enter either coffee or tea'
        		sleep 2s
        		exit 1
		else
        		cp -ap $install_dir/assets/${d_bev}-ascii.txt $install_dir/assets/ascii.txt
		fi
	fi

	while true
	do
		read -p "Break Interval [in Xhrs:Ymins e.g. 01:30 or 03:00] " int
		if [[ $int =~ ^[0-9]{1,2}:[0-5][0-9]$ ]]
		then
			d_int=`echo "$int" | awk -F: '{print $1*60 + $2}'`
			[ "$d_int" -gt 0 ] && break
		fi
		echo 'Please use HH:MM, e.g. 01:30 (must be more than 00:00).'
	done

	sed -i "s|@Interval@|$d_int|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@Interval@|$d_int|g" $install_dir/scripts/short-break/systemd/take-a-short-break.timer

	d_bf_time=`ask_time "What is your breakfast time ? [either in 12/24 hour format e.g. 09:00 AM or 09:00] "`
	before_d_bf_time=`date -d "$d_bf_time 1 hour ago" +%H%M`
	after_d_bf_time=`date -d "$d_bf_time 1hour" +%H%M`
	d_bf_hour=`echo $d_bf_time | awk -F":" '{print $1}'`

	sed -i "s|@BeforeBreakfast@|$before_d_bf_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@AfterBreakfast@|$after_d_bf_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@Breakfast@|$d_bf_time|g" $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer
	sed -i "s|@BreakfastHour@|$d_bf_hour|g" $install_dir/scripts/meal-break/take-a-meal-break.sh

	d_l_time=`ask_time "What is your lunch time ? [either in 12/24 hour format e.g. 01:00 PM or 13:00] "`
	d_l_hour=`echo $d_l_time | awk -F":" '{print $1}'`

	before_d_l_time=`date -d "$d_l_time 1 hour ago" +%H%M`
	after_d_l_time=`date -d "$d_l_time 1 hour" +%H%M`
	sed -i "s|@BeforeLunch@|$before_d_l_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@AfterLunch@|$after_d_l_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@Lunch@|$d_l_time|g" $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer
	sed -i "s|@LunchHour@|$d_l_hour|g" $install_dir/scripts/meal-break/take-a-meal-break.sh

	d_d_time=`ask_time "What is your dinner time ? [either in 12/24 hour format e.g. 08:00 PM or 20:00] "`
	before_d_d_time=`date -d "$d_d_time 1 hour ago" +%H%M`
	after_d_d_time=`date -d "$d_d_time 1hour" +%H%M`
	d_d_hour=`echo $d_d_time | awk -F":" '{print $1}'`

	sed -i "s|@BeforeDinner@|$before_d_d_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@AfterDinner@|$after_d_d_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
	sed -i "s|@Dinner@|$d_d_time|g" $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer
	sed -i "s|@DinnerHour@|$d_d_hour|g" $install_dir/scripts/meal-break/take-a-meal-break.sh

	mkdir -p $HOME/.config/systemd/user
	cp -ap $install_dir/scripts/short-break/systemd/take-a-short-break.service $HOME/.config/systemd/user/
	cp -ap $install_dir/scripts/short-break/systemd/take-a-short-break.timer $HOME/.config/systemd/user/
	cp -ap $install_dir/scripts/meal-break/systemd/take-a-meal-break.service $HOME/.config/systemd/user/
	cp -ap $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer $HOME/.config/systemd/user/

	systemctl --user daemon-reload
	systemctl --user enable --now take-a-short-break.timer take-a-meal-break.timer

}

uninstall() {
		echo 'Stopping, disabling and removing the enable symlink'
		systemctl --user disable --now take-a-short-break.timer take-a-meal-break.timer
		echo 'Deleting the installed and systemd files'
		rm ~/.config/systemd/user/take-a-short-break.{service,timer}
		rm ~/.config/systemd/user/take-a-meal-break.{service,timer}
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

			echo 'systemctl --user disable --now take-a-short-break.timer take-a-meal-break.timer'
			echo 'rm ~/.config/systemd/user/take-a-short-break.{service,timer}'
			echo 'rm ~/.config/systemd/user/take-a-meal-break.{service,timer}'
			echo 'rm -rf ~/.Take-a-Break'
			echo 'systemctl --user daemon-reload'
			echo 'systemctl --user reset-failed'
		fi
}

restore_files() {
	echo 'Restoring Files..'
	rm $install_dir/assets/ascii.txt
	cp -ap $install_dir/scripts/short-break/systemd/take-a-short-break.service.org $install_dir/scripts/short-break/systemd/take-a-short-break.service
	cp -ap $install_dir/scripts/short-break/systemd/take-a-short-break.timer.org $install_dir/scripts/short-break/systemd/take-a-short-break.timer
        cp -ap $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer.org $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer
	cp -ap $install_dir/scripts/meal-break/systemd/take-a-meal-break.service.org $install_dir/scripts/meal-break/systemd/take-a-meal-break.service
	cp -ap $install_dir/scripts/meal-break/take-a-meal-break.sh.org $install_dir/scripts/meal-break/take-a-meal-break.sh
}

if [ -d $install_dir ] && [ $systemd_flg != 0 ]
then
	echo 'Take-a-Break already exists. Skipping installation...'
	read -p "Do you want to modify or uninstall ? [modify/m uninstall/u]" response

	tmp=`echo "${response,,}"`
	response=$tmp
	
	if [ "$response" == "uninstall" ] || [ "$response" == "u" ]
	then
		uninstall
	elif [ "$response" == "modify" ] || [ "$response" == "m" ]
	then
		echo 'Stopping, disabling and removing the enable symlink'
		systemctl --user disable --now take-a-short-break.timer take-a-meal-break.timer

		restore_files
		setup
	else
		echo 'Please input "uninstall or u" or "modify or m".'
		sleep 2s
		exit 1
	fi
	
else
	mkdir -p $install_dir
	cp -ap $PWD/* $install_dir
	setup
fi
