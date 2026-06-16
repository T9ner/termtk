# DOX Framework — Documentation Directory Contract

- This directory governs all design documents, ADRs, CE lessons, and agent guides.
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose

Project-wide documentation for human developers and AI agents — architectural decisions, domain vocabulary, compound engineering memory, and triage workflow.

## Ownership

- [ce_lessons.md](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md): Compound Engineering lessons log — institutional memory for regression prevention
- [adr/](file:///C:/Users/HP/Desktop/termtk/docs/adr): Architecture Decision Records
- [agents/](file:///C:/Users/HP/Desktop/termtk/docs/agents): Agent-specific guides (domain vocabulary, triage labels, issue tracker schema)

## Local Contracts

- **ADRs**: Create under `docs/adr/` using `NNNN-title.md` format. Consult existing ADRs before proposing architectural shifts
- **CE Lessons**: Append new entries with unique ID (`CE-XXX`). Document Symptom → Root Cause → Code Change → Prevention Rule
- **Domain Vocabulary**: Align all code, naming, and issues with [CONTEXT.md](file:///C:/Users/HP/Desktop/termtk/CONTEXT.md) terminology (also documented in [domain.md](file:///C:/Users/HP/Desktop/termtk/docs/agents/domain.md))
- **Issue Triage**: Use canonical states from [triage-labels.md](file:///C:/Users/HP/Desktop/termtk/docs/agents/triage-labels.md). Issues and PRDs live in `.scratch/`

## Work Guidance

- CE lessons are **mandatory reading** at session start — enforced by root AGENTS.md
- ADRs are append-only. To supersede an ADR, create a new one referencing the old

## Verification

No automated verification. Manual review of doc accuracy during DOX closeout.

## Child DOX Index

- [adr/](file:///C:/Users/HP/Desktop/termtk/docs/adr) — Architecture Decision Records (0001-custom-relay-server.md)
- [agents/](file:///C:/Users/HP/Desktop/termtk/docs/agents) — domain.md, issue-tracker.md, triage-labels.md
