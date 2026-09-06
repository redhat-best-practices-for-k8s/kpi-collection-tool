#!/bin/bash
# Runs inside chroot /host so top and /proc are the node's.
# Env: ISOLCPUS (optional), TOP_DELAY, TOP_COUNT, MEMINFO_MARKER, CMDLINE_MARKER
set -e
isol="$ISOLCPUS"
if [ -z "$isol" ]; then
  for field in $(cat /proc/cmdline); do
    case "$field" in
      isolcpus=*) isol=${field#isolcpus=} ;;
    esac
  done
  isol=$(echo "$isol" | tr ',' '\n' | grep -v '^$' | grep -v '^managed_irq$' | paste -sd, -)
fi
if [ -z "$isol" ]; then
  echo "isolcpus list is empty" >&2
  exit 1
fi
taskset -c "$isol" top -b -1 -i -w 200 -d "$TOP_DELAY" -n "$TOP_COUNT"
printf '%s\n' "$MEMINFO_MARKER"
cat /proc/meminfo
printf '%s\n' "$CMDLINE_MARKER"
cat /proc/cmdline
