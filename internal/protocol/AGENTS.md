# DOX Framework — Protocol Directory Contract

- This directory governs shared protocol types used across packages.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `protocol` defines shared wire-format types for TermTalk's relay protocol. These types are imported by both the relay server (`cmd/termtalk-relay`) and the client networking layer (`internal/network`).

## Ownership

- [frames.go](file:///C:/Users/HP/Desktop/termtk/internal/protocol/frames.go): `RelayFrame` struct — the envelope type for all relay communication (register, message, ping, pong, sync_request, sync_response)

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
