import React from "react";
import "../tokens.css";

interface ChatBubbleProps {
  content: string;
  timestamp: string;
  isSent: boolean;
  senderName?: string;
}

/** A single chat message bubble. Sent messages align right with darker bg, received align left with elevated surface. */
export const ChatBubble: React.FC<ChatBubbleProps> = ({
  content,
  timestamp,
  isSent,
  senderName,
}) => {
  return (
    <div
      style={{
        display: "flex",
        justifyContent: isSent ? "flex-end" : "flex-start",
        padding: `0 var(--nod-space-lg)`,
        marginBottom: "var(--nod-space-sm)",
      }}
    >
      <div
        style={{
          maxWidth: "var(--nod-max-text-width)",
          background: isSent ? "var(--nod-sent-msg)" : "var(--nod-received-msg)",
          borderRadius: "var(--nod-radius-lg)",
          padding: "var(--nod-space-md) var(--nod-space-lg)",
          border: `1px solid ${isSent ? "var(--nod-accent-border)" : "var(--nod-border)"}`,
        }}
      >
        {!isSent && senderName && (
          <div
            style={{
              fontFamily: "var(--nod-font-mono)",
              fontSize: "0.75rem",
              color: "var(--nod-accent)",
              marginBottom: "var(--nod-space-xs)",
              letterSpacing: "0.08em",
              textTransform: "uppercase",
            }}
          >
            {senderName}
          </div>
        )}
        <div
          style={{
            fontFamily: "var(--nod-font-body)",
            fontSize: "0.9rem",
            lineHeight: 1.6,
            color: "var(--nod-text-primary)",
          }}
        >
          {content}
        </div>
        <div
          style={{
            fontFamily: "var(--nod-font-mono)",
            fontSize: "0.7rem",
            color: "var(--nod-text-muted)",
            marginTop: "var(--nod-space-xs)",
            textAlign: isSent ? "right" : "left",
          }}
        >
          {timestamp}
        </div>
      </div>
    </div>
  );
};

export default ChatBubble;
