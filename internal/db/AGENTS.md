# DOX Framework - Database Directory Contract

- This directory governs all database storage, schemas, migrations, and model definitions.
- Parent: [Internal AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose and Scope

This package (`db`) provides SQLite-based persistent storage for the TermTalk client.
Key responsibilities:
- Managing SQLite connection configuration (WAL mode, busy timeout).
- Schema migrations (`profile`, `contacts`, and `messages` tables).
- CRUD operations for the local user profile, contact list/discovery registry, and local chat history.
- Querying unsynced messages for outbound synchronization.

## Guidelines for Database Work

1. **SQLite Optimization**:
   - Ensure WAL mode and busy timeouts are configured on connection initialization to handle concurrency cleanly.

2. **Schema & Migrations**:
   - Schema definitions are declared inside `migrate()` in [db.go](file:///C:/Users/HP/Desktop/termtk/internal/db/db.go). Any schema modifications require updating `migrate()` and potentially existing models in [models.go](file:///C:/Users/HP/Desktop/termtk/internal/db/models.go).

3. **Message Integrity**:
   - Message IDs are SHA-256 hashes generated from the message sender, recipient, content, and timestamp using `GenerateID()`.
