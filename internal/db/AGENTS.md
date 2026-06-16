# DOX Framework — Database Directory Contract

- This directory governs all database storage, schemas, migrations, and model definitions.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `db` provides SQLite-based persistent storage for the TermTalk client using CGO-free `github.com/ncruces/go-sqlite3`.

## Ownership

- [db.go](file:///C:/Users/HP/Desktop/termtk/internal/db/db.go): Connection config, schema migrations, CRUD operations
- [models.go](file:///C:/Users/HP/Desktop/termtk/internal/db/models.go): `Profile`, `Contact`, `Message` structs
- [sneakernet.go](file:///C:/Users/HP/Desktop/termtk/internal/db/sneakernet.go): JSON-based sync file export/import for network-free sharing

## Local Contracts

- **SQLite Config**: WAL mode + `busy_timeout=5000` + `SetMaxOpenConns(1)` on every connection
- **Schema Migrations**: Declared in `migrate()` in [db.go](file:///C:/Users/HP/Desktop/termtk/internal/db/db.go). Schema changes require updating `migrate()` and models in [models.go](file:///C:/Users/HP/Desktop/termtk/internal/db/models.go)
- **Message Integrity**: Message IDs are SHA-256 hashes generated from `sender|recipient|content|timestamp` via `GenerateID()`. Do NOT change this hash scheme without updating all sync paths
- **Query Optimization**: `GetChatHistory` uses `UNION ALL` with composite indexes — do NOT rewrite to `OR`. See [CE-001](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md)
- **Transactional Imports**: Bulk writes (sneakernet import) must be wrapped in `db.Begin()` transactions. See [CE-002](file:///C:/Users/HP/Desktop/termtk/docs/ce_lessons.md)

## Work Guidance

- Do NOT drop composite indexes `idx_messages_sender_recipient` or `idx_messages_recipient_sender`
- Do NOT execute loops updating SQLite tables without wrapping in transactions

## Verification

```bash
go test ./internal/db/...
```

## Child DOX Index

No children.
