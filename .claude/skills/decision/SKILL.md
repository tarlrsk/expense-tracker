---
name: decision
description: 'Record a decision as an ADR in the spec repo. Use whenever a choice is made in conversation about a library, schema, API shape, infrastructure, security, naming or UX flow — e.g. the user says "let''s use X", "go with option B", "we''ll do it this way", "decided", or approves a recommendation — and also when you make such a choice yourself.'
---
Record the decision (title from `$ARGUMENTS` if given, otherwise from the conversation) as an ADR in `../expense-tracker-spec/decisions/`.

1. Find the highest existing ADR number and use the next one (4 digits). File name: `NNNN-<kebab-slug>.md`.
2. Fill in `0000-template.md` from this conversation: context, the options actually considered, the decision, consequences.
3. Set **Decided by** truthfully: `user`, `claude (approved by user)`, or `claude (pending review)`.
4. If it replaces an earlier ADR, set the old one's status to `Superseded by ADR-NNNN` (change nothing else in it).
5. Add a row to `decisions/README.md`.
6. Update any affected file in `docs/` so the spec stays current.
   If the decision answers a question from `questions/`, set that entry's **Result** to this ADR.
7. Show me the ADR. Do not commit — `/wrap-up` commits the spec repo.
