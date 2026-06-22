import React from "react";
import { Button } from "./Button";

export default { title: "Components/Button", component: Button };

/** Default mint accent pill button */
export const Primary = () => <Button>Send Message</Button>;

/** Outlined secondary button */
export const Secondary = () => <Button variant="secondary">Cancel</Button>;

/** Ghost button for toolbar actions */
export const Ghost = () => <Button variant="ghost">Settings</Button>;

/** Danger button for destructive actions */
export const Danger = () => <Button variant="danger">Delete Chat</Button>;

/** Small button for inline actions */
export const Small = () => <Button size="sm">Copy ID</Button>;

/** Large button for onboarding flows */
export const Large = () => <Button size="lg" variant="primary">Get Started</Button>;

/** Disabled state */
export const Disabled = () => <Button disabled>Connecting…</Button>;
