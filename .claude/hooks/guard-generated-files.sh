#!/usr/bin/env bash
# PreToolUse hook — blocks direct edits to generated files.
# Claude Code passes tool input as JSON on stdin.

file_path=$(python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('tool_input', {}).get('file_path', ''))
except Exception:
    print('')
" 2>/dev/null)

case "$file_path" in
    *.gen.go | *api.gen.ts)
        echo "ERROR: '$file_path' is a generated file — do not edit it directly." >&2
        echo "Edit schemas/openapi.yaml and run 'make generate' instead." >&2
        exit 1
        ;;
esac
