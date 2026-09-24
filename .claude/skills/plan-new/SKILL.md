---
name: plan-new
description: 'Draft a new plan in the spec repo. Use when the user describes new work that is not covered by an existing plan — a feature, change, refactor, migration or experiment — e.g. "I want to add…", "can we build…", "let''s plan…", "next I''d like…", or asks to break down a Draft plan into tasks.'
---
Draft a new plan (title from `$ARGUMENTS` if given, otherwise from the conversation) in `../expense-tracker-spec/plans/`.

1. Read `INDEX.md`, `plans/README.md` and `plans/0000-template.md`, then only the docs and ADRs that `INDEX.md` points to for this topic.
2. Take the next 4-digit number. File: `NNNN-<kebab-slug>.md`.
3. Fill in goal, scope, acceptance criteria (testable), tasks (each small enough for one session and one commit), dependencies, related ADRs, open questions.
4. If anything about the goal, scope or approach is unclear, ask me before writing it (options + recommendation) and record each question and answer in `questions/NNNN-<slug>.md` (ADR-0021). A decision still without an ADR goes under Open questions — do not decide it silently.
5. Status: **Draft**. Add a row to the index in `plans/README.md`, and add the plan to its topic row in `INDEX.md` (new row if it is a new topic).
6. Show me the plan and ask whether to approve it. If I approve, set status to Approved and update the index.

If the request names an existing Draft plan without tasks, refine that plan instead of creating a new one.
