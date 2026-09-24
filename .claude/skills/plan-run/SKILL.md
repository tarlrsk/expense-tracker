---
name: plan-run
description: 'Execute the next task of an existing plan. Use when the user asks to continue or start work on a plan or roadmap step — e.g. "next task", "continue", "let''s keep going", "do the next step of the foundation", "work on plan 0002", "start smart entry".'
---
Execute the next task of the plan (id from `$ARGUMENTS` if given; otherwise the plan the user refers to; if unclear, the In progress plan — ask if more than one) from `../expense-tracker-spec/plans/`.

1. Open the plan. If its status is Draft or Dropped, stop and tell me.
2. Check its dependencies are Done; if not, stop and tell me.
3. Pick the **first unchecked task**. Set the plan status to In progress if it was Approved.
4. Show me a short plan for that task and wait for my go-ahead.
5. Implement it, following `CLAUDE.md`. Run tests and linters.
6. Commit the code with a conventional commit message.
7. Tick the task in the plan, append a progress-log line: `YYYY-MM-DD — T<n> — <commit sha> — <one-line note>`.
8. If all tasks and acceptance criteria are done, set status to Done and update `plans/README.md`.
9. Record any decisions with `/decision`, then run `/wrap-up`.

Do only one task per run.
