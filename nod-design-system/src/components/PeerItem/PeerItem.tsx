import React from "react";
import "../tokens.css";

interface PeerItemProps {
  name: string;
  isOnline: boolean;
  lastSeen?: string;
  unreadCount?: number;
  isActive?: boolean;
}

/** A peer/contact row in the sidebar roster. Shows online status dot, name, and optional unread badge. */
export const PeerItem: React.FC<PeerItemProps> = ({
  name,
  isOnline,
  lastSeen,
  unreadCount = 0,
  isActive = false,
}) => {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "var(--nod-space-md)",
        padding: "var(--nod-space-md) var(--nod-space-lg)",
        borderRadius: "var(--nod-radius-md)",
        background: isActive ? "var(--nod-surface-elevated)" : "transparent",
        cursor: "pointer",
        transition: "var(--nod-transition)",
        borderLeft: isActive ? "2px solid var(--nod-accent)" : "2px solid transparent",
      }}
    >
      {/* Status dot */}
      <div
        style={{
          width: "8px",
          height: "8px",
          borderRadius: "50%",
          background: isOnline ? "var(--nod-online)" : "var(--nod-offline)",
          flexShrink: 0,
        }}
      />

      {/* Name and last seen */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontFamily: "var(--nod-font-body)",
            fontSize: "0.9rem",
            color: "var(--nod-text-primary)",
            fontWeight: isActive ? 500 : 400,
          }}
        >
          {name}
        </div>
        {lastSeen && (
          <div
            style={{
              fontFamily: "var(--nod-font-mono)",
              fontSize: "0.7rem",
              color: "var(--nod-text-muted)",
            }}
          >
            {isOnline ? "Online" : `Last seen ${lastSeen}`}
          </div>
        )}
      </div>

      {/* Unread badge */}
      {unreadCount > 0 && (
        <div
          style={{
            background: "var(--nod-accent)",
            color: "var(--nod-bg)",
            fontFamily: "var(--nod-font-mono)",
            fontSize: "0.7rem",
            fontWeight: 600,
            borderRadius: "var(--nod-radius-pill)",
            padding: "2px 8px",
            minWidth: "20px",
            textAlign: "center",
          }}
        >
          {unreadCount}
        </div>
      )}
    </div>
  );
};

export default PeerItem;
