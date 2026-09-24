---
name: wrap-up
description: 'End-of-session wrap-up: write the journal entry, record missing decisions, sync plans and docs, commit the spec repo. Use when a plan task is finished, when the user says they are done, stopping, taking a break, "that''s it for today", "wrap up", "let''s stop here", or before the conversation ends after any code change.'
---
Wrap up this session:

1. **Decisions:** list every choice made in this session (by me or by you). For any without an ADR, create one as in `/decision`. Mark choices you made alone as `claude (pending review)`.
2. **Journal:** write `../expense-tracker-spec/journal/YYYY-MM-DD-<slug>.md` (today's date, UTC) using the format in `journal/README.md`: goal, outcome, files changed, decisions (with ADR links), problems and fixes, open questions, next steps.
3. **Plan sync:** if this session worked on a plan, make sure its task checkboxes, progress log, status and the `plans/README.md` index are current.
4. **Spec sync:** update `docs/` if anything changed (API list in `04-api.md`, schema in `02-data-model.md`, roadmap status in `05-roadmap.md`).
5. **Commit the spec repo only:**
   `git -C ../expense-tracker-spec add -A && git -C ../expense-tracker-spec commit -m "journal: <slug>"`
   (this includes today's action logs).
6. Give me a 5-line summary and list anything marked `pending review`.

Do not push. Do not make extra code-repo commits beyond those `/plan-run` already made or I asked for.
