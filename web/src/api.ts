// Nod API client — talks to the Go backend via Vite dev proxy.

const API_BASE = "";

async function request<T>(
  method: string,
  path: string,
  body?: unknown
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

// ── Types ──

export interface Profile {
  registered: boolean;
  uuid?: string;
  username?: string;
  fingerprint?: string;
  public_key?: string;
}

export interface Contact {
  uuid: string;
  username: string;
  ip: string;
  port: number;
  last_seen: string;
  public_key: string | null;
  verified: boolean;
  blocked: boolean;
  pinned: boolean;
  archived: boolean;
  online: boolean;
  unread_count: number;
}

export interface Message {
  id: string;
  sender_uuid: string;
  recipient_uuid: string;
  content: string;
  timestamp: string;
  status: string;
  encrypted: boolean;
  edited: boolean;
  reply_to?: string;
}

export interface Reaction {
  id: string;
  message_id: string;
  sender_uuid: string;
  emoji: string;
  timestamp: string;
}

// ── Profile ──

export const getProfile = () => request<Profile>("GET", "/api/profile");

export const register = (username: string) =>
  request<{ uuid: string; username: string; fingerprint: string }>(
    "POST",
    "/api/register",
    { username }
  );

export const updateProfile = (username: string) =>
  request<Profile>("PUT", "/api/profile", { username });

// ── Contacts ──

export const listContacts = () => request<Contact[]>("GET", "/api/contacts");

export const addContact = (username: string, uuid: string) =>
  request<{ status: string }>("POST", "/api/contacts", { username, uuid });

export const deleteContact = (uuid: string) =>
  request<{ status: string }>("DELETE", `/api/contacts/${uuid}`);

export const verifyContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/verify`);

// ── Messages ──

export const getMessages = (contactUUID: string, limit = 50, offset = 0) =>
  request<Message[]>("GET", `/api/contacts/${contactUUID}/messages?limit=${limit}&offset=${offset}`);

export const sendMessage = (contactUUID: string, content: string, replyTo?: string) =>
  request<{ status: string }>("POST", `/api/contacts/${contactUUID}/messages`, {
    content,
    ...(replyTo ? { reply_to: replyTo } : {}),
  });

export const getMessage = (id: string) =>
  request<Message>("GET", `/api/messages/${id}`);

export const markRead = (contactUUID: string, messageIDs: string[]) =>
  request<{ status: string }>(
    "POST",
    `/api/contacts/${contactUUID}/messages/read`,
    { message_ids: messageIDs }
  );

export const getUnreadCount = (contactUUID: string) =>
  request<{ count: number }>("GET", `/api/contacts/${contactUUID}/unread`);

export const deleteMessages = (messageIDs: string[]) =>
  request<{ status: string }>("POST", "/api/messages/delete", {
    message_ids: messageIDs,
  });

export const deleteMessagesForEveryone = (
  contactUUID: string,
  messageIDs: string[]
) =>
  request<{ status: string }>("POST", "/api/messages/delete-for-everyone", {
    contact_uuid: contactUUID,
    message_ids: messageIDs,
  });

export const editMessage = (id: string, content: string) =>
  request<{ status: string }>("PUT", `/api/messages/${id}`, { content });

// ── Reactions ──

export const reactToMessage = (
  contactUUID: string,
  messageID: string,
  emoji: string
) =>
  request<{ status: string }>(
    "POST",
    `/api/contacts/${contactUUID}/messages/${messageID}/react`,
    { emoji }
  );

export const getReactions = (contactUUID: string) =>
  request<Record<string, Reaction[]>>(
    "GET",
    `/api/contacts/${contactUUID}/reactions`
  );

// ── Typing ──

export const sendTyping = (contactUUID: string) =>
  request<{ status: string }>("POST", `/api/contacts/${contactUUID}/typing`);

// ── Peer Status ──

export const isPeerOnline = (contactUUID: string) =>
  request<{ online: boolean }>("GET", `/api/contacts/${contactUUID}/online`);

// ── Relay / Directory ──

export const searchUsers = (query: string) =>
  request<{ status: string }>("POST", "/api/search", { query });

export const fetchOnlineUsers = () =>
  request<{ status: string }>("POST", "/api/online-users");

export const fetchAllUsers = () =>
  request<{ status: string }>("POST", "/api/list-users");

// ── Settings ──

export const getNotifications = () =>
  request<{ enabled: boolean }>("GET", "/api/settings/notifications");

export const setNotifications = (enabled: boolean) =>
  request<{ status: string }>("PUT", "/api/settings/notifications", {
    enabled,
  });

// ── Block / Unblock ──

export const blockContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/block`);

export const unblockContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/unblock`);

// ── Pin / Unpin ──

export const pinContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/pin`);

export const unpinContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/unpin`);

// ── Archive / Unarchive ──

export const archiveContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/archive`);

export const unarchiveContact = (uuid: string) =>
  request<{ status: string }>("POST", `/api/contacts/${uuid}/unarchive`);

// ── Files (F11/F12: File & Image Sharing) ──

export interface Attachment {
  id: string;
  filename: string;
  mime_type: string;
  size: number;
  msg_id: string;
}

export const uploadFile = async (file: File, msgId?: string): Promise<Attachment> => {
  const formData = new FormData();
  formData.append('file', file);
  if (msgId) formData.append('msg_id', msgId);

  const res = await fetch(`${API_BASE}/api/upload`, {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
};

export const getFileUrl = (attachmentId: string) =>
  `/api/files/${attachmentId}`;

export const getContactAttachments = (contactUUID: string) =>
  request<Record<string, Attachment[]>>('GET', `/api/contacts/${contactUUID}/attachments`);

// ── Feature P7: Full-Text Search ──

export const searchMessages = (query: string, limit = 50) =>
  request<Message[]>('POST', '/api/search-messages', { query, limit });

// ── Feature P8: Data Export ──

export const exportChat = (contactUUID: string, format: 'json' | 'txt') => {
  return fetch(`/api/export`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ contact_uuid: contactUUID, format }),
  }).then(res => res.blob()).then(blob => {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `chat_export.${format}`;
    a.click();
    URL.revokeObjectURL(a.href);
  });
};
