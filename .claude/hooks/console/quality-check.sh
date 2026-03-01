#!/usr/bin/env bash
# PostToolUse hook — runs oxlint + biome format + eslint --fix on console source files.
#
# Intentionally omits tsc. Run `bun run typecheck` manually at the end of a task
# — tsc on every edit creates noise during multi-file refactors.

file_path=$(python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('tool_input', {}).get('file_path', ''))
except Exception:
    print('')
" 2>/dev/null)

# Only process TypeScript files under apps/console/src/
[[ "$file_path" == *"apps/console/src/"* ]] || exit 0
[[ "$file_path" == *.ts || "$file_path" == *.tsx ]] || exit 0

# Skip deleted files
[[ -f "$file_path" ]] || exit 0

PROJECT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$PROJECT_ROOT/apps/console" || exit 0

has_error=0

# oxlint — fast lint pass
if ! bunx oxlint "$file_path" 2>&1; then
  has_error=1
fi

# biome — auto-format in place (writes back to file, silent on no changes)
if ! bunx biome format --write "$file_path" 2>&1; then
  has_error=1
fi

# eslint --fix — auto-fix lint issues, organise imports, remove unused
if ! bunx eslint --fix "$file_path" 2>&1; then
  has_error=1
fi

exit $has_error
