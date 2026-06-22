/**
 * Nod Design System — Theme Tokens (TypeScript)
 *
 * Canonical token definitions for the Nod dark theme.
 * Use these in React components via inline styles or CSS-in-JS.
 */

export const colors = {
  bg: "#090D13",
  surface: "#111519",
  surfaceElevated: "#1A1E24",
  border: "#252A30",
  borderFocus: "#3A4048",

  textPrimary: "#E8EAED",
  textSecondary: "#8B9098",
  textMuted: "#505660",

  accent: "#7DFFC2",
  accentHover: "#5AE6A0",
  accentSubtle: "rgba(125, 255, 194, 0.04)",
  accentBorder: "rgba(125, 255, 194, 0.12)",

  sentMsg: "#151A20",
  receivedMsg: "#1A1E24",

  danger: "#FF6B6B",
  dangerSubtle: "rgba(255, 107, 107, 0.1)",
  dangerBorder: "rgba(255, 107, 107, 0.2)",
  warning: "#FFD93D",

  online: "#7DFFC2",
  offline: "#505660",
} as const;

export const fonts = {
  display: '"Geist", "Inter", sans-serif',
  body: '"Geist", "Inter", sans-serif',
  mono: '"Geist Mono", "JetBrains Mono", monospace',
} as const;

export const typography = {
  display: {
    fontFamily: fonts.display,
    fontWeight: 500,
    fontSize: "96px",
    letterSpacing: "-0.04em",
    lineHeight: 1,
  },
  h2: {
    fontFamily: fonts.display,
    fontWeight: 500,
    fontSize: "1.5rem",
    letterSpacing: "-0.01em",
  },
  body: {
    fontFamily: fonts.body,
    fontWeight: 400,
    fontSize: "0.95rem",
    lineHeight: 1.6,
    letterSpacing: "0.01em",
  },
  small: {
    fontFamily: fonts.body,
    fontWeight: 400,
    fontSize: "0.8rem",
    lineHeight: 1.4,
    letterSpacing: "0.02em",
  },
  mono: {
    fontFamily: fonts.mono,
    fontWeight: 400,
    fontSize: "0.85rem",
  },
  label: {
    fontFamily: fonts.mono,
    fontWeight: 400,
    fontSize: "0.75rem",
    textTransform: "uppercase" as const,
    letterSpacing: "0.08em",
  },
} as const;

export const spacing = {
  xs: "4px",
  sm: "8px",
  md: "12px",
  lg: "16px",
  xl: "24px",
  "2xl": "32px",
  "3xl": "48px",
  "4xl": "64px",
} as const;

export const radii = {
  sm: "6px",
  md: "8px",
  lg: "12px",
  pill: "999px",
} as const;

export const transitions = {
  default: "150ms ease",
  spring: "150ms cubic-bezier(0.16, 1, 0.3, 1)",
} as const;

export const layout = {
  sidebarWidth: "260px",
  headerHeight: "56px",
  panelPadding: "24px",
  bubbleGap: "8px",
  maxTextWidth: "65ch",
} as const;

/** Design rules for AI context */
export const designRules = [
  "Dark mode only. Background is near-black (#090D13), never white.",
  "Mint accent (#7DFFC2) used sparingly — max 3 instances per viewport for interactive signals.",
  "No box shadows. Depth is conveyed by background color variation only.",
  "All transitions use 150ms ease curves.",
  "Border radius: 12px cards, 8px inputs, 999px pill buttons, 6px small elements.",
  "Uppercase labels always use letter-spacing: 0.08em with Geist Mono.",
  "Film grain overlay at 2.5% opacity (optional decorative layer).",
  "Body text max-width: 65ch for readability.",
  "Contrast ratio must exceed 4.5:1 for standard visibility.",
  "Font: Geist by Vercel is the primary typeface (npm i geist).",
];
