#!/bin/bash

work_dir=$PWD
install_dir=$HOME/.Take-a-Break

mkdir -p $install_dir
cp -ap $PWD/* $install_dir

read -p "What is your Favorite Beverage ? [Coffee/Tea]" bev
d_bev=`echo "${bev,,}"`
if [ $d_bev != coffee ] && [ $d_bev != tea ]
then
	echo 'Please enter either coffee or tea'
	sleep 2s
	exit
else
	cp -ap $install_dir/assets/${d_bev}-ascii.txt $install_dir/assets/ascii.txt
fi

read -p "Break Interval [in HH:MM e.g. 01:30 or 03:00]" int
d_int=`echo "$int" | awk -F: '{print $1*60 + $2}'`
sed -i "s|@Interval@|$d_int|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
sed -i "s|@Interval@|$d_int|g" $install_dir/scripts/short-break/systemd/take-a-short-break.timer

read -p "What is your breakfast time ? [either in 12/24 hour format e.g. 09:00 AM or 09:00]" b_time
tmp=`echo "${b_time^^}"`
b_time=$tmp
d_bf_time=`date -d "$b_time" +%H:%M`

before_d_bf_time=`date -d "$d_bf_time 1 hour ago" +%H%M`
after_d_bf_time=`date -d "$d_bf_time 1hour" +%H%M`
d_bf_hour=`echo $d_bf_time | awk -F":" '{print $1}'`

sed -i "s|@BeforeBreakfast@|$before_d_bf_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
sed -i "s|@AfterBreakfast@|$after_d_bf_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
sed -i "s|@Breakfast@|$d_bf_time|g" $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer
sed -i "s|@BreakfastHour@|$d_bf_hour|g" $install_dir/scripts/meal-break/take-a-meal-break.sh

read -p "What is your lunch time ? [either in 12/24 hour format e.g. 01:00 PM or 13:00]" l_time
tmp=`echo "${l_time^^}"`
l_time=$tmp
d_l_time=`date -d "$l_time" +%H:%M`
d_l_hour=`echo $d_l_time | awk -F":" '{print $1}'`

before_d_l_time=`date -d "$d_l_time 1 hour ago" +%H%M`
after_d_l_time=`date -d "$d_l_time 1 hour" +%H%M`
sed -i "s|@BeforeLunch@|$before_d_l_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
sed -i "s|@AfterLunch@|$after_d_l_time|g" $install_dir/scripts/short-break/systemd/take-a-short-break.service
sed -i "s|@Lunch@|$d_l_time|g" $install_dir/scripts/meal-break/systemd/take-a-meal-break.timer
sed -i "s|@LunchHour@|$d_l_hour|g" $install_dir/scripts/meal-break/take-a-meal-break.sh

read -p "What is your dinner time ? [either in 12/24 hour format e.g. 08:00 PM or 20:00]" d_time
tmp=`echo "${d_time^^}"`
d_time=$tmp
d_d_time=`date -d "$d_time" +%H:%M`

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
