# DOX Framework — Protocol Directory Contract

- This directory governs shared protocol types used across packages.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `protocol` defines shared wire-format types for TermTalk's relay protocol. These types are imported by both the relay server (`cmd/termtalk-relay`) and the client networking layer (`internal/network`).

## Ownership

- [frames.go](file:///C:/Users/HP/Desktop/termtk/internal/protocol/frames.go): `RelayFrame` struct — the envelope type for all relay communication. `UserInfo` struct — user identity in search/online results. `MessageIDs` field for read_ack batches. `PublicKey`/`Signature` fields for Ed25519 signed messages. `Encrypted`/`Nonce`/`X25519PublicKey` fields for NaCl box E2E encryption (US-3)

### Frame Types

| Type | Direction | Purpose |
|------|-----------|----------|
| `register` | client→relay | Register with UUID and username |
| `registered` | relay→client | Registration acknowledgement |
| `relay` | client→relay | Route a message to a recipient |
| `msg` | relay→client | Incoming message from another peer |
| `offline` | relay→client | Recipient is not connected |
| `stored` | relay→client | Message stored for offline recipient |
| `delivered` | relay→client | Stored message was delivered |
| `search` | client→relay | Search users by username query |
| `search_result` | relay→client | List of matching users |
| `who_online` | client→relay | Request list of online users |
| `online_list` | relay→client | List of online users |
| `ping`/`pong` | both | Keepalive heartbeat |
| `read_ack` | client→relay→client | Batch read receipt for messages |
| `delete` | client→relay→client | Delete messages by ID |

## Local Contracts

- **Single Source of Truth**: `RelayFrame` must be defined only here. Do NOT duplicate in other packages
- **Wire Compatibility**: Changes to `RelayFrame` fields affect both client and relay server — update and test both sides

## Work Guidance

- Keep this package minimal — only shared protocol types, no business logic
- If new frame types or envelope fields are needed, add them here

## Verification

```bash
go build ./internal/protocol/...
```

## Child DOX Index

No children.
