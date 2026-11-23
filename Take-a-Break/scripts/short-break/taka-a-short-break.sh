#!/bin/bash

scripts_dir=$HOME/.Take-a-Break/scripts/short-break/

hyprctl dispatch "hl.dsp.exec_cmd([[xdg-terminal-exec bash ${scripts_dir}/short-break-dialog.sh]], { float = true, size = \"1350 600\", center = true })"
