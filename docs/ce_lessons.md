# TermTalk Compound Engineering - Lessons & AI Memory

This file serves as the institutional memory for AI agents co-developing TermTalk. It documents past bugs, architectural decisions, and performance optimizations. 

**ALL AGENTS MUST READ THIS FILE ON STARTUP TO AVOID REGRESSIONS.**

---

## Lessons Learned & Optimization Registry

### CE-001: SQLite Query Optimization (GetChatHistory)
* **Date:** 2026-06-14
* **Symptom:** Retrieving chat history via `GetChatHistory` was slow (~4.89ms for only 5,000 rows).
* **Root Cause:** 
  1. The `messages` table had no indexes on query columns (`sender_uuid`, `recipient_uuid`, `timestamp`).
  2. The SQL query utilized a logical `OR` condition (`WHERE (A AND B) OR (C AND D)`), which prevented SQLite from using index scans efficiently.
* **Code Change / Fix:**
  1. Created composite indexes:
     ```sql
     CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient ON messages(sender_uuid, recipient_uuid, timestamp);
     CREATE INDEX IF NOT EXISTS idx_messages_recipient_sender ON messages(recipient_uuid, sender_uuid, timestamp);
     ```
  2. Rewrote the SQL query using `UNION ALL`:
     ```sql
     SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
     FROM (
         SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
         FROM messages 
         WHERE sender_uuid = ? AND recipient_uuid = ?
         UNION ALL
         SELECT id, sender_uuid, recipient_uuid, content, timestamp, status 
         FROM messages 
         WHERE sender_uuid = ? AND recipient_uuid = ?
     )
     ORDER BY timestamp ASC
     ```
* **Strict Rule to Prevent Regression:**
  * Do NOT replace the `UNION ALL` query in `GetChatHistory` with a combined `OR` condition.
  * Do NOT drop the composite indexes `idx_messages_sender_recipient` or `idx_messages_recipient_sender`.

---

## How to Log a New Lesson
When completing a debugging task or a major architectural optimization:
1. Append a new entry under a unique ID (e.g., `CE-XXX`).
2. Document the **Symptom**, **Root Cause**, **Code Change**, and a **Strict Rule** to prevent regression.
3. Keep rules clear and concise so future agents can parse them in their system prompt.

---

### CE-002: Core Performance & Concurrency Tuning (Relay, DB, UI, Network)
* **Date:** 2026-06-14
* **Symptom:**
  1. Relaying messages concurrently was crashing the server or corrupting json frames due to lack of write serialization.
  2. Reconnection evicted re-registered connections instead of cleaning up old sockets.
  3. UI viewport resizing calculation caused flickering and rendering overflows.
  4. Non-atomic database writes during sync files import/export created performance degradation.
* **Root Cause:**
  1. `json.Encoder` is not thread-safe.
  2. Deferred client disconnect delete handler didn't verify that the active connection matched the closing session.
  3. Viewport heights didn't subtract headers and borders correctly.
  4. Sync operations performed loop-based singular writes instead of bulk database transactions.
* **Code Change / Fix:**
  1. Implemented client connection send lock mutexes in `cmd/termtalk-relay/main.go` and `internal/network/sync.go`.
  2. Wrapped cleanup checks in matching condition filters (`clients[client.UUID] == client`).
  3. Fixed UI viewport resizing offsets dynamically.
  4. Wrapped import/export sneakernet writes inside transactions.
* **Strict Rule to Prevent Regression:**
  * Do NOT perform concurrent writes on `json.Encoder` instances without mutex locks.
  * Always verify mapping integrity before deleting clients from maps inside deferred disconnect loops.
  * Do NOT execute loops updating SQLite tables without wrapping in `db.Begin()` transactions.

---

### CE-003: Relay Connection Close Race Condition (Double-Close Panic)
* **Date:** 2026-06-14
* **Symptom:** Run validations or tests failed intermittently with `panic: close of closed channel` inside `SyncManager.relayLoop`.
* **Root Cause:** 
  1. The relay heartbeat stop channel (`pingStop`) was closed in both the decoder goroutine (when `Decode` returned an error) and in the outer select loop (`case <-errChan` / `case <-sm.stopChan`), leading to double-closing.
  2. The test validation had a hardcoded `time.Sleep(200 * time.Millisecond)` to wait for relay connection, causing timing flakes.
* **Code Change / Fix:**
  1. Wrapped the `pingStop` channel close in a thread-safe `sync.Once` block (`closePing`) to ensure it's closed exactly once.
  2. Replaced the hardcoded sleep in `relay_test.go` with a robust polling loop checking `IsRelayOnline()`.
* **Strict Rule to Prevent Regression:**
  * Do NOT close coordination channels like `pingStop` directly in multiple concurrent routines.
  * Always use synchronization primitives like `sync.Once` or context-based cancellations (`context.Context`) for goroutine lifecycle stop signals.

---

### CE-004: Relay Server Struct Refactor (Global State → Testable RelayServer)
* **Date:** 2026-06-16
* **Symptom:** Relay server used package-level `var clients` and `var clientsMu` globals, making it impossible to test store-and-forward, search, or flush logic without real TCP listeners. Tests couldn't create isolated relay instances.
* **Root Cause:** The original relay was a flat `handleClient()` goroutine with all state in package globals — fine for a v0.1 prototype, but not testable or extensible.
* **Code Change / Fix:**
  1. Extracted all state (`clients`, `messageStore`, `userRegistry`) and logic (`RegisterClient`, `HandleRelay`, `HandleSearch`, `HandleWhoOnline`) into a `RelayServer` struct.
  2. Tests create isolated `RelayServer` instances with `net.Pipe()`/TCP loopback, enabling 8 relay tests without real network listeners.
  3. The `main()` function creates a single `RelayServer` and passes it to `handleClient()`.
* **Strict Rule to Prevent Regression:**
  * Do NOT add new relay state as package-level globals. All relay state must live inside `RelayServer`.
  * Do NOT test relay logic through real TCP listeners when `RelayServer` methods can be called directly.
  * When adding new frame types, add both the handler method AND a corresponding test.

---

### CE-005: Relay Encoder Race Condition (Connection Immediately Drops After Registration)
* **Date:** 2026-06-16
* **Symptom:** Client relay connection died within 1 second of registering. Relay logs showed `Client registered` → `Client disconnected` immediately. Search returned "No results". Messages stayed `[Queued]` forever. Client logs showed `failed to send relay sync_list: use of closed network connection`.
* **Root Cause:**
  1. `sendRelayFrame()`, `SendSearchRequest()`, `SendWhoOnline()`, and the ping goroutine all acquired `relayMu`, copied the `relayEnc` pointer, **released the lock**, then called `enc.Encode()` **without the lock**. This caused concurrent writes on `json.Encoder` which corrupted the TCP stream.
  2. `triggerRelaySyncAll()` sent `sync_list` frames through the relay, but the relay server had no handler for them. These frames were incorrectly routed as `"relay"` type, causing spurious store operations and confusing the relay.
* **Code Change / Fix:**
  1. Changed all relay encoder methods to hold `relayMu` for the **entire** `Encode()` call using `defer sm.relayMu.Unlock()`, not just for the pointer read.
  2. Removed `triggerRelaySyncAll()` from the relay connect path — `sync_list` is a P2P protocol, not relay. `drainOutbox()` handles relay message delivery.
* **Strict Rule to Prevent Regression:**
  * **NEVER** release `relayMu` between reading `relayEnc` and calling `relayEnc.Encode()`. The mutex must be held for the entire write operation.
  * When adding new methods that write to the relay, use the pattern: `sm.relayMu.Lock(); defer sm.relayMu.Unlock(); sm.relayEnc.Encode(...)`.
  * Do NOT send P2P-only frame types (sync_list, sync_request, sync_response) through the relay. Only send relay-native frame types (relay, search, who_online, ping).

---

### CE-006: Fly.io Deployment Without IP Allocation (Zero Connectivity)
* **Date:** 2026-06-17
* **Symptom:** Relay deployed successfully to Fly.io. Health checks passed. But zero clients could connect — `connected=0 registered=0` for the entire 12+ hour lifetime. Users reported TermTalk "not working" with no errors shown.
* **Root Cause:** The deployment used `fly apps create` + `fly deploy` instead of `fly launch`. The `fly launch` command automatically allocates public IPs (IPv4 + IPv6), but `fly apps create` does NOT. Without public IPs, the `.fly.dev` hostname had no DNS records and no external traffic could reach the app.
* **Code Change / Fix:**
  ```bash
  fly ips allocate-v6           # Free, dedicated
  fly ips allocate-v4 --shared  # Free, shared Anycast
  ```
  After allocation, `termtalk-relay.fly.dev` resolved to both IPv4 (`66.241.124.156`) and IPv6 (`2a09:8280:1::12c:5455:0`), and TCP connections on port 55558 succeeded immediately.
* **Strict Rule to Prevent Regression:**
  * After ANY Fly.io deployment, verify `fly ips list` shows at least one public IP.
  * After ANY Fly.io deployment, verify TCP connectivity: `fly ping` or a manual TCP socket test to the app hostname and port.
  * Prefer `fly launch` over `fly apps create` + `fly deploy` for new apps — it handles IP allocation, volume creation, and service configuration automatically.

---

### CE-007: Fly.io Shared IPv4 Cannot Route Raw TCP on Custom Ports
* **Date:** 2026-06-17
* **Symptom:** After allocating shared IPv4 and IPv6, TCP connections to `termtalk-relay.fly.dev:55558` appeared to succeed at socket level (SYN/ACK) but the relay application never received any data. Registration always returned empty response. Relay health logs showed `connected=0` despite clients connecting.
* **Root Cause:** Fly.io shared IPv4 uses Anycast and routes traffic by inspecting SNI (TLS) or Host headers (HTTP). Raw TCP on a non-standard port (55558) has no such headers, so the proxy accepts the TCP handshake but cannot route bytes to the correct app. The health check passed because it runs inside Fly's internal network, not through the public proxy.
* **Code Change / Fix:**
  1. Allocated a **dedicated IPv4** (`fly ips allocate-v4`, $2/month) which routes ALL ports directly.
  2. Released the useless shared IPv4 (`fly ips release <shared-ip>`).
  3. Added `handlers = []` in `fly.toml` to explicitly request raw TCP passthrough (no TLS termination, no proxy protocol).
* **Strict Rule to Prevent Regression:**
  * Raw TCP services on custom ports MUST use dedicated IPv4, not shared.
  * Always specify `handlers = []` in `[[services.ports]]` for raw TCP to prevent the proxy from interpreting the stream as HTTP.
  * After ANY IP change, flush local DNS cache (`ipconfig /flushdns`) and verify resolution points to the correct IP.

---

### CE-008: Multiple Fly.io Machines Split In-Memory Relay State
* **Date:** 2026-06-17
* **Symptom:** Two clients connected to the relay but could not find or message each other. Search returned no results. The relay health log on each machine showed only 0 or 1 connected client.
* **Root Cause:** Fly.io auto-created 2 machines for "high availability." Each machine ran an independent relay with its own in-memory `clients`, `userRegistry`, and `messageStore`. Client A landing on machine 1 and client B on machine 2 were completely isolated — different registries, different connection maps, different stored messages.
* **Code Change / Fix:**
  1. Scaled to exactly 1 machine: `fly scale count 1 --yes`.
  2. Added `[deploy] min_machines_running = 1` to `fly.toml` to prevent auto-scaling.
  3. Documented that multi-machine support requires relay-side persistence (v0.4.0 scope).
* **Strict Rule to Prevent Regression:**
  * Do NOT scale the relay beyond 1 machine while state is in-memory.
  * Before enabling multi-machine, implement shared state (relay SQLite on a Fly volume, or external coordination).
  * After every `fly deploy`, verify `fly scale show` reports exactly 1 machine.

---

### CE-009: sql.RawBytes Panics with QueryRow().Scan()
* **Date:** 2026-06-17
* **Symptom:** `TestProfile` panicked with `sql: RawBytes isn't allowed on Row.Scan`. Nil pointer dereference followed because profile was never returned.
* **Root Cause:** `sql.RawBytes` is only valid with `Rows.Scan()` (multiple-row iteration). When used with `QueryRow().Scan()` (single-row), the database/sql driver rejects it because RawBytes references internal driver memory that is only stable during Rows iteration.
* **Code Change / Fix:** Replaced `sql.RawBytes` with `[]byte` in `GetProfile()` and `GetContact()` (both use `QueryRow`). Left `[]byte` in `ListContacts()` too for consistency.
* **Strict Rule to Prevent Regression:**
  * NEVER use `sql.RawBytes` with `QueryRow().Scan()` — always use `[]byte`.
  * `sql.RawBytes` is only safe inside a `for rows.Next()` loop, and even then `[]byte` is simpler and preferred.

---

### CE-010: Relay Forwarding Drops Fields When Constructing New Frames
* **Date:** 2026-06-17
* **Symptom:** E2E encryption was completely non-functional. Encrypted messages arrived as empty/corrupted because the recipient had no `Encrypted`, `Nonce`, or `X25519PublicKey` fields to decrypt with.
* **Root Cause:** `HandleRelay` in the relay server constructed NEW `RelayFrame` structs for both online forwarding and offline storage, explicitly listing only `Type`, `UUID`, and `Message` fields. All crypto fields (`Encrypted`, `Nonce`, `X25519PublicKey`, `PublicKey`, `Signature`) were silently dropped.
* **Code Change / Fix:** Added all 5 crypto fields to both the online forwarding frame and the offline storage frame in `HandleRelay`.
* **Strict Rule to Prevent Regression:**
  * When forwarding relay frames, ALWAYS pass through ALL fields from the incoming frame that the recipient needs. Do NOT construct minimal new frames — explicitly list every field that should be forwarded.
  * When adding new fields to `RelayFrame`, audit every place that constructs a `RelayFrame` to determine if the new field should be forwarded.
  * Also caught: `Ctrl+V` (verify) fired from chat focus, blocking paste. Guard state-transition shortcuts with focus checks when the shortcut conflicts with standard terminal behavior.

---

### CE-011: TOCTOU Race in ICE Negotiation Initiation
* **Date:** 2026-06-18
* **Symptom:** Two goroutines could simultaneously pass the `negotiations[peerUUID]` existence check and both spawn `doInitiate` goroutines, wasting an ICE agent allocation.
* **Root Cause:** `InitiateConnection` checked `im.negotiations[peerUUID]` under the lock, released the lock, then spawned `go im.doInitiate()`. Between lock release and goroutine start, another call could pass the same check.
* **Code Change / Fix:** Reserve a placeholder entry `im.negotiations[peerUUID] = &iceNegotiation{}` under the lock before spawning the goroutine. `doInitiate` replaces the placeholder with the real negotiation entry.
* **Strict Rule to Prevent Regression:**
  * When using check-then-act patterns with goroutine spawns, always reserve the slot under the lock BEFORE spawning the goroutine. Never release the lock between the check and the action that depends on it.

---

### CE-012: Relay Stores Ephemeral ICE Signaling Frames for Offline Peers
* **Date:** 2026-06-18
* **Symptom:** ICE offer/answer frames were persisted in the relay's message store when the recipient was offline. These frames are time-sensitive — by the time the peer reconnects, the sender's ICE agent has timed out and the frames are useless dead state.
* **Root Cause:** `HandleRelay` treated all incoming frames uniformly — if the recipient was offline, the frame was stored for later delivery regardless of whether it was ephemeral signaling or a durable chat message.
* **Code Change / Fix:** Added an ephemeral frame check at the top of the offline branch in `HandleRelay`. Parses the inner frame type and skips storage for `ice_offer` and `ice_answer`, responding with an `offline` frame instead.
* **Strict Rule to Prevent Regression:**
  * When adding new relay frame types that are ephemeral (signaling, presence, typing indicators), add them to the ephemeral check in `HandleRelay` so they are NOT stored for offline recipients.
  * Only durable chat messages and read receipts should be stored for offline delivery.

### CE-013: v0.5.0 Parallel Feature Merge — Typing as Ephemeral Frame
* **Date:** 2026-06-18
* **Context:** v0.5.0 added 4 features via parallel branches: typing indicators, message reactions, ICE activation, OS notifications. All 4 branches touched `client.go`, `sync.go`, `update.go`, `model.go`, `view.go`.
* **Key Decision:** `typing` frames added to the ephemeral check in `HandleRelay` alongside `ice_offer`/`ice_answer` (CE-012 compliance). Reactions are **NOT** ephemeral — they are durable and stored for offline delivery.
* **Strict Rule to Prevent Regression:**
  * New ephemeral frame types added in v0.5.0: `typing` (joins `ice_offer`, `ice_answer`)
  * New durable frame types added in v0.5.0: `reaction` (stored for offline peers like `msg`)
  * When merging parallel feature branches, resolve conflicts by keeping ALL additive code from both sides — never drop one side silently.
