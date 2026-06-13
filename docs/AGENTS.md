# DOX Framework - Documentation Directory Contract

- This directory governs all design documents, architectural decision records (ADRs), and agent guides.
- Parent: [Root AGENTS.md](file:///C:/Users/HP/Desktop/termtk/AGENTS.md)

## Purpose and Scope

This folder holds project-wide documentation and guides for human developers and autonomous agents.
Specifically, it indexes and structures:
- Domain terminology, glossaries, and context maps (`CONTEXT.md`, `docs/agents/domain.md`).
- Architectural Decision Records (`docs/adr/`).
- Local issue tracker status schema and triage labels (`docs/agents/issue-tracker.md`, `docs/agents/triage-labels.md`).

## Guidelines for Managing Docs

1. **Architecture Decisions (ADRs)**:
   - Create new ADRs under `docs/adr/` using standard markdown format.
   - Prior to proposing architectural shifts, consult existing ADRs to ensure compatibility or explicitly document the decision to override them.

2. **Issue Triage & States**:
   - Issue files and PRDs live in `.scratch/`.
   - Update issue files with the canonical states mapped in [triage-labels.md](file:///C:/Users/HP/Desktop/termtk/docs/agents/triage-labels.md).

3. **Domain Vocabulary**:
   - Align all code modifications, symbol naming, and issues with the terminology defined in [CONTEXT.md](file:///C:/Users/HP/Desktop/termtk/CONTEXT.md) (as documented in [domain.md](file:///C:/Users/HP/Desktop/termtk/docs/agents/domain.md)).
