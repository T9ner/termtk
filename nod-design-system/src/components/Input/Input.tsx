import React from "react";
import "../tokens.css";

interface InputProps {
  placeholder?: string;
  value?: string;
  onChange?: (value: string) => void;
  type?: "text" | "password";
  label?: string;
  mono?: boolean;
}

/** Text input field with optional label. Uses surface bg with border, mint focus ring. */
export const Input: React.FC<InputProps> = ({
  placeholder,
  value,
  onChange,
  type = "text",
  label,
  mono = false,
}) => {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--nod-space-xs)" }}>
      {label && (
        <label
          style={{
            fontFamily: "var(--nod-font-mono)",
            fontSize: "0.75rem",
            color: "var(--nod-text-muted)",
            textTransform: "uppercase",
            letterSpacing: "0.08em",
          }}
        >
          {label}
        </label>
      )}
      <input
        type={type}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        style={{
          background: "var(--nod-surface)",
          border: "1px solid var(--nod-border)",
          borderRadius: "var(--nod-radius-md)",
          padding: "var(--nod-space-md) var(--nod-space-lg)",
          fontFamily: mono ? "var(--nod-font-mono)" : "var(--nod-font-body)",
          fontSize: "0.9rem",
          color: "var(--nod-text-primary)",
          outline: "none",
          transition: "var(--nod-transition)",
          width: "100%",
        }}
      />
    </div>
  );
};

export default Input;
