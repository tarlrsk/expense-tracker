---
name: spec
description: 'Answer questions from the spec repo knowledge base with file citations. Use when the user asks why something was decided, what the spec says, what happened in earlier sessions, what the status of a plan is, or what Claude did on a given day.'
---
Answer the question (from `$ARGUMENTS` or the conversation) using only `../expense-tracker-spec` (docs, plans, decisions, journal, logs).

Search before answering. Cite file paths. If it is not recorded, say so plainly, and offer general advice only if clearly labelled as not a project decision. Follow superseded ADRs to the current one.
