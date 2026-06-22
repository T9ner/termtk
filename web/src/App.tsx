import { useState, useEffect, useRef, useCallback } from "react";
import * as api from "./api";
import type { WSEvent } from "./useWebSocket";
import { useWebSocket } from "./useWebSocket";
import React from "react";

// Feature P3: Emoji Picker Categories
const EMOJI_CATEGORIES: Record<string, string[]> = {
  '😀 Smileys': ['😀','😁','😂','🤣','😃','😄','😅','😆','😉','😊','😋','😎','😍','🥰','😘','😗','😙','😚','🙂','🤗','🤩','🤔','🤨','😐','😑','😶','🙄','😏','😣','😥','😮','🤐','😯','😪','😫','🥱','😴','😌','😛','😜','😝','🤤','😒','😓','😔','😕','🙃','🤑','😲','☹️','🙁','😖','😞','😟','😤','😢','😭','😦','😧','😨','😩','🤯','😬','😰','😱','🥵','🥶','😳','🤪','😵','🥴','😠','😡','🤬','😷','🤒','🤕','🤢','🤮','🥳','🥺','🤠','🤡','🤥','🤫','🤭','🧐','🤓'],
  '👋 Gestures': ['👋','🤚','🖐️','✋','🖖','👌','🤌','🤏','✌️','🤞','🤟','🤘','🤙','👈','👉','👆','🖕','👇','☝️','👍','👎','✊','👊','🤛','🤜','👏','🙌','👐','🤲','🤝','🙏'],
  '❤️ Hearts': ['❤️','🧡','💛','💚','💙','💜','🖤','🤍','🤎','💔','❣️','💕','💞','💓','💗','💖','💘','💝','💟'],
  '🎉 Objects': ['🎉','🎊','🎈','🎁','🏆','🏅','⚽','🏀','🎮','🎵','🎶','🔥','⭐','✨','💫','🌟','💡','💎','🔔','📌','📎','✏️','📝','💬','💭','🗯️','📱','💻','⌨️','🖥️'],
};

// UserInfo matches protocol.UserInfo from the Go relay.
interface UserInfo {
  uuid: string;
  username: string;
  online: boolean;
  last_seen?: string;
}

// Toast notification system
type ToastType = 'success' | 'error' | 'info';
interface Toast { id: number; message: string; type: ToastType; }
let toastId = 0;

function useToast() {
  const [toasts, setToasts] = React.useState<Toast[]>([]);
  const show = React.useCallback((message: string, type: ToastType = 'info') => {
    const id = ++toastId;
    setToasts(prev => [...prev, { id, message, type }]);
    setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 3000);
  }, []);
  return { toasts, show };
}

function App() {
  // ── State ──
  const [profile, setProfile] = useState<api.Profile | null>(null);
  const [contacts, setContacts] = useState<api.Contact[]>([]);
  const [activeChat, setActiveChat] = useState<string | null>(null);
  const [messages, setMessages] = useState<api.Message[]>([]);
  const [draft, setDraft] = useState("");
  const [username, setUsername] = useState("");
  const [loading, setLoading] = useState(true);
  const [typingPeers, setTypingPeers] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState("");

  // Feature F6: Chat search
  const [chatSearch, setChatSearch] = useState('');
  const [chatSearchOpen, setChatSearchOpen] = useState(false);

  // Feature F9: Last message preview per contact
  const [lastMessages, setLastMessages] = useState<Record<string, string>>({});

  const { toasts, show: showToast } = useToast();

  // Find People state
  const [showFind, setShowFind] = useState(false);
  const [findQuery, setFindQuery] = useState("");
  const [findResults, setFindResults] = useState<UserInfo[]>([]);
  const [findTab, setFindTab] = useState<"search" | "online" | "all">("online");
  const [findLoading, setFindLoading] = useState(false);

  // Feature A: Settings modal
  const [showSettings, setShowSettings] = useState(false);
  const [settingsUsername, setSettingsUsername] = useState("");
  const [notificationsEnabled, setNotificationsEnabled] = useState(false);
  const [wsConnected, _setWsConnected] = useState(true);

  // Feature B: Reactions
  const [reactions, setReactions] = useState<Record<string, api.Reaction[]>>({});
  const [reactionPickerMsgId, setReactionPickerMsgId] = useState<string | null>(null);

  // Feature C: Message context menu
  const [contextMenu, setContextMenu] = useState<{x: number, y: number, msgId: string, isMine: boolean} | null>(null);

  // Feature F1: Edit messages
  const [editingMsg, setEditingMsg] = useState<{id: string, content: string} | null>(null);

  // Feature F2: Reply to messages
  const [replyingTo, setReplyingTo] = useState<api.Message | null>(null);

  // Feature D: Contact management
  const [contactMenu, setContactMenu] = useState<string | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  // Feature E: Mobile responsive
  const [mobileShowChat, setMobileShowChat] = useState(false);

  // Feature F11/F12: File & Image sharing
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);
  const [lightboxImg, setLightboxImg] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Feature F19: Pin Conversations / Feature F20: Archive Conversations
  const [showArchived, setShowArchived] = useState(false);

  // Feature P3: Emoji Picker
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [emojiCategory, setEmojiCategory] = useState(Object.keys(EMOJI_CATEGORIES)[0]);

  // Feature P5: Drag-and-drop file upload
  const [dragOver, setDragOver] = useState(false);

  // Feature P1: Theme toggle
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('nod-theme');
    if (saved === 'light' || saved === 'dark') return saved;
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  });

  // Feature P7: Global search
  const [globalSearchResults, setGlobalSearchResults] = useState<api.Message[]>([]);
  const [globalSearchLoading, setGlobalSearchLoading] = useState(false);
  const globalSearchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const lastTypingSent = useRef<number>(0);

  // ── Load profile on mount ──
  useEffect(() => {
    api.getProfile().then((p) => {
      setProfile(p);
      if (p.registered) loadContacts();
      setLoading(false);
    }).catch(() => setLoading(false));
  }, []);

  // Feature P1: Apply theme
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('nod-theme', theme);
  }, [theme]);

  // Feature P7: Debounced global search
  useEffect(() => {
    if (globalSearchTimer.current) clearTimeout(globalSearchTimer.current);
    if (!searchQuery.trim()) {
      setGlobalSearchResults([]);
      return;
    }
    setGlobalSearchLoading(true);
    globalSearchTimer.current = setTimeout(async () => {
      try {
        const results = await api.searchMessages(searchQuery.trim());
        setGlobalSearchResults(results || []);
      } catch {
        setGlobalSearchResults([]);
      }
      setGlobalSearchLoading(false);
    }, 300);
    return () => { if (globalSearchTimer.current) clearTimeout(globalSearchTimer.current); };
  }, [searchQuery]);

  const loadContacts = useCallback(async () => {
    try {
      const c = await api.listContacts();
      setContacts(c);
    } catch (err) {
      console.error("Failed to load contacts:", err);
    }
  }, []);

  // Messages are loaded by selectChat — no separate useEffect needed
  // to avoid double-fetching.

  // ── Load reactions when chat changes ──
  const loadReactions = useCallback(async (chatUuid: string) => {
    try {
      const data = await api.getReactions(chatUuid);
      setReactions(data || {});
    } catch (err) {
      console.error("Failed to load reactions:", err);
      setReactions({});
    }
  }, []);

  // ── Scroll to bottom on new messages ──
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  // ── Close context menu on click outside ──
  useEffect(() => {
    const handleClick = () => { setContextMenu(null); setReactionPickerMsgId(null); setContactMenu(null); setShowEmojiPicker(false); };
    document.addEventListener("click", handleClick);
    return () => { document.removeEventListener("click", handleClick); };
  }, []);

  // ── Feature D4: Global keyboard shortcuts ──
  useEffect(() => {
    const handleKeyboard = (e: KeyboardEvent) => {
      // Escape: close any open modal/overlay (priority order)
      if (e.key === 'Escape') {
        if (lightboxImg) { setLightboxImg(null); return; }
        if (editingMsg) { setEditingMsg(null); return; }
        if (showFind) { setShowFind(false); return; }
        if (showSettings) { setShowSettings(false); return; }
        if (chatSearchOpen) { setChatSearchOpen(false); setChatSearch(''); return; }
        if (contextMenu) { setContextMenu(null); return; }
        // Fallback: close pickers and menus
        setReactionPickerMsgId(null); setContactMenu(null); setDeleteConfirm(null); setShowEmojiPicker(false);
        return;
      }
      // Ctrl+K: Focus sidebar search
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        document.querySelector<HTMLInputElement>('.search-input')?.focus();
      }
      // Ctrl+N: Open Find People
      if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault();
        setShowFind(true);
      }
      // Ctrl+F: Toggle chat search (only when chat is open)
      if ((e.ctrlKey || e.metaKey) && e.key === 'f' && activeChat) {
        e.preventDefault();
        setChatSearchOpen(prev => !prev);
      }
    };
    window.addEventListener('keydown', handleKeyboard);
    return () => window.removeEventListener('keydown', handleKeyboard);
  }, [lightboxImg, editingMsg, showFind, showSettings, chatSearchOpen, contextMenu, activeChat]);

  // Feature P8: Export chat
  const handleExport = async () => {
    if (!activeChat) return;
    await api.exportChat(activeChat, 'json');
    showToast('Chat exported!', 'success');
  };

  // ── WebSocket event handler ──
  const handleWSEvent = useCallback(
    (evt: WSEvent) => {
      switch (evt.type) {
        case "message_received": {
          const msg = evt.data as api.Message;
          if (
            activeChat &&
            (msg.sender_uuid === activeChat || msg.recipient_uuid === activeChat)
          ) {
            setMessages((prev) => {
              // Deduplicate: skip if message ID already in the list
              if (prev.some((m) => m.id === msg.id)) return prev;
              return [...prev, msg];
            });
          }
          // Feature F9: Update last message preview
          const previewKey = msg.sender_uuid === profile?.uuid ? msg.recipient_uuid : msg.sender_uuid;
          const previewText = msg.content.length > 40 ? msg.content.slice(0, 40) + '…' : msg.content;
          setLastMessages(prev => ({ ...prev, [previewKey]: previewText }));
          // D2: Browser notification for messages not in active chat
          if (msg.sender_uuid !== profile?.uuid && msg.sender_uuid !== activeChat) {
            if (Notification.permission === 'granted') {
              const sender = contacts.find(c => c.uuid === msg.sender_uuid);
              new Notification(sender?.username || 'New message', { body: previewText });
            } else if (Notification.permission !== 'denied') {
              Notification.requestPermission();
            }
          }
          loadContacts();
          break;
        }
        case "peer_discovered":
          loadContacts();
          break;
        case "typing": {
          const data = evt.data as { sender_uuid: string };
          setTypingPeers((prev) => new Set(prev).add(data.sender_uuid));
          const existing = typingTimers.current.get(data.sender_uuid);
          if (existing) clearTimeout(existing);
          typingTimers.current.set(
            data.sender_uuid,
            setTimeout(() => {
              setTypingPeers((prev) => {
                const next = new Set(prev);
                next.delete(data.sender_uuid);
                return next;
              });
            }, 3000)
          );
          break;
        }
        case "read_ack": {
          const data = evt.data as { sender_uuid: string; message_ids: string[] };
          setMessages((prev) =>
            prev.map((m) =>
              data.message_ids.includes(m.id) ? { ...m, status: "read" } : m
            )
          );
          break;
        }
        // Relay search/directory results
        case "search_result": {
          const users = evt.data as UserInfo[];
          setFindResults(users);
          setFindLoading(false);
          break;
        }
        case "online_list": {
          const users = evt.data as UserInfo[];
          setFindResults(users);
          setFindLoading(false);
          break;
        }
        case "user_list": {
          const users = evt.data as UserInfo[];
          setFindResults(users);
          setFindLoading(false);
          break;
        }
        case "reaction": {
          // Refresh reactions for the current chat.
          if (activeChat) {
            loadReactions(activeChat);
          }
          break;
        }
        case "ice_connected":
          // Informational — peer upgraded to direct P2P.
          loadContacts();
          break;
        default:
          break;
      }
    },
    [activeChat, loadContacts, loadReactions]
  );

  useWebSocket(handleWSEvent);

  // ── Handlers ──

  const handleRegister = async () => {
    if (!username.trim()) return;
    try {
      await api.register(username.trim());
      const p = await api.getProfile();
      setProfile(p);
      loadContacts();
    } catch (err) {
      console.error("Registration failed:", err);
      showToast('Registration failed', 'error');
    }
  };

  // Feature F11/F12: File handling
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length > 0) setPendingFiles(prev => [...prev, ...files]);
    e.target.value = ''; // Reset to allow re-selecting same file
  };

  const removePendingFile = (idx: number) => {
    setPendingFiles(prev => prev.filter((_, i) => i !== idx));
  };

  const handleSend = async () => {
    if ((!draft.trim() && pendingFiles.length === 0) || !activeChat) return;
    const content = draft.trim();
    const replyTo = replyingTo?.id;
    const filesToSend = [...pendingFiles];
    setDraft("");
    setReplyingTo(null);
    setPendingFiles([]);

    try {
      let msgContent = content;
      if (filesToSend.length > 0) {
        setUploading(true);
        const uploaded: api.Attachment[] = [];
        for (const file of filesToSend) {
          const att = await api.uploadFile(file);
          uploaded.push(att);
        }
        const fileRefs = uploaded.map(a => `[file:${a.id}:${a.filename}:${a.mime_type}:${a.size}]`).join('');
        msgContent = msgContent ? msgContent + '\n' + fileRefs : fileRefs;
        setUploading(false);
      }

      if (msgContent) {
        await api.sendMessage(activeChat, msgContent, replyTo);
        const msgs = await api.getMessages(activeChat);
        setMessages(msgs);
      }
    } catch (err) {
      setUploading(false);
      console.error("Send failed:", err);
      showToast('Failed to send', 'error');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const selectChat = async (uuid: string) => {
    setActiveChat(uuid);
    setMobileShowChat(true);
    setReactionPickerMsgId(null);
    setContextMenu(null);
    setChatSearchOpen(false);
    setChatSearch('');
    const msgs = await api.getMessages(uuid);
    setMessages(msgs);
    // Feature F9: Update last message preview from loaded messages
    if (msgs.length > 0) {
      const lastMsg = msgs[msgs.length - 1];
      const previewText = lastMsg.content.length > 40 ? lastMsg.content.slice(0, 40) + '…' : lastMsg.content;
      setLastMessages(prev => ({ ...prev, [uuid]: previewText }));
    }
    loadReactions(uuid);
    const unread = msgs
      .filter((m) => m.sender_uuid === uuid && m.status !== "read")
      .map((m) => m.id);
    if (unread.length > 0) {
      await api.markRead(uuid, unread);
      loadContacts();
    }
  };

  // ── Find People handlers ──

  const openFindPeople = () => {
    setShowFind(true);
    setFindResults([]);
    setFindQuery("");
    setFindTab("online");
    handleFetchOnline();
  };

  const handleFetchOnline = async () => {
    setFindTab("online");
    setFindLoading(true);
    setFindResults([]);
    try {
      await api.fetchOnlineUsers();
      // Results arrive via WebSocket online_list event
    } catch (err) {
      console.error("Failed to fetch online users:", err);
      setFindLoading(false);
    }
  };

  const handleFindSearch = async () => {
    if (!findQuery.trim()) return;
    setFindTab("search");
    setFindLoading(true);
    setFindResults([]);
    try {
      await api.searchUsers(findQuery.trim());
      // Results arrive via WebSocket search_result event
    } catch (err) {
      console.error("Search failed:", err);
      setFindLoading(false);
    }
  };

  // Feature G: Fetch all users
  const handleFetchAll = async () => {
    setFindTab("all");
    setFindLoading(true);
    setFindResults([]);
    try {
      await api.fetchAllUsers();
      // Results arrive via WebSocket user_list event
    } catch (err) {
      console.error("Failed to fetch all users:", err);
      setFindLoading(false);
    }
  };

  const handleAddUser = async (user: UserInfo) => {
    try {
      await api.addContact(user.username, user.uuid);
      await loadContacts();
      showToast('Contact added', 'success');
    } catch (err) {
      console.error("Add contact failed:", err);
      showToast('Failed to add contact', 'error');
    }
  };

  const isAlreadyContact = (uuid: string) =>
    contacts.some((c) => c.uuid === uuid);

  const activeContact = contacts.find((c) => c.uuid === activeChat);

  const filteredContacts = contacts
    .filter((c) => c.username.toLowerCase().includes(searchQuery.toLowerCase()))
    .sort((a, b) => {
      // Pinned contacts first
      if (a.pinned && !b.pinned) return -1;
      if (!a.pinned && b.pinned) return 1;
      return 0;
    });

  // Feature F20: Split contacts into active and archived
  const activeContacts = filteredContacts.filter(c => !c.archived);
  const archivedContacts = filteredContacts.filter(c => c.archived);

  const formatTime = (ts: string) => {
    // Guard against Go's zero-value time (0001-01-01T00:00:00Z).
    if (!ts || ts.startsWith("0001")) return "Never";
    const d = new Date(ts);
    if (isNaN(d.getTime())) return "Unknown";
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  };

  // Feature F7: Date divider formatter
  const formatDateDivider = (ts: string) => {
    const d = new Date(ts);
    const today = new Date();
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    if (d.toDateString() === today.toDateString()) return 'Today';
    if (d.toDateString() === yesterday.toDateString()) return 'Yesterday';
    return d.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });
  };

  // Feature F6: Highlight matching search text
  const highlightText = (text: string, query: string) => {
    if (!query) return text;
    const parts = text.split(new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'));
    return parts.map((part, i) =>
      part.toLowerCase() === query.toLowerCase() ? <mark key={i}>{part}</mark> : part
    );
  };

  // Feature F5 + F11/F12: Render message content with clickable URLs, search highlighting, and file markers
  const isImageMime = (mime: string) => mime.startsWith('image/');

  const renderMessageContent = (text: string, searchQuery: string) => {
    // First extract file markers so they're rendered as components
    const filePattern = /\[file:([a-f0-9]+):([^:]+):([^:]+):(\d+)\]/g;
    const segments: React.ReactNode[] = [];
    let lastIndex = 0;
    let match;

    while ((match = filePattern.exec(text)) !== null) {
      // Add text before this match
      if (match.index > lastIndex) {
        const textBefore = text.substring(lastIndex, match.index);
        segments.push(<React.Fragment key={`t-${lastIndex}`}>{renderTextSegment(textBefore, searchQuery, lastIndex)}</React.Fragment>);
      }

      const [, fileId, fileName, mimeType, fileSize] = match;
      const fileUrl = api.getFileUrl(fileId);

      if (isImageMime(mimeType)) {
        segments.push(
          <div key={`f-${fileId}`} className="msg-image-wrap">
            <img
              src={fileUrl}
              alt={fileName}
              className="msg-image"
              onClick={() => setLightboxImg(fileUrl)}
            />
          </div>
        );
      } else {
        const size = Number(fileSize);
        const sizeStr = size > 1024 * 1024
          ? `${(size / (1024 * 1024)).toFixed(1)} MB`
          : `${(size / 1024).toFixed(1)} KB`;
        segments.push(
          <a key={`f-${fileId}`} href={fileUrl} download={fileName} className="msg-file-card">
            <span className="msg-file-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg></span>
            <span className="msg-file-info">
              <span className="msg-file-name">{fileName}</span>
              <span className="msg-file-size">{sizeStr}</span>
            </span>
          </a>
        );
      }

      lastIndex = match.index + match[0].length;
    }

    // Remaining text after last file marker
    if (lastIndex < text.length) {
      const remaining = text.substring(lastIndex);
      segments.push(<React.Fragment key={`t-${lastIndex}`}>{renderTextSegment(remaining, searchQuery, lastIndex)}</React.Fragment>);
    }

    if (segments.length === 0) {
      return renderTextSegment(text, searchQuery, 0);
    }
    return <>{segments}</>;
  };

  // Feature P4: Parse markdown-like formatting into React nodes
  const parseFormatting = (text: string, keyPrefix: string): React.ReactNode[] => {
    // Order matters: code blocks first, then inline code, then bold, then italic
    const formatRegex = /(```[\s\S]*?```|`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*|_[^_]+_)/g;
    const parts = text.split(formatRegex);
    return parts.map((part, i) => {
      const key = `${keyPrefix}-fmt-${i}`;
      if (part.startsWith('```') && part.endsWith('```')) {
        return <pre key={key}><code>{part.slice(3, -3).replace(/^\n/, '')}</code></pre>;
      }
      if (part.startsWith('`') && part.endsWith('`') && part.length > 2) {
        return <code key={key}>{part.slice(1, -1)}</code>;
      }
      if (part.startsWith('**') && part.endsWith('**') && part.length > 4) {
        return <strong key={key}>{part.slice(2, -2)}</strong>;
      }
      if ((part.startsWith('*') && part.endsWith('*') && part.length > 2 && !part.startsWith('**'))) {
        return <strong key={key}>{part.slice(1, -1)}</strong>;
      }
      if (part.startsWith('_') && part.endsWith('_') && part.length > 2) {
        return <em key={key}>{part.slice(1, -1)}</em>;
      }
      return part;
    });
  };

  // Helper: render a text segment with URL detection, markdown formatting, and search highlighting (original F5 logic + P4)
  const renderTextSegment = (text: string, sq: string, baseKey: number) => {
    const urlRegex = /(https?:\/\/[^\s]+)/gi;
    const parts = text.split(urlRegex);

    return parts.map((part, i) => {
      if (/^https?:\/\//i.test(part)) {
        return (
          <a key={`${baseKey}-${i}`} href={part} target="_blank" rel="noopener noreferrer" className="msg-link">
            {part}
          </a>
        );
      }
      // Apply markdown formatting first, then search highlighting
      const formatted = parseFormatting(part, `${baseKey}-${i}`);
      if (sq) {
        // Apply search highlighting to plain string segments only
        return <React.Fragment key={`${baseKey}-${i}`}>{formatted.map((node, j) => {
          if (typeof node === 'string') return <React.Fragment key={`hl-${j}`}>{highlightText(node, sq)}</React.Fragment>;
          return node;
        })}</React.Fragment>;
      }
      return <React.Fragment key={`${baseKey}-${i}`}>{formatted}</React.Fragment>;
    });
  };

  // ── Feature A: Settings handlers ──
  const openSettings = async () => {
    setShowSettings(true);
    setSettingsUsername(profile?.username || "");
    try {
      const n = await api.getNotifications();
      setNotificationsEnabled(n.enabled);
    } catch {
      setNotificationsEnabled(false);
    }
  };

  const handleSaveUsername = async () => {
    if (!settingsUsername.trim()) return;
    try {
      const updated = await api.updateProfile(settingsUsername.trim());
      setProfile(updated);
      showToast('Username updated', 'success');
    } catch {
      showToast('Failed to update username', 'error');
    }
  };

  const handleToggleNotifications = async (enabled: boolean) => {
    setNotificationsEnabled(enabled);
    try {
      await api.setNotifications(enabled);
      showToast(enabled ? 'Notifications enabled' : 'Notifications disabled', 'success');
    } catch {
      setNotificationsEnabled(!enabled);
      showToast('Failed to update notifications', 'error');
    }
  };

  // ── Feature B: Reaction handlers ──
  const REACTION_EMOJIS = ['👍', '❤️', '😂', '😮', '😢', '🔥'];

  const handleReact = async (messageId: string, emoji: string) => {
    if (!activeChat) return;
    setReactionPickerMsgId(null);
    try {
      await api.reactToMessage(activeChat, messageId, emoji);
      loadReactions(activeChat);
    } catch {
      showToast('Failed to add reaction', 'error');
    }
  };

  const groupReactions = (msgReactions: api.Reaction[]) => {
    const grouped: Record<string, number> = {};
    for (const r of msgReactions) {
      grouped[r.emoji] = (grouped[r.emoji] || 0) + 1;
    }
    return grouped;
  };

  // ── Feature C: Message deletion handlers ──
  const handleMessageContextMenu = (e: React.MouseEvent, msgId: string, isMine: boolean) => {
    e.preventDefault();
    e.stopPropagation();
    const x = Math.min(e.clientX, window.innerWidth - 180);
    const y = Math.min(e.clientY, window.innerHeight - 100);
    setContextMenu({ x, y, msgId, isMine });
  };

  const handleDeleteForMe = async () => {
    if (!contextMenu) return;
    const msgId = contextMenu.msgId;
    setContextMenu(null);
    try {
      await api.deleteMessages([msgId]);
      setMessages(prev => prev.filter(m => m.id !== msgId));
      showToast('Message deleted', 'success');
    } catch {
      showToast('Failed to delete message', 'error');
    }
  };

  const handleDeleteForEveryone = async () => {
    if (!contextMenu || !activeChat) return;
    const msgId = contextMenu.msgId;
    setContextMenu(null);
    try {
      await api.deleteMessagesForEveryone(activeChat, [msgId]);
      setMessages(prev => prev.filter(m => m.id !== msgId));
      showToast('Message deleted for everyone', 'success');
    } catch {
      showToast('Failed to delete message', 'error');
    }
  };

  // ── Feature D: Contact deletion handler ──
  const handleDeleteContact = async (uuid: string) => {
    setDeleteConfirm(null);
    setContactMenu(null);
    try {
      await api.deleteContact(uuid);
      await loadContacts();
      if (activeChat === uuid) {
        setActiveChat(null);
        setMessages([]);
      }
      showToast('Contact deleted', 'success');
    } catch {
      showToast('Failed to delete contact', 'error');
    }
  };

  // Feature F17: Block/unblock contact handler
  const handleBlockContact = async (uuid: string, block: boolean) => {
    setContactMenu(null);
    try {
      if (block) {
        await api.blockContact(uuid);
      } else {
        await api.unblockContact(uuid);
      }
      setContacts(prev => prev.map(c => c.uuid === uuid ? {...c, blocked: block} : c));
      showToast(block ? 'User blocked' : 'User unblocked', 'success');
    } catch {
      showToast('Failed to update block status', 'error');
    }
  };

  // Feature F19: Pin/Unpin contact handler
  const handlePinContact = async (uuid: string, pin: boolean) => {
    setContactMenu(null);
    try {
      if (pin) await api.pinContact(uuid); else await api.unpinContact(uuid);
      setContacts(prev => prev.map(c => c.uuid === uuid ? {...c, pinned: pin} : c));
      showToast(pin ? 'Pinned' : 'Unpinned', 'success');
    } catch {
      showToast('Failed to update pin status', 'error');
    }
  };

  // Feature F20: Archive/Unarchive contact handler
  const handleArchiveContact = async (uuid: string, archive: boolean) => {
    setContactMenu(null);
    try {
      if (archive) await api.archiveContact(uuid); else await api.unarchiveContact(uuid);
      setContacts(prev => prev.map(c => c.uuid === uuid ? {...c, archived: archive} : c));
      showToast(archive ? 'Archived' : 'Unarchived', 'success');
    } catch {
      showToast('Failed to update archive status', 'error');
    }
  };

  // ── Render ──

  if (loading) {
    return (
      <div className="empty-state">
        <div className="empty-state-text">Connecting…</div>
      </div>
    );
  }

  if (!profile?.registered) {
    return (
      <div className="register-screen">
        <div className="register-card">
          <div className="register-title">Nod</div>
          <div className="register-subtitle">Communication that never breaks</div>
          <input
            className="register-input"
            placeholder="Choose a username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleRegister()}
            autoFocus
          />
          <button
            className="btn-register"
            onClick={handleRegister}
            disabled={!username.trim()}
          >
            Get Started
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className={`app-layout${mobileShowChat ? ' mobile-chat' : ''}`}>
      <div className="toast-container">
        {toasts.map(t => (
          <div key={t.id} className={`toast ${t.type}`}>{t.message}</div>
        ))}
      </div>
      {/* Sidebar */}
      <div className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-header-row">
            <div className="sidebar-brand">Nod</div>
            <div style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
              <button className="btn-theme-sidebar" onClick={() => setTheme(t => t === 'dark' ? 'light' : 'dark')} title="Toggle theme">
                {theme === 'dark' ? <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg> : <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>}
              </button>
              <button className="btn-settings" onClick={openSettings} title="Settings">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="3" />
                  <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                </svg>
              </button>
            </div>
          </div>
          <div className="sidebar-subtitle">{profile.username}</div>
        </div>

        <div className="sidebar-actions">
          <button className="btn-find-people" onClick={openFindPeople}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            Find People
          </button>
        </div>

        <div className="sidebar-search">
          <input
            className="search-input"
            placeholder="Filter contacts…"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>

        {/* Feature P7: Global search results */}
        {searchQuery.trim() && (
          <div className="global-search-results">
            <div className="global-search-results-header">
              {globalSearchLoading ? 'Searching messages…' : `${globalSearchResults.length} message${globalSearchResults.length !== 1 ? 's' : ''} found`}
            </div>
            {globalSearchResults.slice(0, 20).map(msg => (
              <div
                key={msg.id}
                className="global-search-item"
                onClick={() => {
                  const chatUuid = msg.sender_uuid === profile?.uuid ? msg.recipient_uuid : msg.sender_uuid;
                  selectChat(chatUuid);
                }}
              >
                <div className="global-search-label">
                  {contacts.find(c => c.uuid === msg.sender_uuid)?.username || msg.sender_uuid.slice(0, 8)}
                  {' · '}
                  {formatTime(msg.timestamp)}
                </div>
                <div className="global-search-text">
                  {msg.content.length > 80 ? msg.content.slice(0, 80) + '…' : msg.content}
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="contact-list">
          {activeContacts.length === 0 && archivedContacts.length === 0 ? (
            <div className="empty-state" style={{ padding: "40px 0" }}>
              <div className="empty-state-text">No contacts yet</div>
            </div>
          ) : (
            <>
            {activeContacts.map((contact) => (
              <div
                key={contact.uuid}
                className={`contact-item ${activeChat === contact.uuid ? "active" : ""}`}
                onClick={() => selectChat(contact.uuid)}
              >
                <div className="contact-avatar">
                  {contact.username.charAt(0).toUpperCase()}
                  <div
                    className={`contact-status ${contact.online ? "online" : "offline"}`}
                  />
                </div>
                <div className="contact-info">
                  <div className="contact-name">{contact.pinned && <svg className="pin-indicator" width="12" height="12" viewBox="0 0 24 24" fill="currentColor"><path d="M16 12V4h1V2H7v2h1v8l-2 2v2h5v6l1 1 1-1v-6h5v-2l-2-2z"/></svg>}{contact.username}</div>
                  <div className="contact-preview">
                    {lastMessages[contact.uuid] || (contact.online ? "Online" : `Last seen ${formatTime(contact.last_seen)}`)}
                  </div>
                </div>
                <div className="contact-meta">
                  {contact.unread_count > 0 && (
                    <div className="badge-unread">{contact.unread_count}</div>
                  )}
                  <button
                    className="btn-contact-menu"
                    onClick={(e) => {
                      e.stopPropagation();
                      setContactMenu(contactMenu === contact.uuid ? null : contact.uuid);
                    }}
                    title="Contact options"
                  >
                    ⋮
                  </button>
                  {contactMenu === contact.uuid && (
                    <div className="contact-dropdown" onClick={(e) => e.stopPropagation()}>
                      <button
                        className="contact-dropdown-item"
                        onClick={() => handlePinContact(contact.uuid, !contact.pinned)}
                      >
                        {contact.pinned ? 'Unpin' : 'Pin'}
                      </button>
                      <button
                        className="contact-dropdown-item"
                        onClick={() => handleArchiveContact(contact.uuid, !contact.archived)}
                      >
                        {contact.archived ? 'Unarchive' : 'Archive'}
                      </button>
                      <button
                        className="contact-dropdown-item"
                        onClick={() => handleBlockContact(contact.uuid, !contact.blocked)}
                      >
                        {contact.blocked ? 'Unblock' : 'Block'}
                      </button>
                      <button
                        className="contact-dropdown-item danger"
                        onClick={() => setDeleteConfirm(contact.uuid)}
                      >
                        Delete contact
                      </button>
                    </div>
                  )}
                </div>
              </div>
            ))}
            {archivedContacts.length > 0 && (
              <div className="archived-section">
                <button className="archived-toggle" onClick={() => setShowArchived(p => !p)}>
                  Archived ({archivedContacts.length})
                </button>
                {showArchived && archivedContacts.map((contact) => (
                  <div
                    key={contact.uuid}
                    className={`contact-item ${activeChat === contact.uuid ? "active" : ""}`}
                    onClick={() => selectChat(contact.uuid)}
                  >
                    <div className="contact-avatar">
                      {contact.username.charAt(0).toUpperCase()}
                      <div
                        className={`contact-status ${contact.online ? "online" : "offline"}`}
                      />
                    </div>
                    <div className="contact-info">
                      <div className="contact-name">{contact.username}</div>
                      <div className="contact-preview">
                        {lastMessages[contact.uuid] || (contact.online ? "Online" : `Last seen ${formatTime(contact.last_seen)}`)}
                      </div>
                    </div>
                    <div className="contact-meta">
                      {contact.unread_count > 0 && (
                        <div className="badge-unread">{contact.unread_count}</div>
                      )}
                      <button
                        className="btn-contact-menu"
                        onClick={(e) => {
                          e.stopPropagation();
                          setContactMenu(contactMenu === contact.uuid ? null : contact.uuid);
                        }}
                        title="Contact options"
                      >
                        ⋮
                      </button>
                      {contactMenu === contact.uuid && (
                        <div className="contact-dropdown" onClick={(e) => e.stopPropagation()}>
                          <button
                            className="contact-dropdown-item"
                            onClick={() => handleArchiveContact(contact.uuid, false)}
                          >
                            Unarchive
                          </button>
                          <button
                            className="contact-dropdown-item"
                            onClick={() => handleBlockContact(contact.uuid, !contact.blocked)}
                          >
                            {contact.blocked ? 'Unblock' : 'Block'}
                          </button>
                          <button
                            className="contact-dropdown-item danger"
                            onClick={() => setDeleteConfirm(contact.uuid)}
                          >
                            Delete contact
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
            </>
          )}
        </div>
      </div>

      {/* Chat Area */}
      <div
        className="chat-area"
        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragOver(false);
          const files = Array.from(e.dataTransfer.files);
          if (files.length > 0) setPendingFiles(prev => [...prev, ...files]);
        }}
      >
        {!activeChat ? (
          <div className="empty-state">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" style={{ opacity: 0.2 }}>
              <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
            </svg>
            <div className="empty-state-text">Select a conversation</div>
          </div>
        ) : (
          <>
            <div className="chat-header">
              <button
                className="btn-back"
                onClick={() => { setMobileShowChat(false); setActiveChat(null); }}
                title="Back to contacts"
              >
                ← Back
              </button>
              <div className="contact-avatar" style={{ width: 32, height: 32, fontSize: "0.75rem" }}>
                {activeContact?.username.charAt(0).toUpperCase()}
              </div>
              <div>
                <div className="chat-header-name">
                  {activeContact?.username}
                  {activeContact?.blocked && <span className="blocked-badge">Blocked</span>}
                </div>
                <div
                  className={`chat-header-status ${activeContact?.online ? "is-online" : ""}`}
                >
                  {typingPeers.has(activeChat)
                    ? "typing…"
                    : activeContact?.online
                      ? "Online"
                      : "Offline"}
                </div>
              </div>
              <button className="export-btn" onClick={handleExport} title="Export chat">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
              </button>
            </div>

            {chatSearchOpen && (
              <div className="chat-search-bar">
                <input
                  className="chat-search-input"
                  placeholder="Search messages…"
                  value={chatSearch}
                  onChange={(e) => setChatSearch(e.target.value)}
                  autoFocus
                />
                <span className="chat-search-count">
                  {chatSearch ? `${messages.filter(m => m.content.toLowerCase().includes(chatSearch.toLowerCase())).length} matches` : ''}
                </span>
                <button className="chat-search-close" onClick={() => { setChatSearchOpen(false); setChatSearch(''); }}>✕</button>
              </div>
            )}

            <div className="chat-messages">
              {messages.map((msg, idx) => {
                const isSent = msg.sender_uuid === profile.uuid;
                const msgReactions = reactions[msg.id] || [];
                const grouped = groupReactions(msgReactions);
                // Feature F7: Date divider
                let dateDivider: React.ReactNode = null;
                const msgDate = new Date(msg.timestamp).toDateString();
                const prevDate = idx > 0 ? new Date(messages[idx - 1].timestamp).toDateString() : null;
                if (idx === 0 || msgDate !== prevDate) {
                  const dateLabel = formatDateDivider(msg.timestamp);
                  dateDivider = <div className="date-divider"><span>{dateLabel}</span></div>;
                }
                return (
                  <React.Fragment key={msg.id}>
                    {dateDivider}
                    <div
                      className={`message-row ${isSent ? "sent" : "received"}`}
                      onContextMenu={(e) => handleMessageContextMenu(e, msg.id, isSent)}
                      id={`msg-${msg.id}`}
                    >
                      <div className="message-bubble-wrap">
                        <div className="message-bubble">
                          {!isSent && (
                            <div className="message-sender">
                              {activeContact?.username}
                            </div>
                          )}
                          {msg.reply_to && (
                            <div className="msg-reply-quote" onClick={() => {
                              const el = document.getElementById(`msg-${msg.reply_to}`);
                              if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
                            }}>
                              <span className="msg-reply-text">
                                {messages.find(m => m.id === msg.reply_to)?.content?.substring(0, 50) || 'Original message'}
                              </span>
                            </div>
                          )}
                          <div>{renderMessageContent(msg.content, chatSearch)}</div>
                        <div className="message-time">
                          {formatTime(msg.timestamp)}
                          {msg.encrypted && <span className="encrypted-badge" title="End-to-end encrypted"><svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg></span>}
                          {msg.edited && <span className="msg-edited">(edited)</span>}
                          {isSent && (
                            <span className={`msg-status ${msg.status === 'read' ? 'read' : 'pending'}`}>
                              {msg.status === 'queued' ? ' ○' : msg.status === 'read' ? ' ✓✓' : ' ✓'}
                            </span>
                          )}
                        </div>
                        <button
                          className="btn-react-trigger"
                          onClick={(e) => {
                            e.stopPropagation();
                            setReactionPickerMsgId(reactionPickerMsgId === msg.id ? null : msg.id);
                          }}
                          title="React"
                        >
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><line x1="9" y1="9" x2="9.01" y2="9"/><line x1="15" y1="9" x2="15.01" y2="9"/></svg>
                        </button>
                      </div>
                      {reactionPickerMsgId === msg.id && (
                        <div className="reaction-picker" onClick={(e) => e.stopPropagation()}>
                          {REACTION_EMOJIS.map((emoji) => (
                            <button
                              key={emoji}
                              className="reaction-picker-btn"
                              onClick={() => handleReact(msg.id, emoji)}
                            >
                              {emoji}
                            </button>
                          ))}
                        </div>
                      )}
                      {Object.keys(grouped).length > 0 && (
                        <div className="reaction-pills">
                          {Object.entries(grouped).map(([emoji, count]) => (
                            <span key={emoji} className="reaction-pill" onClick={() => handleReact(msg.id, emoji)}>
                              {emoji} {count}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                  </React.Fragment>
                );
              })}
              <div ref={messagesEndRef} />
            </div>

            <div className="typing-indicator" style={{ padding: "0 24px" }}>
              {typingPeers.has(activeChat) && `${activeContact?.username} is typing…`}
            </div>

            {uploading && <div className="upload-progress">Uploading files…</div>}

            {activeContact?.blocked ? (
              <div className="compose-blocked">You blocked this contact</div>
            ) : (
            <div className="compose-bar">
              {replyingTo && (
                <div className="reply-preview">
                  <div className="reply-preview-content">
                    <span className="reply-preview-label">Replying to</span>
                    <span className="reply-preview-text">{replyingTo.content.substring(0, 60)}{replyingTo.content.length > 60 ? '…' : ''}</span>
                  </div>
                  <button className="reply-preview-close" onClick={() => setReplyingTo(null)}>✕</button>
                </div>
              )}
              {pendingFiles.length > 0 && (
                <div className="pending-files">
                  {pendingFiles.map((f, i) => (
                    <div key={i} className="pending-file">
                      {f.type.startsWith('image/') ? (
                        <img src={URL.createObjectURL(f)} alt={f.name} className="pending-file-thumb" />
                      ) : (
                        <span className="pending-file-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg></span>
                      )}
                      <span className="pending-file-name">{f.name}</span>
                      <button className="pending-file-remove" onClick={() => removePendingFile(i)}>✕</button>
                    </div>
                  ))}
                </div>
              )}
              <button className="compose-attach" onClick={() => fileInputRef.current?.click()} title="Attach file">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
              </button>
              <input
                ref={fileInputRef}
                type="file"
                multiple
                style={{ display: 'none' }}
                onChange={handleFileSelect}
                accept="image/*,.pdf,.doc,.docx,.txt,.zip,.mp3,.mp4"
              />
              <input
                className="compose-input"
                placeholder="Type a message…"
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value);
                  const now = Date.now();
                  if (now - lastTypingSent.current > 2500) {
                    lastTypingSent.current = now;
                    api.sendTyping(activeChat).catch(() => {});
                  }
                }}
                onKeyDown={handleKeyDown}
                autoFocus
              />
              <div style={{ position: 'relative' }}>
                <button className="compose-emoji" onClick={(e) => { e.stopPropagation(); setShowEmojiPicker(!showEmojiPicker); }} title="Emoji">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><line x1="9" y1="9" x2="9.01" y2="9"/><line x1="15" y1="9" x2="15.01" y2="9"/></svg>
                </button>
                {showEmojiPicker && (
                  <div className="emoji-picker" onClick={(e) => e.stopPropagation()}>
                    <div className="emoji-picker-tabs">
                      {Object.keys(EMOJI_CATEGORIES).map(cat => (
                        <button key={cat} className={`emoji-tab ${emojiCategory === cat ? 'active' : ''}`}
                          onClick={() => setEmojiCategory(cat)}>{cat.split(' ')[0]}</button>
                      ))}
                    </div>
                    <div className="emoji-grid">
                      {EMOJI_CATEGORIES[emojiCategory].map(emoji => (
                        <button key={emoji} className="emoji-item" onClick={() => {
                          setDraft(prev => prev + emoji);
                          setShowEmojiPicker(false);
                        }}>{emoji}</button>
                      ))}
                    </div>
                  </div>
                )}
              </div>
              <button
                className="btn-send"
                onClick={handleSend}
                disabled={!draft.trim() && pendingFiles.length === 0}
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="22" y1="2" x2="11" y2="13" />
                  <polygon points="22 2 15 22 11 13 2 9 22 2" />
                </svg>
              </button>
            </div>
            )}
          </>
        )}
        {/* Feature P5: Drag-and-drop overlay */}
        {dragOver && (
          <div className="drag-overlay">
            <div className="drag-overlay-content">
              <span className="drag-icon"><svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg></span>
              <span>Drop files here</span>
            </div>
          </div>
        )}
      </div>

      {/* Find People Modal */}
      {showFind && (
        <div className="find-overlay" onClick={() => setShowFind(false)}>
          <div className="find-panel" onClick={(e) => e.stopPropagation()}>
            <div className="find-header">
              <div className="find-title">Find People</div>
              <button className="btn-close" onClick={() => setShowFind(false)}>✕</button>
            </div>

            <div className="find-search">
              <input
                placeholder="Search by username…"
                value={findQuery}
                onChange={(e) => setFindQuery(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleFindSearch()}
                autoFocus
              />
              <button
                className="btn-search"
                onClick={handleFindSearch}
                disabled={!findQuery.trim()}
              >
                Search
              </button>
            </div>

            <div className="find-tabs">
              <button
                className={`find-tab ${findTab === "online" ? "active" : ""}`}
                onClick={handleFetchOnline}
              >
                Online Now
              </button>
              <button
                className={`find-tab ${findTab === "search" ? "active" : ""}`}
                onClick={() => setFindTab("search")}
              >
                Search Results
              </button>
              <button
                className={`find-tab ${findTab === "all" ? "active" : ""}`}
                onClick={handleFetchAll}
              >
                All Users
              </button>
            </div>

            <div className="find-list">
              {findLoading ? (
                <div className="find-empty">
                  {findTab === "all" ? "Loading users…" : findTab === "online" ? "Loading…" : "Searching…"}
                </div>
              ) : findResults.length === 0 ? (
                <div className="find-empty">
                  {findTab === "online"
                    ? "No other users online right now"
                    : findTab === "all"
                      ? "No users found"
                      : "No results — try a different search"}
                </div>
              ) : (
                findResults
                  .filter((u) => u.uuid !== profile.uuid) // hide self
                  .map((user) => (
                    <div key={user.uuid} className="find-user">
                      <div className="contact-avatar">
                        {user.username.charAt(0).toUpperCase()}
                        {user.online && <div className="contact-status online" />}
                      </div>
                      <div className="find-user-info">
                        <div className="find-user-name">{user.username}</div>
                        <div className="find-user-id">{user.uuid.slice(0, 12)}…</div>
                      </div>
                      {isAlreadyContact(user.uuid) ? (
                        <span className="btn-added">Added</span>
                      ) : (
                        <button
                          className="btn-add"
                          onClick={() => handleAddUser(user)}
                        >
                          Add
                        </button>
                      )}
                    </div>
                  ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* Feature A: Settings Modal */}
      {showSettings && profile && (
        <div className="settings-overlay" onClick={() => setShowSettings(false)}>
          <div className="settings-panel" onClick={(e) => e.stopPropagation()}>
            <div className="settings-header">
              <div className="settings-title">Settings</div>
              <button className="btn-close" onClick={() => setShowSettings(false)}>✕</button>
            </div>

            <div className="settings-section">
              <div className="settings-section-title">Profile</div>
              <div className="settings-field">
                <label className="settings-label">Username</label>
                <div className="settings-input-row">
                  <input
                    className="settings-input"
                    value={settingsUsername}
                    onChange={(e) => setSettingsUsername(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && handleSaveUsername()}
                  />
                  <button className="btn-save" onClick={handleSaveUsername} disabled={!settingsUsername.trim()}>Save</button>
                </div>
              </div>
              <div className="settings-field">
                <label className="settings-label">UUID</label>
                <div className="settings-value mono">{profile.uuid}</div>
              </div>
              <div className="settings-field">
                <label className="settings-label">Fingerprint</label>
                <div className="settings-value mono">{profile.fingerprint || '—'}</div>
              </div>
            </div>

            <div className="settings-section">
              <div className="settings-section-title">Notifications</div>
              <div className="settings-field row">
                <label className="settings-label">Enable notifications</label>
                <label className="toggle-switch">
                  <input
                    type="checkbox"
                    checked={notificationsEnabled}
                    onChange={(e) => handleToggleNotifications(e.target.checked)}
                  />
                  <span className="toggle-slider" />
                </label>
              </div>
            </div>

            <div className="settings-section">
              <div className="settings-section-title">Relay Status</div>
              <div className="settings-field row">
                <label className="settings-label">Connection</label>
                <span className={`settings-status ${wsConnected ? 'connected' : 'offline'}`}>
                  {wsConnected ? 'Connected' : 'Offline'}
                </span>
              </div>
            <div className="shortcuts-section">
              <h4>Keyboard Shortcuts</h4>
              <div className="shortcut-row"><kbd>Ctrl+K</kbd><span>Search contacts</span></div>
              <div className="shortcut-row"><kbd>Ctrl+N</kbd><span>Find people</span></div>
              <div className="shortcut-row"><kbd>Ctrl+F</kbd><span>Search in chat</span></div>
              <div className="shortcut-row"><kbd>Escape</kbd><span>Close modal/overlay</span></div>
            </div>
            </div>

            <div className="settings-section">
              <div className="settings-section-title">Appearance</div>
              <div className="settings-row">
                <span>Theme</span>
                <button className="theme-toggle" onClick={() => setTheme(t => t === 'dark' ? 'light' : 'dark')}>
                  {theme === 'dark' ? 'Light mode' : 'Dark mode'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Feature C: Context menu */}
      {contextMenu && (
        <div className="msg-context-menu" style={{ top: contextMenu.y, left: contextMenu.x }} onClick={(e) => e.stopPropagation()}>
          <button className="msg-context-item" onClick={handleDeleteForMe}>Delete for me</button>
          {contextMenu.isMine && (
            <button className="msg-context-item danger" onClick={handleDeleteForEveryone}>Delete for everyone</button>
          )}
          {contextMenu.isMine && (
            <button className="msg-context-item" onClick={() => {
              const msg = messages.find(m => m.id === contextMenu.msgId);
              if (msg) setEditingMsg({ id: msg.id, content: msg.content });
              setContextMenu(null);
            }}>Edit</button>
          )}
          <button className="msg-context-item" onClick={() => {
            const msg = messages.find(m => m.id === contextMenu.msgId);
            if (msg) setReplyingTo(msg);
            setContextMenu(null);
          }}>Reply</button>
        </div>
      )}

      {/* Feature D: Delete contact confirmation */}
      {deleteConfirm && (
        <div className="settings-overlay" onClick={() => setDeleteConfirm(null)}>
          <div className="confirm-dialog" onClick={(e) => e.stopPropagation()}>
            <div className="confirm-title">Delete contact?</div>
            <div className="confirm-text">This will remove the contact and their messages from your device.</div>
            <div className="confirm-actions">
              <button className="btn-cancel" onClick={() => { setDeleteConfirm(null); setContactMenu(null); }}>Cancel</button>
              <button className="btn-danger" onClick={() => handleDeleteContact(deleteConfirm)}>Delete</button>
            </div>
          </div>
        </div>
      )}

      {/* Feature F1: Edit Message Modal */}
      {editingMsg && (
        <div className="edit-overlay" onClick={() => setEditingMsg(null)}>
          <div className="edit-modal" onClick={e => e.stopPropagation()}>
            <h3>Edit Message</h3>
            <input
              className="edit-input"
              value={editingMsg.content}
              onChange={e => setEditingMsg({...editingMsg, content: e.target.value})}
              onKeyDown={e => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  (async () => {
                    await api.editMessage(editingMsg.id, editingMsg.content);
                    setMessages(prev => prev.map(m => m.id === editingMsg.id ? {...m, content: editingMsg.content, edited: true} : m));
                    setEditingMsg(null);
                    showToast('Message edited', 'success');
                  })();
                }
              }}
              autoFocus
            />
            <div className="edit-actions">
              <button className="btn-cancel" onClick={() => setEditingMsg(null)}>Cancel</button>
              <button className="btn-save" onClick={async () => {
                await api.editMessage(editingMsg.id, editingMsg.content);
                setMessages(prev => prev.map(m => m.id === editingMsg.id ? {...m, content: editingMsg.content, edited: true} : m));
                setEditingMsg(null);
                showToast('Message edited', 'success');
              }}>Save</button>
            </div>
          </div>
        </div>
      )}
      {/* Feature F11/F12: Image lightbox */}
      {lightboxImg && (
        <div className="lightbox-overlay" onClick={() => setLightboxImg(null)}>
          <img src={lightboxImg} alt="Full size" className="lightbox-img" />
          <button className="lightbox-close" onClick={() => setLightboxImg(null)}>✕</button>
        </div>
      )}
    </div>
  );
}

export default App;
