import React from "react";
import { PeerItem } from "./PeerItem";

export default { title: "Components/PeerItem", component: PeerItem };

/** Online peer — green dot, "Online" subtitle */
export const Online = () => (
  <PeerItem name="rogue" isOnline={true} lastSeen="now" />
);

/** Offline peer — gray dot, "Last seen" subtitle */
export const Offline = () => (
  <PeerItem name="alice" isOnline={false} lastSeen="2 hours ago" />
);

/** Active (selected) peer — elevated bg, accent left border */
export const Active = () => (
  <PeerItem name="bob" isOnline={true} lastSeen="now" isActive={true} />
);

/** Peer with unread messages — shows mint badge count */
export const WithUnread = () => (
  <PeerItem name="chidi" isOnline={true} lastSeen="now" unreadCount={3} />
);

/** Offline peer with unreads — gray dot, badge still visible */
export const OfflineWithUnread = () => (
  <PeerItem name="tunde" isOnline={false} lastSeen="5 min ago" unreadCount={12} />
);
