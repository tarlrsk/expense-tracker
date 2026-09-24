---
name: wrap-up
description: 'End-of-session wrap-up: write the journal entry, record missing decisions, sync plans and docs, commit the spec repo. Use when a plan task is finished, when the user says they are done, stopping, taking a break, "that''s it for today", "wrap up", "let''s stop here", or before the conversation ends after any code change.'
---
Wrap up this session:

1. **Questions & decisions:** make sure every question asked this session and its answer is in `questions/` (ADR-0021). List every choice made (by me or by you); for any without an ADR, create one as in `/decision`. Mark trivial choices you made alone as `claude (pending review)`.
2. **Journal:** write `../expense-tracker-spec/journal/YYYY-MM-DD-<slug>.md` (today's date, UTC) using the format in `journal/README.md`: goal, outcome, files changed, decisions (with ADR links), questions asked (links to `questions/` entries), problems and fixes, open questions, next steps.
   The journal is the only session history (ADR-0018), so build "What changed" from git, not memory:
   `git -C <repo> log --since=<session start> --stat` and `git -C <repo> status --short` in both repos.
   If earlier sessions have commits but no journal entry, add a short catch-up entry for them.
3. **Plan sync:** if this session worked on a plan, make sure its task checkboxes, progress log, status and the `plans/README.md` index are current.
4. **Spec sync:** update `docs/` if anything changed (API list in `04-api.md`, schema in `02-data-model.md`, roadmap status in `05-roadmap.md`).
5. **Index (ADR-0022):** add the journal entry to `journal/README.md` and any new questions file to `questions/README.md`; update `INDEX.md` (topic rows for new docs, ADRs or plans; the "Now" section) and **Next task** in `plans/README.md`. Run `../expense-tracker-spec/scripts/check-spec.sh` (ADR-0023) and fix every PROBLEM it reports. The journal must name each code commit by its short hash.
6. **Commit the spec repo only:**
   `git -C ../expense-tracker-spec add -A && git -C ../expense-tracker-spec commit -m "journal: <slug>"`
7. Give me a 5-line summary and list anything marked `pending review`.

Do not push. Do not make extra code-repo commits beyond those `/plan-run` already made or I asked for.
