# DOX Framework — Crypto Directory Contract

- This directory governs end-to-end encryption helpers for TermTalk.
- Parent: [internal/AGENTS.md](file:///C:/Users/HP/Desktop/termtk/internal/AGENTS.md)

## Purpose

Package `crypto` provides NaCl box (Curve25519-XSalsa20-Poly1305) encryption/decryption and Ed25519→X25519 key conversion. Used by `internal/network` to encrypt relay messages end-to-end.

## Ownership

- [crypto.go](file:///C:/Users/HP/Desktop/termtk/internal/crypto/crypto.go): `Ed25519ToX25519Private`, `Ed25519ToX25519Public`, `Encrypt`, `Decrypt`

## Local Contracts

- **Key Derivation**: X25519 private keys are derived from Ed25519 seeds via SHA-512 + clamping (RFC 7748). Public keys are derived from the X25519 private key via scalar multiplication with the basepoint
- **Wire Format**: `Encrypt` returns base64-encoded ciphertext and nonce. `Decrypt` expects the same format
- **No State**: This package is stateless — it only provides pure functions. No global variables or init() functions
- **Dependencies**: Uses `golang.org/x/crypto/nacl/box` and `golang.org/x/crypto/curve25519` (already in go.mod)

## Work Guidance

- Do NOT change the clamping constants in `Ed25519ToX25519Private` — they are per RFC 7748
- When adding new crypto functions, keep them as pure functions with no side effects

## Verification

```bash
go build ./internal/crypto/...
go test ./internal/crypto/...
```

## Child DOX Index

No children.
