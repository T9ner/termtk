Status: completed

# Issue 2: P2P Network Discovery over UDP

## What to build

Implement local network peer auto-discovery using UDP broadcast on port 55555. Every active TermTalk client must run a background listener that handles incoming UDP announcements and updates the local contact registry, and a background announcer that broadcasts its profile.

## Acceptance criteria

- [ ] A background UDP listener that listens on broadcast port 55555.
- [ ] A background UDP broadcaster that transmits the local profile (UUID, username, current TCP port) every X seconds.
- [ ] Parsing incoming discovery packets and updating the local database/in-memory list of active peers with their current IP/port and "last seen" time.
- [ ] Robust error handling for offline/network-disconnected states (e.g. socket bind errors do not crash the app, but log warnings and retry when network becomes active).
- [ ] Covered by unit tests using mock UDP connections.

## Blocked by

- [Issue 1: Database Setup and Local Message Store](file:///C:/Users/HP/Desktop/termtk/.scratch/termtalk/issues/01-database-store.md)
