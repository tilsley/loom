#!/usr/bin/env bash
# PostToolUse hook — reminds Claude to run 'make generate' after editing the OpenAPI schema.
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
    *schemas/openapi.yaml)
        echo "REMINDER: schemas/openapi.yaml was modified."
        echo "Run 'make generate' now to regenerate pkg/api/types.gen.go and apps/console/src/lib/api.gen.ts"
        echo "before running, testing, or making further edits."
        ;;
esac
