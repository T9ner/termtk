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
