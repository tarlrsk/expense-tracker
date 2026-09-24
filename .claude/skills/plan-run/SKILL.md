---
name: plan-run
description: 'Execute the next task of an existing plan. Use when the user asks to continue or start work on a plan or roadmap step — e.g. "next task", "continue", "let''s keep going", "do the next step of the foundation", "work on plan 0002", "start smart entry".'
---
Execute the next task of the plan (id from `$ARGUMENTS` if given; otherwise the plan the user refers to; if unclear, the In progress plan — ask if more than one) from `../expense-tracker-spec/plans/`.

1. Read `INDEX.md`, then the plan. If its status is Draft or Dropped, stop and tell me.
   Read only what the plan and its topic rows in `INDEX.md` point to: its Related ADRs, the listed doc sections,
   `questions/NNNN-<plan-slug>.md` if it exists, and the latest journal entry (ADR-0022).
2. Check its dependencies are Done; if not, stop and tell me.
3. Pick the **first unchecked task**. Set the plan status to In progress if it was Approved.
4. Show me a short plan for that task, with any unclear points as questions (options + recommendation), and wait for my go-ahead.
5. Implement it, following `CLAUDE.md`. Run tests and linters. If something unclear comes up, stop and ask instead of guessing.
   Record every question and answer in `questions/NNNN-<plan-slug>.md` (ADR-0021).
6. Commit the code with a conventional commit message.
7. Tick the task in the plan, update its **Next task** in `plans/README.md`, append a progress-log line: `YYYY-MM-DD — T<n> — <commit sha> — <one-line note>`.
8. If all tasks and acceptance criteria are done, set status to Done and update `plans/README.md`.
9. Record any decisions with `/decision`, then run `/wrap-up`.

Do only one task per run.
