#!/bin/sh
# step.sh LABEL COMMAND [ARGS...]
# Runs COMMAND with its output hidden and prints "  LABEL… ✓". If it fails,
# prints "✗" and the output, then exits with its status. Used by the
# Taskfile so `task install` shows what it does, not every tool's chatter.
label=$1
shift
if [ -t 1 ]; then ok='\033[32m✓\033[0m' bad='\033[31m✗\033[0m'; else ok='✓' bad='✗'; fi
printf '  %s… ' "$label"
log=$(mktemp)
"$@" >"$log" 2>&1
status=$?
if [ $status -eq 0 ]; then
  printf "$ok\n"
else
  printf "$bad\n\n"
  cat "$log"
fi
rm -f "$log"
exit $status
