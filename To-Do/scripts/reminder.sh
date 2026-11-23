#!/bin/bash

pgrep -x hyprlock >/dev/null && exit 0

work_dir=@DATA_DIR@
install_dir=$HOME/.To-Do
alerts_dir="$install_dir/scripts/alerts_files"

mkdir -p "$alerts_dir"

remove_alert() {
        local alerts_file="$1"
        local alert="$2"
        local temp_file

        temp_file=$(mktemp) || return 1

        awk -F'|' -v alert="$alert" '$1 != alert' "$alerts_file" > "$temp_file" &&
                mv "$temp_file" "$alerts_file" || {
                        rm -f "$temp_file"
                        return 1
                }
}

send_notification() {

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
                                remove_alert "$file" "$alert"
                                continue
                        fi

                        if ! date -d "$alert" +%s >/dev/null 2>&1
                        then
                                continue
                        fi

                        current_time=$(date +%s)
                        alert_time=$(date -d "$alert" +%s)

                        if (( alert_time <= current_time ))
                        then
                                if notify-send -u critical -t 0 "To-Do Reminder" "$task"; then

                                        echo "$alert" >> "$task_dir/.tmp/alert_history"
                                        remove_alert "$file" "$alert"

                                fi
                        fi

                        break

                done < <(grep -v '#' "$file" | awk -F'|' '{print $1}')

        done < <(
                find "$work_dir" -type f -path '*/.tmp/alerts' |
                grep -vE '/(DONE|CANCEL|DELETED)_' |
                sort -u
        )
}

send_notification
