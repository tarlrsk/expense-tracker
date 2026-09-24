---
name: spec
description: 'Answer questions from the spec repo knowledge base with file citations. Use when the user asks why something was decided, what the spec says, what happened in earlier sessions, what the status of a plan is, or what Claude did on a given day.'
---
Answer the question (from `$ARGUMENTS` or the conversation) using only `../expense-tracker-spec` (docs, plans, decisions, questions, journal), plus `git log` of both repos for exact file changes and commits.

Start at `INDEX.md` and open the files it points to; search further only if it doesn't cover the question. Follow the superseded map for history. If asked whether the spec is up to date, run `../expense-tracker-spec/scripts/check-spec.sh` and report the result. Cite file paths. If it is not recorded, say so plainly, and offer general advice only if clearly labelled as not a project decision. Follow superseded ADRs to the current one.
