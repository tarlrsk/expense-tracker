#!/usr/bin/env bash
# PreToolUse guard: stops Claude from reading or printing secret files.
# Exit 2 = block the tool call and tell Claude why (stderr).
INPUT="$(cat)"
command -v jq >/dev/null 2>&1 || { echo "guard-secrets: jq not installed, secret guard is OFF" >&2; exit 1; }

TOOL="$(printf '%s' "$INPUT" | jq -r '.tool_name // ""')"
PATH_ARG="$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // .tool_input.path // ""')"
CMD="$(printf '%s' "$INPUT" | jq -r '.tool_input.command // ""')"

SECRET_FILE='(^|/)\.env(\.local|\.production|\.development)?$|service-account[^/]*\.json$|\.pem$|(^|/)[^/]*\.key$'
SECRET_CMD='(^|[[:space:]/="])\.env(\.local|\.production|\.development)?([[:space:];|&)"]|$)|gcloud secrets versions access|(^|[;&|][[:space:]]*)(printenv|env)[[:space:]]*($|[;&|])'

if [ -n "$PATH_ARG" ] && printf '%s' "$PATH_ARG" | grep -Eq "$SECRET_FILE"; then
  echo "Blocked by guard-secrets: '$PATH_ARG' may contain secrets. Use .env.example, or ask the user to check it." >&2
  exit 2
fi
if [ "$TOOL" = "Bash" ] && [ -n "$CMD" ] && printf '%s' "$CMD" | grep -Eq "$SECRET_CMD"; then
  echo "Blocked by guard-secrets: this command could expose secrets. Use .env.example, or ask the user to run it." >&2
  exit 2
fi
exit 0
