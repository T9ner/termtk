import { useState, useEffect, useRef, useCallback } from 'react';
import React from 'react';
import { EventsOn } from '../wailsjs/runtime/runtime';
import {
  GetLocalUser,
  Register,
  GetContacts,
  GetChatHistory,
  SendMessage,
  AddContact,
  DeleteContact,
  SendTyping,
  SendReaction,
  GetChatReactions,
  MarkMessagesRead,
  GetOnlineUsers,
  SearchUsers,
  ListUsers,
  ChangeUsername,
  PinContact,
  ArchiveContact,
  BlockContact,
  DeleteMessagesLocal,
  DeleteMessagesForEveryone,
  EditMessageContent,
  SearchMessages,
} from '../wailsjs/go/main/App';

// ── Types matching Go structs ──

interface ContactInfo {
  uuid: string;
  username: string;
  online: boolean;
  pinned: boolean;
  archived: boolean;
  blocked: boolean;
  unread_count: number;
  last_seen: string;
}

interface MessageInfo {
  id: string;
  sender: string;
  content: string;
  timestamp: string;
  status: string;
  encrypted: boolean;
  isMe: boolean;
  edited: boolean;
  replyTo?: string;
}

interface ReactionInfo {
  emoji: string;
  count: number;
}

interface UserInfo {
  uuid: string;
  username: string;
  online?: boolean;
}

// Feature P3: Emoji Picker Categories
const EMOJI_CATEGORIES: Record<string, string[]> = {
  '😀 Smileys': ['😀','😁','😂','🤣','😃','😄','😅','😆','😉','😊','😋','😎','😍','🥰','😘','😗','😙','😚','🙂','🤗','🤩','🤔','🤨','😐','😑','😶','🙄','😏','😣','😥','😮','🤐','😯','😪','😫','🥱','😴','😌','😛','😜','😝','🤤','😒','😓','😔','😕','🙃','🤑','😲','☹️','🙁','😖','😞','😟','😤','😢','😭','😦','😧','😨','😩','🤯','😬','😰','😱','🥵','🥶','😳','🤪','😵','🥴','😠','😡','🤬','😷','🤒','🤕','🤢','🤮','🥳','🥺','🤠','🤡','🤥','🤫','🤭','🧐','🤓'],
  '👋 Gestures': ['👋','🤚','🖐️','✋','🖖','👌','🤌','🤏','✌️','🤞','🤟','🤘','🤙','👈','👉','👆','🖕','👇','☝️','👍','👎','✊','👊','🤛','🤜','👏','🙌','👐','🤲','🤝','🙏'],
  '❤️ Hearts': ['❤️','🧡','💛','💚','💙','💜','🖤','🤍','🤎','💔','❣️','💕','💞','💓','💗','💖','💘','💝','💟'],
  '🎉 Objects': ['🎉','🎊','🎈','🎁','🏆','🏅','⚽','🏀','🎮','🎵','🎶','🔥','⭐','✨','💫','🌟','💡','💎','🔔','📌','📎','✏️','📝','💬','💭','🗯️','📱','💻','⌨️','🖥️'],
};

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

// Feature B: Reaction emojis
const REACTION_EMOJIS = ['👍', '❤️', '😂', '😮', '😢', '🔥'];

function App() {
  // ── State ──
  const [registered, setRegistered] = useState<boolean | null>(null);
  const [localUser, setLocalUser] = useState<{ uuid: string; username: string } | null>(null);
  const [contacts, setContacts] = useState<ContactInfo[]>([]);
  const [activeChat, setActiveChat] = useState<string | null>(null);
  const [messages, setMessages] = useState<MessageInfo[]>([]);
  const [draft, setDraft] = useState('');
  const [username, setUsername] = useState('');
  const [typingPeers, setTypingPeers] = useState<Set<string>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');

  // Feature F6: Chat search
  const [chatSearch, setChatSearch] = useState('');
  const [chatSearchOpen, setChatSearchOpen] = useState(false);

  // Feature F9: Last message preview per contact
  const [lastMessages, setLastMessages] = useState<Record<string, string>>({});

  const { toasts, show: showToast } = useToast();

  // Find People state
  const [showFind, setShowFind] = useState(false);
  const [findQuery, setFindQuery] = useState('');
  const [findResults, setFindResults] = useState<UserInfo[]>([]);
  const [findTab, setFindTab] = useState<'search' | 'online' | 'all'>('online');
  const [findLoading, setFindLoading] = useState(false);

  // Feature A: Settings modal
  const [showSettings, setShowSettings] = useState(false);
  const [settingsUsername, setSettingsUsername] = useState('');
  const [notificationsEnabled, setNotificationsEnabled] = useState(false);

  // Feature B: Reactions
  const [reactions, setReactions] = useState<Record<string, ReactionInfo[]>>({});
  const [reactionPickerMsgId, setReactionPickerMsgId] = useState<string | null>(null);

  // Feature C: Message context menu
  const [contextMenu, setContextMenu] = useState<{x: number, y: number, msgId: string, isMine: boolean} | null>(null);

  // Feature F1: Edit messages
  const [editingMsg, setEditingMsg] = useState<{id: string, content: string} | null>(null);

  // Feature F2: Reply to messages
  const [replyingTo, setReplyingTo] = useState<MessageInfo | null>(null);

  // Feature D: Contact management
  const [contactMenu, setContactMenu] = useState<string | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  // Feature E: Mobile responsive
  const [mobileShowChat, setMobileShowChat] = useState(false);

  // Feature F19: Pin / Feature F20: Archive
  const [showArchived, setShowArchived] = useState(false);

  // Feature P3: Emoji Picker
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [emojiCategory, setEmojiCategory] = useState(Object.keys(EMOJI_CATEGORIES)[0]);

  // Feature P1: Theme toggle
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('nod-theme');
    if (saved === 'light' || saved === 'dark') return saved;
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  });

  // Feature P7: Global search
  const [globalSearchResults, setGlobalSearchResults] = useState<MessageInfo[]>([]);
  const [globalSearchLoading, setGlobalSearchLoading] = useState(false);
  const globalSearchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const lastTypingSent = useRef<number>(0);

  // ── Load profile on mount ──
  useEffect(() => {
    GetLocalUser()
      .then((user) => {
        setLocalUser(user);
        setRegistered(true);
      })
      .catch(() => {
        setRegistered(false);
      });
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
        const results = await SearchMessages(searchQuery.trim(), 50);
        setGlobalSearchResults(results || []);
      } catch {
        setGlobalSearchResults([]);
      }
      setGlobalSearchLoading(false);
    }, 300);
    return () => { if (globalSearchTimer.current) clearTimeout(globalSearchTimer.current); };
  }, [searchQuery]);

  // ── Load contacts ──
  const loadContacts = useCallback(async () => {
    try {
      const c = await GetContacts();
      setContacts(c || []);
    } catch (err) {
      console.error('Failed to load contacts:', err);
    }
  }, []);

  // Load contacts when registered + start periodic online user polling
  useEffect(() => {
    if (!registered) return;
    loadContacts();
    GetOnlineUsers().catch(() => {});
    const interval = setInterval(() => {
      GetOnlineUsers().catch(() => {});
    }, 15000);
    return () => clearInterval(interval);
  }, [registered, loadContacts]);

  // ── Load reactions ──
  const loadReactions = useCallback(async (chatUuid: string) => {
    try {
      const data = await GetChatReactions(chatUuid);
      setReactions(data || {});
    } catch (err) {
      console.error('Failed to load reactions:', err);
      setReactions({});
    }
  }, []);

  // ── Scroll to bottom on new messages ──
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // ── Close context menu on click outside ──
  useEffect(() => {
    const handleClick = () => { setContextMenu(null); setReactionPickerMsgId(null); setContactMenu(null); setShowEmojiPicker(false); };
    document.addEventListener('click', handleClick);
    return () => { document.removeEventListener('click', handleClick); };
  }, []);

  // ── Wails event subscriptions ──
  useEffect(() => {
    if (!registered) return;

    const cleanups = [
      EventsOn('new_message', () => {
        loadContacts();
        if (activeChat) {
          GetChatHistory(activeChat).then(msgs => {
            setMessages(msgs || []);
          }).catch(() => {});
          MarkMessagesRead(activeChat).catch(() => {});
        }
      }),
      EventsOn('typing', (data: { sender: string }) => {
        setTypingPeers(prev => new Set(prev).add(data.sender));
        const existing = typingTimers.current.get(data.sender);
        if (existing) clearTimeout(existing);
        typingTimers.current.set(
          data.sender,
          setTimeout(() => {
            setTypingPeers(prev => {
              const next = new Set(prev);
              next.delete(data.sender);
              return next;
            });
          }, 3000)
        );
      }),
      EventsOn('peer_discovered', () => {
        loadContacts();
      }),
      EventsOn('reaction', () => {
        if (activeChat) {
          loadReactions(activeChat);
        }
      }),
      EventsOn('contacts_changed', () => {
        loadContacts();
      }),
      EventsOn('presence_changed', () => {
        loadContacts();
      }),
      EventsOn('read_ack', (data: { sender: string; messageIds: string[] }) => {
        setMessages(prev =>
          prev.map(m =>
            data.messageIds.includes(m.id) ? { ...m, status: 'read' } : m
          )
        );
      }),
      EventsOn('online_list', (users: UserInfo[]) => {
        setFindResults(users || []);
        setFindLoading(false);
      }),
      EventsOn('search_results', (users: UserInfo[]) => {
        setFindResults(users || []);
        setFindLoading(false);
      }),
      EventsOn('user_list', (users: UserInfo[]) => {
        setFindResults(users || []);
        setFindLoading(false);
      }),
      // D2: Native OS notifications
      EventsOn('show_notification', (data: { title: string; body: string }) => {
        if (Notification.permission === 'granted') {
          new Notification(data.title, { body: data.body });
        } else if (Notification.permission !== 'denied') {
          Notification.requestPermission().then(p => {
            if (p === 'granted') {
              new Notification(data.title, { body: data.body });
            }
          });
        }
      }),
    ];

    return () => {
      cleanups.forEach(fn => fn && typeof fn === 'function' && fn());
    };
  }, [registered, activeChat, loadContacts, loadReactions]);

  // ── Feature D4: Global keyboard shortcuts ──
  useEffect(() => {
    const handleKeyboard = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (editingMsg) { setEditingMsg(null); return; }
        if (showFind) { setShowFind(false); return; }
        if (showSettings) { setShowSettings(false); return; }
        if (chatSearchOpen) { setChatSearchOpen(false); setChatSearch(''); return; }
        if (contextMenu) { setContextMenu(null); return; }
        setReactionPickerMsgId(null); setContactMenu(null); setDeleteConfirm(null); setShowEmojiPicker(false);
        return;
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        document.querySelector<HTMLInputElement>('.search-input')?.focus();
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault();
        setShowFind(true);
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'f' && activeChat) {
        e.preventDefault();
        setChatSearchOpen(prev => !prev);
      }
    };
    window.addEventListener('keydown', handleKeyboard);
    return () => window.removeEventListener('keydown', handleKeyboard);
  }, [editingMsg, showFind, showSettings, chatSearchOpen, contextMenu, activeChat]);

  // ── Handlers ──

  const handleRegister = async () => {
    if (!username.trim()) return;
    try {
      const user = await Register(username.trim());
      setLocalUser(user);
      setRegistered(true);
    } catch (err) {
      console.error('Registration failed:', err);
      showToast('Registration failed', 'error');
    }
  };

  const handleSend = async () => {
    if (!draft.trim() || !activeChat) return;
    const content = draft.trim();
    setDraft('');
    setReplyingTo(null);
    try {
      await SendMessage(activeChat, content);
      const msgs = await GetChatHistory(activeChat);
      setMessages(msgs || []);
      loadContacts();
    } catch (err) {
      console.error('Send failed:', err);
      showToast('Failed to send', 'error');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
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
    const msgs = await GetChatHistory(uuid);
    setMessages(msgs || []);
    if (msgs && msgs.length > 0) {
      const lastMsg = msgs[msgs.length - 1];
      const previewText = lastMsg.content.length > 40 ? lastMsg.content.slice(0, 40) + '…' : lastMsg.content;
      setLastMessages(prev => ({ ...prev, [uuid]: previewText }));
    }
    loadReactions(uuid);
    MarkMessagesRead(uuid).catch(() => {});
    setContacts(prev => prev.map(c => c.uuid === uuid ? { ...c, unread_count: 0 } : c));
  };

  // ── Find People handlers ──
  const openFindPeople = () => {
    setShowFind(true);
    setFindResults([]);
    setFindQuery('');
    setFindTab('online');
    handleFetchOnline();
  };

  const handleFetchOnline = async () => {
    setFindTab('online');
    setFindLoading(true);
    setFindResults([]);
    try {
      await GetOnlineUsers();
    } catch {
      setFindLoading(false);
    }
  };

  const handleFindSearch = async () => {
    if (!findQuery.trim()) return;
    setFindTab('search');
    setFindLoading(true);
    setFindResults([]);
    try {
      await SearchUsers(findQuery.trim());
    } catch {
      setFindLoading(false);
    }
  };

  const handleFetchAll = async () => {
    setFindTab('all');
    setFindLoading(true);
    setFindResults([]);
    try {
      await ListUsers();
    } catch {
      setFindLoading(false);
    }
  };

  const handleAddUser = async (user: UserInfo) => {
    try {
      await AddContact(user.username, user.uuid);
      await loadContacts();
      showToast('Contact added', 'success');
    } catch {
      showToast('Failed to add contact', 'error');
    }
  };

  const isAlreadyContact = (uuid: string) =>
    contacts.some(c => c.uuid === uuid);

  const activeContact = contacts.find(c => c.uuid === activeChat);

  const filteredContacts = contacts
    .filter(c => c.username.toLowerCase().includes(searchQuery.toLowerCase()))
    .sort((a, b) => {
      if (a.pinned && !b.pinned) return -1;
      if (!a.pinned && b.pinned) return 1;
      return 0;
    });

  const activeContacts = filteredContacts.filter(c => !c.archived);
  const archivedContacts = filteredContacts.filter(c => c.archived);

  const formatTime = (ts: string) => {
    if (!ts || ts.startsWith('0001')) return 'Never';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return 'Unknown';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const formatDateDivider = (ts: string) => {
    const d = new Date(ts);
    const today = new Date();
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    if (d.toDateString() === today.toDateString()) return 'Today';
    if (d.toDateString() === yesterday.toDateString()) return 'Yesterday';
    return d.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' });
  };

  const highlightText = (text: string, query: string) => {
    if (!query) return text;
    const parts = text.split(new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi'));
    return parts.map((part, i) =>
      part.toLowerCase() === query.toLowerCase() ? <mark key={i}>{part}</mark> : part
    );
  };

  // Feature P4: Parse markdown-like formatting
  const parseFormatting = (text: string, keyPrefix: string): React.ReactNode[] => {
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

  // Feature F5: Render message content with clickable URLs and search highlighting
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
      const formatted = parseFormatting(part, `${baseKey}-${i}`);
      if (sq) {
        return <React.Fragment key={`${baseKey}-${i}`}>{formatted.map((node, j) => {
          if (typeof node === 'string') return <React.Fragment key={`hl-${j}`}>{highlightText(node, sq)}</React.Fragment>;
          return node;
        })}</React.Fragment>;
      }
      return <React.Fragment key={`${baseKey}-${i}`}>{formatted}</React.Fragment>;
    });
  };

  const renderMessageContent = (text: string, searchQ: string) => {
    return <>{renderTextSegment(text, searchQ, 0)}</>;
  };

  // ── Feature A: Settings handlers ──
  const openSettings = () => {
    setShowSettings(true);
    setSettingsUsername(localUser?.username || '');
  };

  const handleSaveUsername = async () => {
    if (!settingsUsername.trim()) return;
    try {
      await ChangeUsername(settingsUsername.trim());
      setLocalUser(prev => prev ? { ...prev, username: settingsUsername.trim() } : prev);
      showToast('Username updated', 'success');
    } catch {
      showToast('Failed to update username', 'error');
    }
  };

  const handleToggleNotifications = (enabled: boolean) => {
    setNotificationsEnabled(enabled);
    if (enabled) {
      Notification.requestPermission().then(p => {
        if (p !== 'granted') {
          setNotificationsEnabled(false);
          showToast('Notification permission denied', 'error');
        } else {
          showToast('Notifications enabled', 'success');
        }
      });
    } else {
      showToast('Notifications disabled', 'success');
    }
  };

  // ── Feature B: Reaction handlers ──
  const handleReact = async (messageId: string, emoji: string) => {
    if (!activeChat) return;
    setReactionPickerMsgId(null);
    try {
      await SendReaction(activeChat, messageId, emoji);
      loadReactions(activeChat);
    } catch {
      showToast('Failed to add reaction', 'error');
    }
  };

  const groupReactions = (msgReactions: ReactionInfo[]) => {
    const grouped: Record<string, number> = {};
    for (const r of msgReactions) {
      grouped[r.emoji] = (grouped[r.emoji] || 0) + r.count;
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
      await DeleteMessagesLocal([msgId]);
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
      await DeleteMessagesForEveryone(activeChat, [msgId]);
      setMessages(prev => prev.filter(m => m.id !== msgId));
      showToast('Message deleted for everyone', 'success');
    } catch {
      showToast('Failed to delete message', 'error');
    }
  };

  // ── Feature D: Contact management handlers ──
  const handleDeleteContact = async (uuid: string) => {
    setDeleteConfirm(null);
    setContactMenu(null);
    try {
      await DeleteContact(uuid);
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

  const handleBlockContact = async (uuid: string, block: boolean) => {
    setContactMenu(null);
    try {
      await BlockContact(uuid, block);
      setContacts(prev => prev.map(c => c.uuid === uuid ? {...c, blocked: block} : c));
      showToast(block ? 'User blocked' : 'User unblocked', 'success');
    } catch {
      showToast('Failed to update block status', 'error');
    }
  };

  const handlePinContact = async (uuid: string, pin: boolean) => {
    setContactMenu(null);
    try {
      await PinContact(uuid, pin);
      setContacts(prev => prev.map(c => c.uuid === uuid ? {...c, pinned: pin} : c));
      showToast(pin ? 'Pinned' : 'Unpinned', 'success');
    } catch {
      showToast('Failed to update pin status', 'error');
    }
  };

  const handleArchiveContact = async (uuid: string, archive: boolean) => {
    setContactMenu(null);
    try {
      await ArchiveContact(uuid, archive);
      setContacts(prev => prev.map(c => c.uuid === uuid ? {...c, archived: archive} : c));
      showToast(archive ? 'Archived' : 'Unarchived', 'success');
    } catch {
      showToast('Failed to update archive status', 'error');
    }
  };

  // ── Render ──

  if (registered === null) {
    return (
      <div className="empty-state">
        <div className="empty-state-text">Connecting…</div>
      </div>
    );
  }

  if (!registered) {
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
            onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
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
          <div className="sidebar-subtitle">{localUser?.username}</div>
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
                  const chatUuid = msg.isMe ? '' : msg.sender;
                  if (chatUuid) selectChat(chatUuid);
                }}
              >
                <div className="global-search-label">
                  {contacts.find(c => c.uuid === msg.sender)?.username || msg.sender.slice(0, 8)}
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
            <div className="empty-state" style={{ padding: '40px 0' }}>
              <div className="empty-state-text">No contacts yet</div>
            </div>
          ) : (
            <>
            {activeContacts.map((contact) => (
              <div
                key={contact.uuid}
                className={`contact-item ${activeChat === contact.uuid ? 'active' : ''}`}
                onClick={() => selectChat(contact.uuid)}
              >
                <div className="contact-avatar">
                  {contact.username.charAt(0).toUpperCase()}
                  <div className={`contact-status ${contact.online ? 'online' : 'offline'}`} />
                </div>
                <div className="contact-info">
                  <div className="contact-name">{contact.pinned && <svg className="pin-indicator" width="12" height="12" viewBox="0 0 24 24" fill="currentColor"><path d="M16 12V4h1V2H7v2h1v8l-2 2v2h5v6l1 1 1-1v-6h5v-2l-2-2z"/></svg>}{contact.username}</div>
                  <div className="contact-preview">
                    {lastMessages[contact.uuid] || (contact.online ? 'Online' : `Last seen ${formatTime(contact.last_seen)}`)}
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
                      <button className="contact-dropdown-item" onClick={() => handlePinContact(contact.uuid, !contact.pinned)}>
                        {contact.pinned ? 'Unpin' : 'Pin'}
                      </button>
                      <button className="contact-dropdown-item" onClick={() => handleArchiveContact(contact.uuid, !contact.archived)}>
                        {contact.archived ? 'Unarchive' : 'Archive'}
                      </button>
                      <button className="contact-dropdown-item" onClick={() => handleBlockContact(contact.uuid, !contact.blocked)}>
                        {contact.blocked ? 'Unblock' : 'Block'}
                      </button>
                      <button className="contact-dropdown-item danger" onClick={() => setDeleteConfirm(contact.uuid)}>
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
                    className={`contact-item ${activeChat === contact.uuid ? 'active' : ''}`}
                    onClick={() => selectChat(contact.uuid)}
                  >
                    <div className="contact-avatar">
                      {contact.username.charAt(0).toUpperCase()}
                      <div className={`contact-status ${contact.online ? 'online' : 'offline'}`} />
                    </div>
                    <div className="contact-info">
                      <div className="contact-name">{contact.username}</div>
                      <div className="contact-preview">
                        {lastMessages[contact.uuid] || (contact.online ? 'Online' : `Last seen ${formatTime(contact.last_seen)}`)}
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
                          <button className="contact-dropdown-item" onClick={() => handleArchiveContact(contact.uuid, false)}>
                            Unarchive
                          </button>
                          <button className="contact-dropdown-item" onClick={() => handleBlockContact(contact.uuid, !contact.blocked)}>
                            {contact.blocked ? 'Unblock' : 'Block'}
                          </button>
                          <button className="contact-dropdown-item danger" onClick={() => setDeleteConfirm(contact.uuid)}>
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
      <div className="chat-area">
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
              <div className="contact-avatar" style={{ width: 32, height: 32, fontSize: '0.75rem' }}>
                {activeContact?.username.charAt(0).toUpperCase()}
              </div>
              <div>
                <div className="chat-header-name">
                  {activeContact?.username}
                  {activeContact?.blocked && <span className="blocked-badge">Blocked</span>}
                </div>
                <div className={`chat-header-status ${activeContact?.online ? 'is-online' : ''}`}>
                  {typingPeers.has(activeChat)
                    ? 'typing…'
                    : activeContact?.online
                      ? 'Online'
                      : 'Offline'}
                </div>
              </div>
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
                const isSent = msg.isMe;
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
                      className={`message-row ${isSent ? 'sent' : 'received'}`}
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
                          {msg.replyTo && (
                            <div className="msg-reply-quote" onClick={() => {
                              const el = document.getElementById(`msg-${msg.replyTo}`);
                              if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
                            }}>
                              <span className="msg-reply-text">
                                {messages.find(m => m.id === msg.replyTo)?.content?.substring(0, 50) || 'Original message'}
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

            <div className="typing-indicator" style={{ padding: '0 24px' }}>
              {typingPeers.has(activeChat) && `${activeContact?.username} is typing…`}
            </div>

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
              <input
                className="compose-input"
                placeholder="Type a message…"
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value);
                  const now = Date.now();
                  if (now - lastTypingSent.current > 2500) {
                    lastTypingSent.current = now;
                    SendTyping(activeChat).catch(() => {});
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
                disabled={!draft.trim()}
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
                onKeyDown={(e) => e.key === 'Enter' && handleFindSearch()}
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
                className={`find-tab ${findTab === 'online' ? 'active' : ''}`}
                onClick={handleFetchOnline}
              >
                Online Now
              </button>
              <button
                className={`find-tab ${findTab === 'search' ? 'active' : ''}`}
                onClick={() => setFindTab('search')}
              >
                Search Results
              </button>
              <button
                className={`find-tab ${findTab === 'all' ? 'active' : ''}`}
                onClick={handleFetchAll}
              >
                All Users
              </button>
            </div>

            <div className="find-list">
              {findLoading ? (
                <div className="find-empty">
                  {findTab === 'all' ? 'Loading users…' : findTab === 'online' ? 'Loading…' : 'Searching…'}
                </div>
              ) : findResults.length === 0 ? (
                <div className="find-empty">
                  {findTab === 'online'
                    ? 'No other users online right now'
                    : findTab === 'all'
                      ? 'No users found'
                      : 'No results — try a different search'}
                </div>
              ) : (
                findResults
                  .filter(u => u.uuid !== localUser?.uuid)
                  .map(user => (
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
      {showSettings && localUser && (
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
                    onKeyDown={(e) => e.key === 'Enter' && handleSaveUsername()}
                  />
                  <button className="btn-save" onClick={handleSaveUsername} disabled={!settingsUsername.trim()}>Save</button>
                </div>
              </div>
              <div className="settings-field">
                <label className="settings-label">UUID</label>
                <div className="settings-value mono">{localUser.uuid}</div>
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
                <span className="settings-status connected">
                  Connected
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
                    await EditMessageContent(editingMsg.id, editingMsg.content);
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
                await EditMessageContent(editingMsg.id, editingMsg.content);
                setMessages(prev => prev.map(m => m.id === editingMsg.id ? {...m, content: editingMsg.content, edited: true} : m));
                setEditingMsg(null);
                showToast('Message edited', 'success');
              }}>Save</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
