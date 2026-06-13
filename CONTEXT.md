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
