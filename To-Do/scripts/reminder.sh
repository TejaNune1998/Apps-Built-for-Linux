#!/bin/bash

pgrep -x hyprlock >/dev/null && exit 0

work_dir=$HOME/Documents/To-Do
install_dir=$PWD
alerts_dir=$install_dir/scripts/alerts_files

get_details_files() {
	rm "$alerts_dir/alerts_details.txt" 2>/dev/null
	find "$work_dir" -type f -path '*/.tmp/alerts' -print | grep -vE '/(DONE|CANCEL|DELETED)_' |sort -u > "$alerts_dir/alerts_details.txt"
}

get_task_details() {
	rm "$alerts_dir/tasks_and_alerts.csv" 2>/dev/null
	touch "$alerts_dir/tasks_and_alerts.csv"
	while IFS= read -r file
	do
		task_dir=$(dirname "$(dirname "$file")")
		task=$(cat "$task_dir/.tmp/title")

		while IFS= read -r alert
		do
			if [[ -z "$alert" ]]
			then
				continue
			fi

			if grep -Fxq -- "$alert" "$task_dir/.tmp/alert_history" 2>/dev/null
			then
				sed -i "\|^${alert}|d" "$file"
				continue
			fi

			if ! date -d "$alert" +%s >/dev/null 2>&1
			then
				continue
			fi

			echo "$task,$alert,$task_dir" >> $alerts_dir/tasks_and_alerts.csv
			break

		done < <(grep -v '#' "$file" | awk -F'|' '{print $1}')

	done < "$alerts_dir/alerts_details.txt"
	sort -u -o "$alerts_dir/tasks_and_alerts.csv" "$alerts_dir/tasks_and_alerts.csv"

}


send_notification() {
	while IFS=',' read -r task alert task_dir
	do
		if [[ -n "$alert" ]]
		then
			if grep -Fxq -- "$alert" "$task_dir/.tmp/alert_history" 2>/dev/null
			then
    				continue
			fi
			current_time=$(date +%s)
                   	alert_time=$(date -d "$alert" +%s)
			if (( alert_time <= current_time ))
			then
				if notify-send -u critical -t 0 "To-Do Reminder" "$task"; then
   					 echo "$alert" >> "$task_dir/.tmp/alert_history"
    					sed -i "\|^${alert}|d" "$task_dir/.tmp/alerts"
				fi
			fi
		fi

	done < "$alerts_dir/tasks_and_alerts.csv"
}

get_details_files
get_task_details
send_notification
