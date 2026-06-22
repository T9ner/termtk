import React from "react";
import { Input } from "./Input";

export default { title: "Components/Input", component: Input };

/** Standard text input for username */
export const TextInput = () => <Input placeholder="Enter your username" label="Username" />;

/** Monospace input for relay addresses */
export const MonoInput = () => (
  <Input placeholder="relay.nod.chat:9090" label="Relay Address" mono={true} />
);

/** Password input for encryption key passphrase */
export const PasswordInput = () => (
  <Input placeholder="Enter passphrase" label="Passphrase" type="password" />
);
