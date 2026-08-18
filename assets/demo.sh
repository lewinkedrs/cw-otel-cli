#!/usr/bin/env bash
# Drives the cwpromql demo for asciinema recording. Prints a prompt + the
# command (so viewers see what's run), then executes it, with pauses.
#
# Record + render (needs AWS creds + cwpromql on PATH):
#   asciinema rec --overwrite --window-size 120x32 --command "bash assets/demo.sh" assets/demo.cast
#   agg --font-size 20 --theme dracula --speed 1.3 assets/demo.cast assets/demo.gif
set -u

PROMPT=$'\033[38;5;39m❯\033[0m '

run() {
	printf '%s%s\n' "$PROMPT" "$1"
	sleep 0.7
	eval "$1"
	sleep 2.2
}

clear
sleep 0.5
run 'cwpromql --version'
run 'cwpromql metrics --filter DCGM'
run 'cwpromql query '\''sum by ("@resource.k8s.namespace.name")({"up"})'\'''
run 'cwpromql range '\''{"DCGM_FI_DEV_GPU_UTIL"}'\'' --since 1h --height 12'
sleep 1.5
