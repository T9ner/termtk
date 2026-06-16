# Context: TermTalk Domain Model

This document outlines the ubiquitous language and core domain concepts for TermTalk.

## Ubiquitous Language

*   **Peer**: An active, running instance of TermTalk on the local network. Peers are identified by a unique ID (UUID) and a human-readable username.
*   **Contact (Friend)**: A registered peer in the local database. Messages can only be sent to and received from contacts.
*   **Discovery**: The automated process where active peers broadcast their details (username, UUID, IP address, and port) over UDP broadcast, and register neighboring peers.
*   **Message**: A single textual communication sent from one peer to another.
    *   **Message Hash (ID)**: A SHA-256 hash of the message contents, sender UUID, recipient UUID, and timestamp. Used for unique identification and deduplication.
*   **Sync File**: A JSON document containing a list of messages. Used for offline synchronization (Sneakernet) when no direct network connection is available between two peers.
*   **TCP Sync**: The peer-to-peer sync protocol executed over TCP. When a peer comes online:
    1. A TCP connection is established.
    2. The peers exchange a list of message hashes they possess.
    3. Missing messages are requested and sent.
    4. The connection is kept open for real-time instant messaging.
*   **Outbox**: A list of messages queued locally in the database that have not yet been successfully delivered to the recipient peer.
*   **Relay**: A TCP signaling and routing server that mediates communication between peers that cannot reach each other directly (e.g. across VLANs, NATs, or campus networks). The relay runs at `termtalk-relay.fly.dev:55558` by default.
*   **Store-and-Forward**: When a message is sent to an offline recipient via the relay, the relay stores it in memory and delivers it when the recipient reconnects. The sender receives a "stored" acknowledgement; upon delivery, a "delivered" receipt is sent back.
*   **User Registry**: The relay maintains a registry of all users who have ever registered. This enables searching for users by username — like a campus directory — without needing to know their IP address or UUID.
*   **Outbox Drain**: When the client reconnects to the relay, it automatically re-sends all locally queued messages (status "queued") through the relay. This is the core "offline-first" mechanism — compose messages offline, they deliver automatically when connectivity returns.
*   **Offline-First**: The design philosophy where all data persists locally (SQLite), the app works fully without any network, and synchronisation happens automatically when a connection is restored.
