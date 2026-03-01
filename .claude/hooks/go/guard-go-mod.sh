#!/usr/bin/env bash
# PreToolUse hook — blocks direct edits to go.mod and go.sum.
# These files must only be modified via go tooling (go get, go mod tidy, etc.)

file_path=$(python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('tool_input', {}).get('file_path', ''))
except Exception:
    print('')
" 2>/dev/null)

case "$file_path" in
    */go.mod | */go.sum)
        echo "ERROR: '$file_path' must not be edited directly." >&2
        echo "Use 'go get <pkg>', 'go mod tidy', or 'go mod edit' instead." >&2
        exit 1
        ;;
esac
