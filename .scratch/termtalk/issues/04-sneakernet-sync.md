Status: completed

# Issue 4: Sneakernet Offline Sync File Export/Import

## What to build

Implement offline synchronization via files ("Sneakernet"). This allows users to write messages while completely offline, export their outbox/pending messages to a JSON file, transfer it via physical media (like a USB drive), and import/merge messages on their friend's instance.

## Acceptance criteria

- [ ] Export function: Query SQLite database for unsynced messages meant for a specific contact, serialize them to a JSON file schema (including message content, hashes, sender details, timestamp).
- [ ] Import function: Read a sync JSON file, validate the message formats and hashes, insert missing messages into the local database, and reconcile sync state.
- [ ] Verify message hashes during import to ensure messages are not tampered with.
- [ ] Deduplication: Importing the same sync file multiple times has no side effects and does not create duplicate database entries.
- [ ] Unit tests for JSON marshalling/unmarshalling, validation, and database state merging.

## Blocked by

- [Issue 1: Database Setup and Local Message Store](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/01-database-store.md)
