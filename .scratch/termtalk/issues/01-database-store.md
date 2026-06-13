Status: completed

# Issue 1: Database Setup and Local Message Store

## What to build

Design and implement the persistent local database for TermTalk using SQLite (`modernc.org/sqlite`). This includes database schema creation, profile management, contact (friend) tracking, and local message storage with unique hashing for deduplication.

## Acceptance criteria

- [ ] Database initialized automatically on startup in a standard location (e.g. `~/.config/termtalk/termtalk.db` or current directory for development).
- [ ] Schema contains tables for:
  - `profile`: stores local user's UUID and username.
  - `contacts`: stores contact UUID, username, last seen timestamp, and last known IP/port.
  - `messages`: stores message ID (hash), sender UUID, recipient UUID, content, timestamp, and sync status (`draft`, `queued`, `synced`).
- [ ] Functions/methods implemented to:
  - Initialize/migrate the DB.
  - Retrieve or set local profile.
  - Add/list contacts.
  - Save messages and generate unique SHA-256 message IDs (hashing content + sender + recipient + timestamp).
  - Fetch message history for a specific contact.
- [ ] Database package is covered by automated unit tests (`go test`).

## Blocked by

None - can start immediately
