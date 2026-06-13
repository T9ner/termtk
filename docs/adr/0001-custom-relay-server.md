# ADR 0001: Hybrid Decentralized Relay Server for NAT Traversal

## Context

TermTalk is designed to work out of the box on Windows and macOS, with or without a local network connection.
However, direct peer-to-peer (P2P) communication over the public internet is blocked by default on home and mobile routers due to Network Address Translation (NAT) and firewalls.

To allow users to message each other over the internet without requiring manual configuration (VPNs, Tailscale, or router port forwarding), we need a traversal mechanism.
Heavy P2P frameworks like `libp2p` solve this but introduce a massive dependency graph that is slow to download, slow to compile, and prone to platform-specific compatibility failures.

## Decision

We will implement a lightweight **Relay Server** architecture:

1.  **Relay Protocol**: Build a clean, zero-dependency TCP signaling and relay server in Go.
2.  **Default Public Node**: The client will connect to a default public relay node when an internet connection is available (while still supporting local UDP discovery on local Wi-Fi).
3.  **Client Integration**: The `SyncManager` will connect to the relay server to register the user's UUID and listen for incoming messages.
4.  **Message Routing**:
    *   When sending a message, the client will first check if a direct local TCP connection is active.
    *   If not, it will transmit the message to the relay server to be forwarded.
    *   If the recipient is offline, the message remains queued in the local database. When the recipient reconnects, the sync protocol triggers to download all queued messages.
5.  **Decentralized Nature**: Any user can run their own relay server (by running a simple command) and configure their client to point to it via a command-line flag (`-relay`).

## Consequences

*   **Zero-Configuration**: Friends only need each other's TermTalk UUID to communicate anywhere in the world.
*   **Compile Speed**: Keeps compile times under 2 seconds and maintains a tiny binary size.
*   **Reliability**: Avoids complex NAT hole-punching failures; works behind cellular networks and corporate firewalls.
*   **Trust Model**: While the relay forwards messages, all messages are hashed for integrity. (Future phases can add end-to-end encryption so the relay cannot read message payloads).
