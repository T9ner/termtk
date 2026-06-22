# Nod Design System

## Visual Theme & Atmosphere
- Mood: cinematic_dark
- Feel: Deep, immersive, command-line heritage with premium softness — like a hacker's terminal that became luxury software
- References: Terminal-green-on-black heritage, cinematic UI (Tron, Mr. Robot), messaging apps (Signal dark mode, Telegram dark), the "Terminal" brand reference image
- Brand: "Terminal" — Communication that never breaks

## Color Palette & Roles
- Background: #090D13 (deep charcoal-black, the void)
- Surface: #111519 (dark neutral, card/panel fill)
- Surface Elevated: #1A1E24 (lifted surfaces, modals, hover states)
- Border: #252A30 (neutral gray, subtle structure lines)
- Text Primary: #E8EAED (true off-white, no color cast)
- Text Secondary: #8B9098 (neutral mid-gray)
- Text Muted: #505660 (neutral dark gray, timestamps, hints)
- Accent: #6EE8B3 (soft mint green — signature color, used sparingly)
- Accent Hover: #5AD9A0 (deeper mint on hover)
- Accent Subtle: rgba(110, 232, 179, 0.03) (barely-there accent tint for backgrounds)
- Sent Message: #151A20 (neutral dark, own messages)
- Received Message: #1A1E24 (elevated surface, others' messages)
- Online: #6EE8B3 (same as accent)
- Offline: #505660 (same as text muted)
- Danger: #FF6B6B (error/destructive red)
- Warning: #FFD93D (caution yellow)

## Typography Rules
- Display: Geist, Inter, sans-serif, 500, 96px (responsive: clamp(3rem, 10vw, 6rem)), letter-spacing: -0.04em, line-height: 1
- Body: Geist, Inter, sans-serif, 400, 0.95rem/1.6, letter-spacing: 0.01em
- Small: Geist, Inter, sans-serif, 400, 0.8rem/1.4, letter-spacing: 0.02em
- Mono: Geist Mono, JetBrains Mono, monospace, 400, 0.85rem (code blocks, IDs, timestamps)
- Font source: Google Fonts for Geist (sans + mono) and Inter
- Letter spacing: Display -0.04em, Body 0.01em, Small 0.02em, Mono 0

## Component Stylings
- Buttons: rounded-full (pill shape), accent bg (#7DFFC2), dark text (#080E18), 600 weight
- Button hover: #5AE6A0 bg, scale(1.02) transform, 150ms ease transition
- Cards: surface bg (#0F1A28), 1px border (#1A3040), 12px radius
- Card hover: elevated bg (#162233), border brightens to #2A4050
- Inputs: transparent bg, 1px border (#1A3040), 10px radius, focus → accent border
- Message bubbles: sent → #0A2A1A bg, 16px 16px 4px 16px radius; received → #162233 bg, 16px 16px 16px 4px radius
- Scrollbars: 6px wide, #1A3040 thumb, transparent track, rounded
- Avatars: 36px circle, accent border for online, surface border for offline
- Status dots: 8px circle, positioned bottom-right of avatar

## Layout Principles
- Max width: none (fills Wails desktop window)
- Grid: three-panel — sidebar (260px) | conversation list (300px) | chat area (flex)
- Section spacing: 0 (panels are flush, separated by 1px border)
- Content padding: 16px (panels), 20px (chat messages area)
- Message gap: 4px between same-sender, 16px between different senders
- Header height: 56px (app header), 52px (panel headers)
- Input area height: auto (min 52px, grows with content)

## Depth & Elevation
- Shadows: none (dark theme — depth via background color stepping, not shadows)
- Grain texture: CSS pseudo-element with noise SVG at 3% opacity, covers entire viewport
- Gradient bloom: radial-gradient from bottom-left, rgba(125, 255, 194, 0.03), creates subtle green light spill
- Borders: 1px solid #1A3040 (panel separators, card edges)
- Z-index layers: base(0), panels(1), header(10), modals(100), toasts(200)

## Do's and Don'ts
- DO use the declared color tokens exclusively
- DO maintain the grain texture overlay on all screens
- DO use the accent mint green (#7DFFC2) sparingly — max 3 focal points per view
- DO ensure all text meets WCAG AA contrast (4.5:1 minimum)
- DO animate state changes with 150ms ease transitions
- DO use the pill-shaped button style consistently
- DON'T use gradients on UI elements (only the background bloom)
- DON'T use box-shadows (depth comes from bg color stepping)
- DON'T use more than 2 font families (Geist + Inter)
- DON'T use stock photos or placeholder images
- DON'T use purple, blue, or warm-toned accents — the signature color is mint green only
- DON'T use rounded-rectangle cards with more than 12px radius

## Responsive Behavior
- Desktop primary: Wails v2 window (min 900x600)
- Compact mode (< 1000px): hide conversation list, show only sidebar + chat
- Mobile (future, < 768px): single column, bottom navigation
- Panels: fixed sidebar, scrollable conversation list, scrollable chat area
- Images in messages: max-width 300px, maintain aspect ratio, 8px radius

## Agent Prompt Guide
- Do NOT invent colors outside this palette — if unsure, use surface (#0F1A28) or border (#1A3040)
- Do NOT add box-shadows — use border and background-color stepping for depth
- Accent color (#7DFFC2) appears maximum 3 times per viewport: primary CTA, online indicators, and one highlight element
- All interactive elements need :focus-visible outline in accent color
- Message timestamps use mono font (JetBrains Mono) in muted color (#3D5A50)
- The grain texture overlay must be present on every screen
- The radial gradient bloom must be present on every screen
- Registration/onboarding screens center content vertically with the "Terminal" brand mark
- Empty states use a geometric SVG icon in text-secondary color, not emoji
- Do NOT use generic AI defaults: purple-blue gradients, vague glass cards, or decorative blobs
