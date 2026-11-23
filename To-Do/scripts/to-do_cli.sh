#!/bin/bash

work_dir=$HOME/Documents/To-Do
install_dir=$PWD

shift_time() {
	local base_time="$1"
	local offset_seconds="$2"
	local base_epoch

	base_epoch=$(date -d "$base_time" +%s 2>/dev/null) || return 1
	date -d "@$((base_epoch + offset_seconds))" '+%Y-%m-%d %H:%M'
}

offset_to_seconds() {
	case "$1" in
		"5 minutes")  echo 300 ;;
		"10 minutes") echo 600 ;;
		"15 minutes") echo 900 ;;
		"30 minutes") echo 1800 ;;
		"1 hour")     echo 3600 ;;
		"2 hours")    echo 7200 ;;
		"1 day")      echo 86400 ;;
		"2 days")     echo 172800 ;;
		"3 days")     echo 259200 ;;
		"4 days")     echo 345600 ;;
		"5 days")     echo 432000 ;;
		"6 days")     echo 518400 ;;
		"1 week")     echo 604800 ;;
		*) return 1 ;;
	esac
}

display_time() {
	date -d "$1" '+%Y-%m-%d %I:%M %p' 2>/dev/null
}

timing () {
	local header="$1"
	local hour minute period

	hour=$(gum choose --header "$header - Hour" --height 14 \
	01 02 03 04 05 06 07 08 09 10 11 12) || return 1
	minute=$(gum choose --header "$header - Minute" \
		00 15 30 45) || return 1
	period=$(gum choose --header "$header - AM/PM" "AM" "PM") || return 1
	printf '%s:%s %s\n' "$hour" "$minute" "$period"
}

# -------------------------
# Common helpers
# -------------------------

# list_dirs <parent_dir>
# Prints the directory names (underscores shown as spaces).
# Hidden (.tmp) and closed (DONE_ / CANCEL_ / DELETED_) directories are skipped.
list_dirs() {
	find "$1" -mindepth 1 -maxdepth 1 -type d ! -name '.*' 2>/dev/null |
		awk -F"/" '{print $NF}' |
		sed 's/_/ /g' |
		grep -vE '^(DONE|CANCEL|DELETED) ' |
		sort
}

# pick_dir <parent_dir> <header> <message when empty>
# Prints the selected directory name.
# Returns 1 if the list is empty or the selection is cancelled.
pick_dir() {
	local parent="$1"
	local header="$2"
	local empty_msg="$3"
	local items

	items=$(list_dirs "$parent")

	if [[ -z "$items" ]]
	then
		echo "$empty_msg" >&2
		return 1
	fi

	echo "$items" | gum choose --header "$header" || return 1
}

# check_name <name>
# Folder names are built from the name, so reject what would break the path
# or clash with the menu entries / closed prefixes.
check_name() {
	local name="$1"

	case "$name" in
		""|*"/"*|*"_"*|.*|"DONE "*|"CANCEL "*|"DELETED "*|"Back"|"Delete Label")
			echo "Invalid name. It cannot be empty, start with '.', 'DONE ', 'CANCEL ' or 'DELETED ', be 'Back' or 'Delete Label', or contain '/' or '_'."
			return 1
			;;
	esac
}

display_alerts() {
	local alerts_file="$1"
	local alert_time description
	local found=0

	printf '## Alerts\n'
	if [[ -f "$alerts_file" ]]; then
		while IFS='|' read -r alert_time description; do
			[[ "$alert_time" == \#* || -z "$alert_time" ]] && continue
			printf -- '- **%s** - %s\n' "$(display_time "$alert_time")" "$description"
			found=1
		done < "$alerts_file"
	fi

	(( found )) || printf -- '- No alerts configured.\n'
}

# display_recurrence <step>   e.g. "1 week" -> "Every week", "" -> "None"
display_recurrence() {
	if [[ -z "$1" ]]
	then
		echo "None"
	elif [[ "$1" == "1 "* ]]
	then
		echo "Every ${1#1 }"
	else
		echo "Every $1"
	fi
}

# choose_recurrence
# Prints the repeat step ("1 week", "2 months", ...) or nothing for "None".
# Returns 1 if cancelled or invalid.
choose_recurrence() {
	local choice n unit

	choice=$(gum choose --header "Repeat" "None" "Daily" "Weekly" "Every 2 Weeks" "Monthly" "Yearly" "Custom") || return 1

	case "$choice" in
		"None")          echo "" ;;
		"Daily")         echo "1 day" ;;
		"Weekly")        echo "1 week" ;;
		"Every 2 Weeks") echo "2 weeks" ;;
		"Monthly")       echo "1 month" ;;
		"Yearly")        echo "1 year" ;;
		"Custom")
			n=$(gum input --header "Repeat every how many?" --placeholder "e.g. 3") || return 1
			if ! [[ "$n" =~ ^[1-9][0-9]*$ ]]
			then
				echo "Enter a whole number greater than 0." >&2
				return 1
			fi
			unit=$(gum choose --header "Unit" "days" "weeks" "months" "years") || return 1
			echo "$n $unit"
			;;
	esac
}

# build_details <dir>
# Combines the field files in <dir>/.tmp into <dir>/details.md
build_details() {
	local dir="$1"
	local tmp_dir="$dir/.tmp"
	local sent
	local latest_worklog

	latest_worklog=$(find "$tmp_dir/worklog" -maxdepth 1 -type f -name '*.md' 2>/dev/null | sort -r | head -n 1)

	{
		printf '# Title: %s\n\n## Description\n%s\n' \
			"$(cat "$tmp_dir/title")" "$(cat "$tmp_dir/description")"

		if [[ -n "$latest_worklog" ]]; then
			printf '\n'
			cat "$latest_worklog"
			printf '\n'
		fi

		printf '===\n---\n\n'
		printf -- '- **Start Time:** %s\n' "$(cat "$tmp_dir/start_time")"
		printf -- '- **Due Time:** %s\n' "$(cat "$tmp_dir/due_time")"
		printf -- '- **Priority:** %s\n' "$(cat "$tmp_dir/priority")"
		printf -- '- **Status:** %s\n' "$(cat "$tmp_dir/status")"
		printf -- '- **Repeat:** %s\n\n' "$(display_recurrence "$(cat "$tmp_dir/recurrence" 2>/dev/null)")"

		display_alerts "$tmp_dir/alerts"

		printf '\n## Alert History\n'
		if [[ -s "$tmp_dir/alert_history" ]]
		then
			while IFS= read -r sent
			do
				printf -- '- %s\n' "$(display_time "$sent")"
			done < "$tmp_dir/alert_history"
		else
			printf -- '- No alerts sent yet.\n'
		fi
	} > "$dir/details.md"
}
# add_work_log <dir>
# Saves a work-log entry with its date, time and description.
add_work_log() {
	local dir="$1"
	local worklog_dir="$dir/.tmp/worklog"
	local description now timestamp log_time worklog_file
	local suffix=1

	description=$(gum write --header "Work Log Description" --placeholder "What did you work on?") || return
	[[ -z "$description" ]] && return

	mkdir -p "$worklog_dir"

	now=$(date +%s)
	timestamp=$(date -d "@$now" '+%Y-%m-%d_%H-%M-%S')
	log_time=$(date -d "@$now" '+%Y-%m-%d %H:%M')
	worklog_file="$worklog_dir/$timestamp.md"

	while [[ -e "$worklog_file" ]]; do
		worklog_file="$worklog_dir/${timestamp}_${suffix}.md"
		suffix=$((suffix + 1))
	done

	{
		printf '### Work Log — %s\n\n' "$log_time"
		printf '%s\n' "$description"
	} > "$worklog_file"
}

# view_work_logs <dir>
# Displays all saved work logs, newest first.
view_work_logs() {
    local worklog_dir="$1/.tmp/worklog"
    local log_file

    clear

    if [[ ! -d "$worklog_dir" ]] || \
       [[ -z "$(find "$worklog_dir" -maxdepth 1 -type f -name '*.md' -print -quit)" ]]
    then
        echo "No work logs available."
        read -r -p "Press Enter to return..."
        return
    fi

    {
        printf '# Work Logs\n\n'

        while IFS= read -r log_file; do
            cat "$log_file"
            printf '\n---\n\n'
        done < <(find "$worklog_dir" -maxdepth 1 -type f -name '*.md' | sort -r)

    } | gum format

    read -r -p "Press Enter to return..."
}

# configure_alerts
# Uses start_time and due_time, fills the alerts array (max 5 per run).
configure_alerts() {
	alerts=()
	while (( ${#alerts[@]} < 5 ))
	do
		alert_type=$(gum choose --header "Configure Alert ($(( ${#alerts[@]} + 1 ))/5)" \
			"At Start Time" \
			"Before Start Time" \
			"At Due Time" \
			"Before Due Time" \
			"Custom" \
			"Finish Alerts") || break

		[[ "$alert_type" == "Finish Alerts" ]] && break

		case "$alert_type" in
			"At Start Time")
				alert_time="$start_time"
				alerts+=("$alert_time|At start time")
				;;
			"Before Start Time")
				alert_offset=$(gum choose --header "How long before start?" \
					"5 minutes" "10 minutes" "15 minutes" "30 minutes" \
					"1 hour" "2 hours" "1 day" "2 days" "3 days" "1 week") || continue
				offset_seconds=$(offset_to_seconds "$alert_offset") || continue
				alert_time=$(shift_time "$start_time" "$((-offset_seconds))") || continue
				alerts+=("$alert_time|$alert_offset before start")
				;;
			"At Due Time")
				alert_time="$due_time"
				alerts+=("$alert_time|At due time")
				;;
			"Before Due Time")
				alert_offset=$(gum choose --header "How long before due?" \
					"1 day" "2 days" "3 days" "4 days" "5 days" "6 days" "1 week") || continue
				offset_seconds=$(offset_to_seconds "$alert_offset") || continue
				alert_time=$(shift_time "$due_time" "$((-offset_seconds))") || continue
				alerts+=("$alert_time|$alert_offset before due")
				;;
			"Custom")
				custom_date=$(gum input --header "Custom Alert Date" \
					--value "$(date '+%Y-%m-%d')" --placeholder "YYYY-MM-DD") || continue
				custom_clock=$(timing "Custom Alert Time") || continue
				custom_time=$(date -d "$custom_date $custom_clock" '+%Y-%m-%d %H:%M' 2>/dev/null) || {
					echo "Invalid custom alert date/time."
					continue
				}
				alerts+=("$custom_time|Custom alert")
				;;
		esac
	done
}

create_entity() {
	entity_path=$1
	entity_title=$2
	entity_type=$3

	description=$(gum write --header "Short Description" --placeholder "Enter description") || return

	start_date=$(gum input --header "Start Date" --value "$(date '+%Y-%m-%d')" --placeholder "YYYY-MM-DD") || return
	start_clock=$(timing "Start Time") || return
	start_time=$(date -d "$start_date $start_clock" '+%Y-%m-%d %H:%M' 2>/dev/null) || {
		echo "Invalid start date/time."
		return
	}

	due_date=$(gum input --header "Due Date" --value "$(date '+%Y-%m-%d')" --placeholder "YYYY-MM-DD") || return
	due_clock=$(timing "Due Time") || return
	due_time=$(date -d "$due_date $due_clock" '+%Y-%m-%d %H:%M' 2>/dev/null) || {
		echo "Invalid due date/time."
		return
	}

	priority=$(gum choose --header "Priority" "Low" "Medium" "High" "Critical") || return
	status=$(gum choose --header "Status" "Todo" "In Progress" "On Hold") || return
	recurrence=$(choose_recurrence) || return

	configure_alerts

	alerts_display="No alerts configured."
	if (( ${#alerts[@]} > 0 )); then
		alerts_display=""
		while IFS='|' read -r alert_time alert_description; do
			alerts_display+="- $(display_time "$alert_time") - $alert_description"$'\n'
		done < <(printf '%s\n' "${alerts[@]}" | sort -t '|' -k1,1)
	fi

	printf '%s\n' \
		"Title: $entity_title" \
		"Description: $description" \
		"Start: $(display_time "$start_time")" \
		"Due: $(display_time "$due_time")" \
		"Priority: $priority" \
		"Status: $status" \
		"Repeat: $(display_recurrence "$recurrence")" \
		"Alerts:" \
		"$alerts_display" | gum style --border rounded --padding '1 2'

	if gum confirm "Create this $entity_type?"
	then
		gum spin --spinner meter --title "Creating '$entity_title' $entity_type..." -- sleep 3
		entity_dir="$entity_path/${entity_title// /_}"
		tmp_dir="$entity_dir/.tmp"
		mkdir -p "$tmp_dir"

		printf '%s\n' "$entity_title" > "$tmp_dir/title"
		printf '%s\n' "$description" > "$tmp_dir/description"
		printf '%s\n' "$start_time" > "$tmp_dir/start_time"
		printf '%s\n' "$due_time" > "$tmp_dir/due_time"
		printf '%s\n' "$priority" > "$tmp_dir/priority"
		printf '%s\n' "$status" > "$tmp_dir/status"
		printf '%s\n' "$recurrence" > "$tmp_dir/recurrence"

		# Alerts file, notification history file and a saved copy of the alerts
		# (alerts_template) that the reminder never touches, used for the next repeat.
		touch "$tmp_dir/alerts" "$tmp_dir/alerts_template" "$tmp_dir/alert_history"
		if (( ${#alerts[@]} > 0 )); then
			printf '%s\n' "${alerts[@]}" | sort -t '|' -k1,1 > "$tmp_dir/alerts"
			cp "$tmp_dir/alerts" "$tmp_dir/alerts_template"
		fi

		build_details "$entity_dir"
	else
		echo "Cancelled."
	fi
}

add_menu() {
    while true; do
        add=$(gum choose "Label" "Task" "Back") || break

        case "$add" in
		"Label")
    		     label=$(gum input --placeholder "Enter label name") || continue
		     check_name "$label" || continue
		     label_flg=$(ls -l $work_dir/${label// /_}  2>/dev/null| wc -l)
    		     if [[ $label_flg == "0" ]]
		     then
			     #echo "Creating '$label' label."
			     gum spin --spinner meter --title "Creating '$label' label..." -- sleep 3
			     mkdir -p $work_dir/${label// /_}
    		     else
			     echo "Label '$label' already exists."
		     fi
		     ;;
            "Task")
		label=$(pick_dir "$work_dir" "Select the Label under which task to be added" "No labels yet.") || continue
		task=$(gum input --placeholder "Enter task name") || continue
		check_name "$task" || continue
		task_flg=$(ls -l $work_dir/${label// /_}/${task// /_} 2>/dev/null| wc -l)
		if [[ $task_flg == "0" ]]
		then
			create_entity "$work_dir/${label// /_}" "$task" "Task"
		else
			echo "Task '$task' already exists."
		fi
                ;;
            "Back")
                break
                ;;
        esac
    done
}

# -------------------------
# Completed / Canceled
# -------------------------

# close_tree <dir> <DONE|CANCEL> <status text>
# Closes every open sub-task first (same status as the parent), then <dir> itself.
# Already closed sub-tasks are left as they are.
close_tree() {
	local dir="$1"
	local prefix="$2"
	local status="$3"
	local children=()
	local child parent target

	mapfile -t children < <(find "$dir" -mindepth 1 -maxdepth 1 -type d ! -name '.*' ! -name 'DONE_*' ! -name 'CANCEL_*')
	for child in "${children[@]}"
	do
		close_tree "$child" "$prefix" "$status"
	done

	printf '%s\n' "$status" > "$dir/.tmp/status"
	build_details "$dir"

	# Closed folders are not shown anywhere, so a timestamp keeps the name unique.
	parent=$(dirname "$dir")
	target="${prefix}_$(basename "$dir")"
	if [[ -e "$parent/$target" ]]
	then
		target="${target}_$(date +%s)"
	fi
	mv "$dir" "$parent/$target"
}

# spawn_next <snapshot_dir> <new_dir>
# Creates the next occurrence of a repeating task from a copy of the finished
# task's .tmp files. Dates move forward by the repeat step (k steps from the old
# dates) until the due time is in the future. Sub-tasks are not copied.
# Note: the step is written without a '+' ("1 week"), because after a time GNU date
# reads '+N' as a timezone offset and shifts the clock time.
spawn_next() {
	local snap="$1"
	local new_dir="$2"
	local new_tmp="$new_dir/.tmp"
	local step n unit k now old_start old_due new_start new_due
	local a_time a_desc a_new offset seconds
	local all_alerts=()

	step=$(cat "$snap/recurrence")
	read -r n unit <<< "$step"
	old_start=$(cat "$snap/start_time")
	old_due=$(cat "$snap/due_time")
	now=$(date +%s)

	if ! date -d "$old_due $n $unit" >/dev/null 2>&1
	then
		echo "Could not calculate the next occurrence from '$step'."
		return 1
	fi

	k=1
	while (( $(date -d "$old_due $((k * n)) $unit" +%s) <= now ))
	do
		k=$((k + 1))
	done
	new_start=$(date -d "$old_start $((k * n)) $unit" '+%Y-%m-%d %H:%M')
	new_due=$(date -d "$old_due $((k * n)) $unit" '+%Y-%m-%d %H:%M')

	# Work the alerts out again from their descriptions (e.g. "1 day before due").
	if [[ -f "$snap/alerts_template" ]]
	then
		while IFS='|' read -r a_time a_desc
		do
			[[ -z "$a_time" ]] && continue
			case "$a_desc" in
				"At start time")
					a_new="$new_start"
					;;
				"At due time")
					a_new="$new_due"
					;;
				*" before start")
					offset="${a_desc% before start}"
					seconds=$(offset_to_seconds "$offset") || continue
					a_new=$(shift_time "$new_start" "$((-seconds))") || continue
					;;
				*" before due")
					offset="${a_desc% before due}"
					seconds=$(offset_to_seconds "$offset") || continue
					a_new=$(shift_time "$new_due" "$((-seconds))") || continue
					;;
				*)
					a_new=$(date -d "$a_time $((k * n)) $unit" '+%Y-%m-%d %H:%M' 2>/dev/null) || continue
					;;
			esac
			all_alerts+=("$a_new|$a_desc")
		done < "$snap/alerts_template"
	fi

	mkdir -p "$new_tmp"
	cp "$snap/title" "$snap/description" "$snap/priority" "$snap/recurrence" "$new_tmp/"
	printf '%s\n' "$new_start" > "$new_tmp/start_time"
	printf '%s\n' "$new_due" > "$new_tmp/due_time"
	printf '%s\n' "Todo" > "$new_tmp/status"
	touch "$new_tmp/alerts" "$new_tmp/alerts_template" "$new_tmp/alert_history"

	if (( ${#all_alerts[@]} > 0 )); then
		printf '%s\n' "${all_alerts[@]}" | sort -t '|' -k1,1 > "$new_tmp/alerts_template"
		# Alerts already in the past are not queued, they would all fire at once.
		while IFS='|' read -r a_time a_desc
		do
			if (( $(date -d "$a_time" +%s) > now ))
			then
				printf '%s|%s\n' "$a_time" "$a_desc" >> "$new_tmp/alerts"
			fi
		done < "$new_tmp/alerts_template"
	fi

	build_details "$new_dir"
	echo "Next occurrence created, due $(display_time "$new_due")."
}

# close_entity <dir> <DONE|CANCEL> <status text>
# Returns 0 only when the folder was renamed (path changed).
# A repeating task creates its next occurrence when completed; canceling stops the repeat.
close_entity() {
	local dir="$1"
	local prefix="$2"
	local status="$3"
	local name sub_count msg recurrence parent folder
	local snap=""

	name=$(cat "$dir/.tmp/title")
	recurrence=$(cat "$dir/.tmp/recurrence" 2>/dev/null)
	sub_count=$(find "$dir" -mindepth 1 \( -name '.*' -o -name 'DONE_*' -o -name 'CANCEL_*' \) -prune -o -type d -print | wc -l)

	msg="Mark '$name' as $status ?"
	if (( sub_count > 0 ))
	then
		msg="Mark '$name' and its $sub_count open sub-task(s) as $status ?"
	fi

	if [[ -n "$recurrence" ]]
	then
		if [[ "$prefix" == "DONE" ]]
		then
			msg="$msg The next occurrence will be created."
		else
			msg="$msg The repeat will stop."
		fi
	fi

	if gum confirm "$msg"
	then
		if [[ -n "$recurrence" && "$prefix" == "DONE" ]]
		then
			snap=$(mktemp -d)
			cp -r "$dir/.tmp/." "$snap/"
		fi

		parent=$(dirname "$dir")
		folder=$(basename "$dir")
		close_tree "$dir" "$prefix" "$status"
		echo "'$name' marked as $status."

		if [[ -n "$snap" ]]
		then
			spawn_next "$snap" "$parent/$folder"
			rm -rf "$snap"
		fi
	else
		return 1
	fi
}

# -------------------------
# Edit
# -------------------------

# edit_entity <dir>
# Updates the field file in <dir>/.tmp and rebuilds details.md.
# Returns 2 if the title changed (the folder was renamed), otherwise 0.
edit_entity() {
	local dir="$1"
	local tmp_dir="$dir/.tmp"
	local field current new_value alert_action alert_list alert_line
	local start_date start_clock due_date due_clock

	while true
	do
		field=$(gum choose --header "Edit" "Title" "Description" "Priority" "Status" "Start & Due Date" "Repeat" "Alerts" "Back") || return

		case "$field" in
			"Title")
				current=$(cat "$tmp_dir/title")
				new_value=$(gum input --header "Title" --value "$current") || continue
				check_name "$new_value" || continue
				if [[ "$new_value" == "$current" ]]
				then
					continue
				fi
				if [[ -e "$(dirname "$dir")/${new_value// /_}" ]]
				then
					echo "'$new_value' already exists."
					continue
				fi
				printf '%s\n' "$new_value" > "$tmp_dir/title"
				build_details "$dir"
				mv "$dir" "$(dirname "$dir")/${new_value// /_}"
				echo "Title changed to '$new_value'."
				return 2
				;;
			"Description")
				current=$(cat "$tmp_dir/description")
				new_value=$(gum write --header "Short Description" --value "$current") || continue
				printf '%s\n' "$new_value" > "$tmp_dir/description"
				;;
			"Priority")
				new_value=$(gum choose --header "Priority" "Low" "Medium" "High" "Critical") || continue
				printf '%s\n' "$new_value" > "$tmp_dir/priority"
				;;
			"Status")
				new_value=$(gum choose --header "Status" "Todo" "In Progress" "On Hold") || continue
				printf '%s\n' "$new_value" > "$tmp_dir/status"
				;;
			"Start & Due Date")
				start_date=$(gum input --header "Start Date" --value "$(cut -d' ' -f1 "$tmp_dir/start_time")" --placeholder "YYYY-MM-DD") || continue
				start_clock=$(timing "Start Time") || continue
				start_time=$(date -d "$start_date $start_clock" '+%Y-%m-%d %H:%M' 2>/dev/null) || {
					echo "Invalid start date/time."
					continue
				}

				due_date=$(gum input --header "Due Date" --value "$(cut -d' ' -f1 "$tmp_dir/due_time")" --placeholder "YYYY-MM-DD") || continue
				due_clock=$(timing "Due Time") || continue
				due_time=$(date -d "$due_date $due_clock" '+%Y-%m-%d %H:%M' 2>/dev/null) || {
					echo "Invalid due date/time."
					continue
				}

				printf '%s\n' "$start_time" > "$tmp_dir/start_time"
				printf '%s\n' "$due_time" > "$tmp_dir/due_time"

				# Alerts are stored as absolute times, so they must be rebuilt for the new dates.
				if gum confirm "Dates changed. Reconfigure alerts now?"
				then
					configure_alerts
					: > "$tmp_dir/alerts"
					if (( ${#alerts[@]} > 0 )); then
						printf '%s\n' "${alerts[@]}" | sort -t '|' -k1,1 > "$tmp_dir/alerts"
					fi
					cp "$tmp_dir/alerts" "$tmp_dir/alerts_template"
				fi
				;;
			"Repeat")
				new_value=$(choose_recurrence) || continue
				printf '%s\n' "$new_value" > "$tmp_dir/recurrence"
				# Repeating tasks rebuild their alerts from this saved copy.
				if [[ ! -f "$tmp_dir/alerts_template" ]]
				then
					cp "$tmp_dir/alerts" "$tmp_dir/alerts_template"
				fi
				;;
			"Alerts")
				alert_action=$(gum choose --header "Alerts" "Add Alert" "Remove Alert" "Clear All Alerts" "Back") || continue

				case "$alert_action" in
					"Add Alert")
						start_time=$(cat "$tmp_dir/start_time")
						due_time=$(cat "$tmp_dir/due_time")
						configure_alerts
						if (( ${#alerts[@]} > 0 )); then
							{
								cat "$tmp_dir/alerts"
								printf '%s\n' "${alerts[@]}"
							} | sort -t '|' -k1,1 > "$tmp_dir/alerts.new"
							mv "$tmp_dir/alerts.new" "$tmp_dir/alerts"
							if [[ -f "$tmp_dir/alerts_template" ]]
							then
								printf '%s\n' "${alerts[@]}" >> "$tmp_dir/alerts_template"
							fi
						fi
						;;
					"Remove Alert")
						alert_list=$(cat "$tmp_dir/alerts")
						if [[ -z "$alert_list" ]]
						then
							echo "No alerts configured."
							continue
						fi
						alert_line=$(echo "$alert_list" | gum choose --header "Select the alert to remove") || continue
						grep -vxF -- "$alert_line" "$tmp_dir/alerts" > "$tmp_dir/alerts.new"
						mv "$tmp_dir/alerts.new" "$tmp_dir/alerts"
						if [[ -f "$tmp_dir/alerts_template" ]]
						then
							grep -vxF -- "$alert_line" "$tmp_dir/alerts_template" > "$tmp_dir/alerts.new"
							mv "$tmp_dir/alerts.new" "$tmp_dir/alerts_template"
						fi
						;;
					"Clear All Alerts")
						if gum confirm "Remove all alerts?"
						then
							: > "$tmp_dir/alerts"
							: > "$tmp_dir/alerts_template"
						fi
						;;
				esac
				;;
			"Back")
				return
				;;
		esac

		build_details "$dir"
	done
}

# -------------------------
# Delete Label
# -------------------------

# delete_label <label name>
# Cancels every open task/sub-task under the label, then renames the label
# folder with the DELETED_ prefix so it is hidden from the labels menu.
# Returns 0 only when the label was deleted.
delete_label() {
	local label="$1"
	local label_dir="$work_dir/${label// /_}"
	local tasks=()
	local task_dir target task_count

	task_count=$(find "$label_dir" -mindepth 1 -maxdepth 1 -type d ! -name '.*' ! -name 'DONE_*' ! -name 'CANCEL_*' | wc -l)

	if gum confirm "Delete label '$label' and cancel its $task_count open task(s)?"
	then
		mapfile -t tasks < <(find "$label_dir" -mindepth 1 -maxdepth 1 -type d ! -name '.*' ! -name 'DONE_*' ! -name 'CANCEL_*')
		for task_dir in "${tasks[@]}"
		do
			close_tree "$task_dir" "CANCEL" "Canceled"
		done

		target="DELETED_${label// /_}"
		if [[ -e "$work_dir/$target" ]]
		then
			target="${target}_$(date +%s)"
		fi
		mv "$label_dir" "$work_dir/$target"
		echo "Label '$label' deleted."
	else
		return 1
	fi
}

# -------------------------
# List Menus
# -------------------------

# entity_menu <dir>
# Shows a task/sub-task with its actions. Returns when the user goes back
# or when the folder path changed (completed, canceled, title edited).
# Only a task (directly under a label) can have sub-tasks.
entity_menu() {
	local dir="$1"
	local action sub_task sub_task_flg
	local actions=()
	
	while true
	do
		clear
		build_details "$dir"
		gum format < "$dir/details.md"
		
		actions=("Edit" "Add Work Log" "Work Logs")
		if [[ "$(dirname "$(dirname "$dir")")" == "$work_dir" ]]
		then
			actions+=("Add Sub-Task")
			if [[ -n "$(find "$dir" -mindepth 1 -maxdepth 1 -type d ! -name '.*' ! -name 'DONE_*' ! -name 'CANCEL_*' -print -quit)" ]]
			then
				actions+=("List Sub-Tasks")
			fi
		fi
		actions+=("Completed" "Canceled" "Back")
		action=$(gum choose "${actions[@]}") || return

		case "$action" in
			"Edit")
				edit_entity "$dir"
				if [[ $? == "2" ]]
				then
					return
				fi
				;;
			"Add Work Log")
				add_work_log "$dir"
				;;
			"Work Logs")
				view_work_logs "$dir"
				;;
			"Add Sub-Task")
				sub_task=$(gum input --placeholder "Enter sub-task name") || continue
				check_name "$sub_task" || continue
				sub_task_flg=$(ls -l "$dir/${sub_task// /_}" 2>/dev/null | wc -l)
				if [[ $sub_task_flg == "0" ]]
				then
					create_entity "$dir" "$sub_task" "Sub-Task"
				else
					echo "Sub-task '$sub_task' already exists."
				fi
				;;
			"List Sub-Tasks")
				sub_task=$(pick_dir "$dir" "Below are Sub-Tasks" "No sub-tasks yet.") || continue
				entity_menu "$dir/${sub_task// /_}"
				;;
			"Completed")
				close_entity "$dir" "DONE" "Completed" && return
				;;
			"Canceled")
				close_entity "$dir" "CANCEL" "Canceled" && return
				;;
			"Back")
				return
				;;
		esac
	done
}

# labels_menu <label name>
labels_menu() {
	local label="$1"
	local label_dir="$work_dir/${label// /_}"
	local task

	while true
	do
		task=$({ list_dirs "$label_dir"; printf 'Delete Label\nBack\n'; } | gum choose --header "Tasks under '$label'") || return

		case "$task" in
			"Delete Label")
				delete_label "$label" && return
				;;
			"Back")
				return
				;;
			*)
				entity_menu "$label_dir/${task// /_}"
				;;
		esac
	done
}

list_menu() {
	local label labels

	while true
	do
		labels=$(list_dirs "$work_dir")
		if [[ -z "$labels" ]]
		then
			echo "No labels yet."
			return
		fi

		label=$({ echo "$labels"; echo "Back"; } | gum choose --header "Select Label") || return

		if [[ "$label" == "Back" ]]
		then
			return
		fi

		labels_menu "$label"
	done
}

# -------------------------
# Home Menu
# -------------------------
home_menu() {
    while true; do
        home=$(gum choose --header "To-Do" "List" "Add" "Exit") || break

        case "$home" in
            "List")
                list_menu
                ;;
            "Add")
                add_menu
                ;;
            "Exit")
                break
                ;;
        esac
    done
}
home_menu
