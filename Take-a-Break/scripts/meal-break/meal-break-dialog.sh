#!/usr/bin/env bash
# dialog.sh - bordered message box + Yes/No confirm in the terminal.
# Uses gum (as Omarchy does) when available, falls back to plain bash.
#
# Usage as a script:  ./dialog.sh
# Usage as a library: source dialog.sh; dialog_confirm "message" "Question?"
# Return code: 0 = Yes, 1 = No / cancelled

set -u

work_dir=$HOME/.Take-a-Break
ascii=$work_dir/assets/meal-ascii.txt
meal=$1
snoozed=${2:-0}
max_snooze=2
snooze_min=30

# --- plain-bash message box -------------------------------------------------
# Pad by character count (printf %-Ns pads by bytes and breaks on "•")
_row() {
  local s=$1 w=$2 pad
  printf -v pad '%*s' $(( w - ${#s} )) ''
  printf '│ %s%s │\n' "$s" "$pad"
}

_box() {
  local text=$1 line w=0 bar="" i
  while IFS= read -r line; do
    (( ${#line} > w )) && w=${#line}
  done <<<"$text"
  for ((i = 0; i < w + 2; i++)); do bar+="─"; done

  printf '┌%s┐\n' "$bar"
  _row "" "$w"
  while IFS= read -r line; do
    _row "$line" "$w"
  done <<<"$text"
  _row "" "$w"
  printf '└%s┘\n' "$bar"
}

# --- dialog_confirm "message" "question" [default: no|yes] ------------------
dialog_confirm() {
  local message=$1 question=${2:-"Continue?"} default=${3:-yes} negative=${4:-"Not Now"}

  if command -v gum >/dev/null 2>&1; then
    gum style --border normal --border-foreground 7 \
      --padding "1 2" --margin "1 0 0 0" "$message"
    echo
    local def=false
    [[ $default == yes ]] && def=true
    gum confirm --affirmative "Sure" --negative "$negative" --default="$def" "$question"
    return $?
  fi

  # Fallback: no gum installed
  echo
  _box "$message"
  echo
  local hint="[y/N]" ans
  [[ $default == yes ]] && hint="[Y/n] (n = $negative)"
  while true; do
    read -r -n1 -s -p "$question $hint " ans || return 1
    echo
    case ${ans,,} in
      y) return 0 ;;
      n) return 1 ;;
      "") [[ $default == yes ]] && return 0 || return 1 ;;
    esac
  done
}

# --- demo (only runs when executed, not when sourced) -----------------------
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  cat $ascii
  MSG=`echo "It's $meal time, Take a break and have a $meal"`

  left=$(( max_snooze - snoozed ))
  neg="Not Now"
  (( left > 0 )) && neg="Snooze ${snooze_min} min ($left left)"

  if dialog_confirm "$MSG" "Continue?" yes "$neg"; then
    echo "Locking the system..."
    # your update commands here
    sleep 2s
    omarchy system lock
  elif (( left > 0 )); then
    echo "Snoozing for ${snooze_min} minutes..."
    systemd-run --user --quiet --setenv=HYPRLAND_INSTANCE_SIGNATURE --on-active=${snooze_min}m \
      "$HOME/.Take-a-Break/scripts/meal-break/take-a-meal-break.sh" "$meal" $(( snoozed + 1 ))
    sleep 2s
  else
    echo "Ok, But have $meal before it too late."
    sleep 2s
    exit 1
  fi
fi
