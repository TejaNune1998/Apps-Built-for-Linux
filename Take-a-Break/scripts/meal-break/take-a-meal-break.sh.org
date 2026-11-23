#!/bin/bash

scripts_dir=$HOME/.Take-a-Break/scripts/meal-break

# Snooze re-launch: take-a-meal-break.sh <Meal> <snoozes used so far>
if [[ -n ${1:-} ]]
then
	hyprctl dispatch "hl.dsp.exec_cmd([[xdg-terminal-exec bash ${scripts_dir}/meal-break-dialog.sh '$1' ${2:-0}]], { float = true, size = \"1350 600\", center = true })"
	exit
fi

hour=`date +%H`

if [[ $hour == "@BreakfastHour@" ]]
then

	hyprctl dispatch "hl.dsp.exec_cmd([[xdg-terminal-exec bash ${scripts_dir}/meal-break-dialog.sh 'Breakfast']], { float = true, size = \"1350 600\", center = true })"
fi

if [[ $hour == "@LunchHour@" ]]
then
	hyprctl dispatch "hl.dsp.exec_cmd([[xdg-terminal-exec bash ${scripts_dir}/meal-break-dialog.sh 'Lunch']], { float = true, size = \"1350 600\", center = true })"
fi

if [[ $hour == "@DinnerHour@" ]]
then
	hyprctl dispatch "hl.dsp.exec_cmd([[xdg-terminal-exec bash ${scripts_dir}/meal-break-dialog.sh 'Dinner']], { float = true, size = \"1350 600\", center = true })"
fi
