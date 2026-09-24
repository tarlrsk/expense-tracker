#!/usr/bin/env bash
# SessionStart hook (ADR-0023): runs the spec check and shows the result to Claude,
# so anything a previous session missed is fixed before new work. Never blocks.
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"
CHECK="$PROJECT_DIR/../expense-tracker-spec/scripts/check-spec.sh"

if [ ! -x "$CHECK" ]; then
  echo "Spec check skipped: $CHECK not found."
  exit 0
fi
if out="$(EXPENSE_CODE_DIR="$PROJECT_DIR" "$CHECK" 2>&1)"; then
  echo "Spec check (ADR-0023): $(printf '%s' "$out" | tr '\n' ' ')"
else
  echo "Spec check (ADR-0023) found problems. Fix them, or ask the user, before starting new work:"
  printf '%s\n' "$out"
fi
exit 0
