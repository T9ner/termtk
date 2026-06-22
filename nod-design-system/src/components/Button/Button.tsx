import React from "react";
import "../tokens.css";

interface ButtonProps {
  children: React.ReactNode;
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";
  onClick?: () => void;
  disabled?: boolean;
}

const variantStyles: Record<string, React.CSSProperties> = {
  primary: {
    background: "var(--nod-accent)",
    color: "var(--nod-bg)",
    border: "none",
  },
  secondary: {
    background: "transparent",
    color: "var(--nod-text-primary)",
    border: "1px solid var(--nod-border)",
  },
  ghost: {
    background: "transparent",
    color: "var(--nod-text-secondary)",
    border: "none",
  },
  danger: {
    background: "var(--nod-danger-subtle)",
    color: "var(--nod-danger)",
    border: "1px solid var(--nod-danger-border)",
  },
};

const sizeStyles: Record<string, React.CSSProperties> = {
  sm: { padding: "6px 12px", fontSize: "0.8rem" },
  md: { padding: "10px 20px", fontSize: "0.9rem" },
  lg: { padding: "14px 28px", fontSize: "1rem" },
};

/** Primary action button with pill shape. Uses Nod accent mint for primary variant. */
export const Button: React.FC<ButtonProps> = ({
  children,
  variant = "primary",
  size = "md",
  onClick,
  disabled = false,
}) => {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      style={{
        ...variantStyles[variant],
        ...sizeStyles[size],
        borderRadius: "var(--nod-radius-pill)",
        fontFamily: "var(--nod-font-body)",
        fontWeight: 500,
        cursor: disabled ? "not-allowed" : "pointer",
        opacity: disabled ? 0.4 : 1,
        transition: "var(--nod-transition)",
        letterSpacing: "0.01em",
      }}
    >
      {children}
    </button>
  );
};

export default Button;
