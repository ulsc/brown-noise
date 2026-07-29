#!/bin/sh

# Match only the installed executable name so unrelated command lines are not
# terminated accidentally.
if pgrep -x brown_noise > /dev/null; then
  if pkill -x brown_noise; then
    echo "Brown noise stopped."
  else
    echo "Failed to stop brown noise." >&2
    exit 1
  fi
else
  if ! command -v brown_noise > /dev/null 2>&1; then
    echo "brown_noise is not installed or is not on PATH." >&2
    exit 1
  fi

  nohup brown_noise > /dev/null 2>&1 &
  echo "Brown noise started (PID $!)."
fi
