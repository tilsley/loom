#!/usr/bin/env bash
# Stop hook — runs go vet + staticcheck on Go packages touched in this session.
#
# Uses `git diff --name-only` to find modified Go files, derives the unique
# packages, and blocks the agent from finishing if there are errors.
#
# stop_hook_active guard prevents infinite loops.

# Write stdin to a temp file to avoid ARG_MAX limits with large payloads
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
cat > "$tmp"

# Extract stop_hook_active — bail immediately if already in a stop loop
stop_hook_active=$(python3 -c "
import sys, json
try:
    with open(sys.argv[1]) as f:
        d = json.load(f)
    print('true' if d.get('stop_hook_active') else 'false')
except Exception:
    print('false')
" "$tmp" 2>/dev/null)

[[ "$stop_hook_active" == "true" ]] && exit 0

PROJECT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$PROJECT_ROOT" || exit 0

# Collect changed + untracked Go files, derive unique package dirs (Python for bash 3 compat)
packages=$(python3 - <<'PYEOF'
import subprocess, os, sys

def run(cmd):
    r = subprocess.run(cmd, capture_output=True, text=True)
    return r.stdout.strip().splitlines()

files = run(["git", "diff", "--name-only", "HEAD"]) + \
        run(["git", "ls-files", "--others", "--exclude-standard"])

pkgs = set()
for f in files:
    if not f.endswith(".go"):
        continue
    if "vendor/" in f or f.endswith(".gen.go"):
        continue
    pkgs.add("./" + os.path.dirname(f))

for p in sorted(pkgs):
    print(p)
PYEOF
)

[[ -z "$packages" ]] && exit 0

vet_output=""
sc_output=""

# go vet
vet_output=$(echo "$packages" | xargs go vet 2>&1)

# staticcheck
if command -v staticcheck &>/dev/null; then
  sc_output=$(echo "$packages" | xargs staticcheck 2>&1)
fi

if [[ -n "$vet_output" || -n "$sc_output" ]]; then
  python3 -c "
import json, sys

vet = sys.argv[1].strip()
sc  = sys.argv[2].strip()

sections = []
if vet:
    sections.append('go vet errors:\n' + vet)
if sc:
    sections.append('staticcheck errors:\n' + sc)

reason = 'Go quality checks failed in touched packages \u2014 please fix before finishing.\n\n'
reason += '\n\n'.join(sections)

print(json.dumps({'decision': 'block', 'reason': reason}))
" "$vet_output" "$sc_output"
fi

exit 0
