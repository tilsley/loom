#!/usr/bin/env bash
# PostToolUse hook — runs gofmt + go build on edited Go source files.
#
# go build compiles production code only (excludes *_test.go), giving fast
# feedback on type/compilation errors without test-file noise.
#
# Intentionally omits go vet and golangci-lint. Run `make lint-go` and
# `make test` manually at the end of a task.

file_path=$(python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    print(d.get('tool_input', {}).get('file_path', ''))
except Exception:
    print('')
" 2>/dev/null)

# Only process Go source files (not vendor, not generated)
[[ "$file_path" == *.go ]] || exit 0
[[ "$file_path" == *vendor/* ]] && exit 0
[[ "$file_path" == *".gen.go" ]] && exit 0

# Must be in one of the Go source trees
[[ "$file_path" == *"apps/server/"* ]] || \
[[ "$file_path" == *"apps/migrators/"* ]] || \
[[ "$file_path" == *"/pkg/"* ]] || exit 0

# Skip deleted files
[[ -f "$file_path" ]] || exit 0

PROJECT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$PROJECT_ROOT" || exit 0

has_error=0

# gofmt — auto-format in place (silent on no changes)
if ! gofmt -w "$file_path" 2>&1; then
  echo "✗ gofmt failed"
  has_error=1
fi

# go build — compile the package containing the edited file.
# Does NOT include _test.go files, so stale tests don't create noise here.
# Run `make test` at end of task to validate tests.
pkg_dir=$(dirname "$file_path")
rel_pkg="./${pkg_dir#"$PROJECT_ROOT/"}"

if ! go build "$rel_pkg" 2>&1; then
  has_error=1
fi

exit $has_error
