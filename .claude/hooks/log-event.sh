#!/usr/bin/env bash
# Appends one redacted JSON line per Claude Code hook event to the spec repo.
# Never blocks Claude: always exits 0.
set -uo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"
SPEC_DIR="${EXPENSE_SPEC_DIR:-$PROJECT_DIR/../expense-tracker-spec}"
LOG_DIR="$SPEC_DIR/logs/actions"
mkdir -p "$LOG_DIR" 2>/dev/null || exit 0

INPUT="$(cat)"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
FILE="$LOG_DIR/$(date -u +%Y-%m-%d).jsonl"
REPO="$(basename "$PROJECT_DIR")"
BRANCH="$(git -C "$PROJECT_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "-")"

LINE=""
if command -v jq >/dev/null 2>&1; then
  LINE="$(printf '%s' "$INPUT" | jq -c --arg ts "$TS" --arg repo "$REPO" --arg branch "$BRANCH" '
    def cut(n): if type == "string" and length > n then .[0:n] + "…[+\(length - n) chars]" else . end;
    def slim: if type == "object"
      then with_entries(.value |= (if type == "string" then cut(400) else (tojson | cut(400)) end))
      else (tojson | cut(400)) end;
    {
      ts: $ts, repo: $repo, branch: $branch,
      event: .hook_event_name,
      session: .session_id,
      tool: .tool_name,
      input: (if .tool_input == null then null else (.tool_input | slim) end),
      result: (if .tool_response == null then null else (.tool_response | tojson | cut(400)) end),
      prompt: (if .prompt == null then null else (.prompt | cut(4000)) end)
    } | with_entries(select(.value != null))' 2>/dev/null)"
fi
[ -z "$LINE" ] && LINE="{\"ts\":\"$TS\",\"repo\":\"$REPO\",\"event\":\"unparsed\",\"note\":\"install jq for full logs\"}"

# Redact secrets before writing.
LINE="$(printf '%s' "$LINE" | sed -E \
  -e 's/sk-ant-[A-Za-z0-9_-]+/[REDACTED_ANTHROPIC_KEY]/g' \
  -e 's/sb_(secret|publishable)_[A-Za-z0-9_-]+/[REDACTED_SUPABASE_KEY]/g' \
  -e 's/eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]+/[REDACTED_JWT]/g' \
  -e 's#postgres(ql)?://[^" ]+#[REDACTED_DB_URL]#g' \
  -e 's/-----BEGIN [A-Z ]*PRIVATE KEY-----[^"]*/[REDACTED_PRIVATE_KEY]/g')"

printf '%s\n' "$LINE" >> "$FILE"
exit 0
