import { useState, useEffect, useRef, useCallback } from 'react';
import { EventsOn } from '../wailsjs/runtime/runtime';
import {
  GetLocalUser,
  Register,
  GetContacts,
  GetChatHistory,
  SendMessage,
  AddContact,
  SendTyping,
  SendReaction,
  GetChatReactions,
  GetUnreadCount,
  MarkMessagesRead,
  GetOnlineUsers,
} from '../wailsjs/go/main/App';

interface Contact {
  uuid: string;
  username: string;
  online: boolean;
  unread?: number;
}

interface Message {
  id: string;
  sender: string;
  content: string;
  timestamp: string;
  status: string;
  encrypted: boolean;
  isMe: boolean;
}

interface ReactionInfo {
  emoji: string;
  count: number;
}

function App() {
  const [registered, setRegistered] = useState<boolean | null>(null);
  const [localUser, setLocalUser] = useState<{ uuid: string; username: string } | null>(null);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [activeContact, setActiveContact] = useState<Contact | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [reactions, setReactions] = useState<Record<string, ReactionInfo[]>>({});
  const [inputText, setInputText] = useState('');
  const [registerName, setRegisterName] = useState('');
  const [showAddContact, setShowAddContact] = useState(false);
  const [addUsername, setAddUsername] = useState('');
  const [addUuid, setAddUuid] = useState('');
  const [typingUser, setTypingUser] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Check if user is registered on mount
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

  // Load contacts when registered + start periodic online user polling
  useEffect(() => {
    if (!registered) return;
    loadContacts();
    // Request online users from relay immediately and every 15 seconds
    GetOnlineUsers().catch(() => {});
    const interval = setInterval(() => {
      GetOnlineUsers().catch(() => {});
    }, 15000);
    return () => clearInterval(interval);
  }, [registered]);

  // Subscribe to Wails events
  useEffect(() => {
    if (!registered) return;

    const cleanups = [
      EventsOn('new_message', () => {
        loadContacts();
        if (activeContact) {
          loadChat(activeContact.uuid);
          // Auto-mark as read since the chat is open
          MarkMessagesRead(activeContact.uuid).catch(() => {});
        }
      }),
      EventsOn('typing', (data: { sender: string }) => {
        setTypingUser(data.sender);
        if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
        typingTimerRef.current = setTimeout(() => setTypingUser(null), 3000);
      }),
      EventsOn('peer_discovered', () => {
        loadContacts();
      }),
      EventsOn('reaction', () => {
        if (activeContact) {
          loadReactions(activeContact.uuid);
        }
      }),
      EventsOn('contacts_changed', () => {
        loadContacts();
      }),
      EventsOn('presence_changed', () => {
        loadContacts();
      }),
    ];

    return () => {
      cleanups.forEach((fn) => fn && typeof fn === 'function' && fn());
    };
  }, [registered, activeContact]);

  // Scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const loadContacts = useCallback(async () => {
    try {
      const contactList = await GetContacts();
      // Enrich with unread counts
      const enriched = await Promise.all(
        (contactList || []).map(async (c) => {
          try {
            const count = await GetUnreadCount(c.uuid);
            return { ...c, unread: count };
          } catch {
            return { ...c, unread: 0 };
          }
        })
      );
      setContacts(enriched);
    } catch (err) {
      console.error('Failed to load contacts:', err);
    }
  }, []);

  const loadChat = useCallback(async (uuid: string) => {
    try {
      const history = await GetChatHistory(uuid);
      setMessages(history || []);
      await loadReactions(uuid);
    } catch (err) {
      console.error('Failed to load chat:', err);
    }
  }, []);

  const loadReactions = async (uuid: string) => {
    try {
      const r = await GetChatReactions(uuid);
      setReactions(r || {});
    } catch {
      setReactions({});
    }
  };

  const handleSelectContact = async (contact: Contact) => {
    setActiveContact(contact);
    await loadChat(contact.uuid);
    // Mark messages as read when opening the chat
    try {
      await MarkMessagesRead(contact.uuid);
      // Update the contact's unread count in sidebar
      setContacts((prev) =>
        prev.map((c) => (c.uuid === contact.uuid ? { ...c, unread: 0 } : c))
      );
    } catch (err) {
      console.error('Failed to mark messages read:', err);
    }
  };

  const handleSendMessage = async () => {
    if (!inputText.trim() || !activeContact) return;
    try {
      await SendMessage(activeContact.uuid, inputText.trim());
      setInputText('');
      await loadChat(activeContact.uuid);
      await loadContacts();
    } catch (err) {
      console.error('Failed to send message:', err);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
    // Send typing indicator
    if (activeContact && e.key !== 'Enter') {
      SendTyping(activeContact.uuid).catch(() => {});
    }
  };

  const handleRegister = async () => {
    if (!registerName.trim()) return;
    try {
      const user = await Register(registerName.trim());
      setLocalUser(user);
      setRegistered(true);
    } catch (err) {
      console.error('Failed to register:', err);
    }
  };

  const handleAddContact = async () => {
    if (!addUsername.trim() || !addUuid.trim()) return;
    try {
      await AddContact(addUsername.trim(), addUuid.trim());
      setShowAddContact(false);
      setAddUsername('');
      setAddUuid('');
      await loadContacts();
    } catch (err) {
      console.error('Failed to add contact:', err);
    }
  };

  const handleReaction = async (messageId: string, emoji: string) => {
    if (!activeContact) return;
    try {
      await SendReaction(activeContact.uuid, messageId, emoji);
      await loadReactions(activeContact.uuid);
    } catch (err) {
      console.error('Failed to send reaction:', err);
    }
  };

  const formatTime = (ts: string) => {
    try {
      const d = new Date(ts);
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return '';
    }
  };

  const getInitial = (name: string) => {
    return name ? name.charAt(0).toUpperCase() : '?';
  };

  // ---- Registration Screen ----
  if (registered === null) {
    return (
      <div className="register-screen">
        <div className="grain-overlay" />
        <div className="register-logo">
          <svg className="register-logo-icon" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="4" y="4" width="40" height="40" rx="8" stroke="currentColor" strokeWidth="2.5" fill="none" />
            <path d="M14 18h20M14 24h14M14 30h8" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
          </svg>
          <h1>Terminal</h1>
        </div>
        <p>Loading...</p>
      </div>
    );
  }

  if (!registered) {
    return (
      <div className="register-screen">
        <div className="grain-overlay" />
        <div className="register-logo">
          <svg className="register-logo-icon" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="4" y="4" width="40" height="40" rx="8" stroke="currentColor" strokeWidth="2.5" fill="none" />
            <path d="M14 18h20M14 24h14M14 30h8" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
          </svg>
          <h1>Terminal</h1>
        </div>
        <span className="register-tagline">Communication that never breaks</span>
        <p>
          Works on internet, campus LAN, or completely offline.
          Choose a username to get started.
        </p>
        <div className="register-form">
          <input
            type="text"
            placeholder="Choose a username"
            value={registerName}
            onChange={(e) => setRegisterName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleRegister()}
            autoFocus
          />
          <button onClick={handleRegister}>Start</button>
        </div>
      </div>
    );
  }

  // ---- Main Chat UI ----
  return (
    <div className="app-layout">
      <div className="grain-overlay" />
      {/* Sidebar */}
      <div className="sidebar">
        <div className="sidebar-header">
          <h2>TermTalk</h2>
          {localUser && <span className="user-badge">{localUser.username}</span>}
        </div>

        <div className="contact-list">
          {contacts.length === 0 ? (
            <div style={{ padding: '20px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
              No contacts yet. Add someone below.
            </div>
          ) : (
            contacts.map((contact) => (
              <div
                key={contact.uuid}
                className={`contact-item ${activeContact?.uuid === contact.uuid ? 'active' : ''}`}
                onClick={() => handleSelectContact(contact)}
              >
                <div className="contact-avatar">
                  {getInitial(contact.username)}
                  <span className={`online-dot ${contact.online ? 'online' : 'offline'}`} />
                </div>
                <div className="contact-info">
                  <div className="contact-name">{contact.username}</div>
                  <div className="contact-status">
                    {contact.online ? 'Online' : 'Offline'}
                  </div>
                </div>
                {(contact.unread ?? 0) > 0 && (
                  <span className="unread-badge">{contact.unread}</span>
                )}
              </div>
            ))
          )}
        </div>

        <div className="add-contact-section">
          {showAddContact ? (
            <div className="add-contact-form">
              <input
                type="text"
                placeholder="Username"
                value={addUsername}
                onChange={(e) => setAddUsername(e.target.value)}
                autoFocus
              />
              <input
                type="text"
                placeholder="UUID"
                value={addUuid}
                onChange={(e) => setAddUuid(e.target.value)}
              />
              <div className="add-contact-actions">
                <button className="btn-confirm" onClick={handleAddContact}>
                  Add
                </button>
                <button className="btn-cancel" onClick={() => setShowAddContact(false)}>
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <button className="add-contact-btn" onClick={() => setShowAddContact(true)}>
              + Add Contact
            </button>
          )}
        </div>
      </div>

      {/* Chat Area */}
      <div className="chat-area">
        {!activeContact ? (
          <div className="no-chat-selected">
            <div className="icon">
              <svg viewBox="0 0 56 56" fill="none" xmlns="http://www.w3.org/2000/svg">
                <rect x="4" y="4" width="48" height="48" rx="12" stroke="currentColor" strokeWidth="2" />
                <path d="M18 22h20M18 28h16M18 34h10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            <p>Select a contact to start chatting</p>
          </div>
        ) : (
          <>
            {/* Chat Header */}
            <div className="chat-header">
              <div className="contact-avatar">
                {getInitial(activeContact.username)}
                <span className={`online-dot ${activeContact.online ? 'online' : 'offline'}`} />
              </div>
              <div className="chat-header-info">
                <h3>{activeContact.username}</h3>
                {typingUser === activeContact.uuid ? (
                  <span className="typing-indicator">
                    typing<span className="typing-dots"><span>.</span><span>.</span><span>.</span></span>
                  </span>
                ) : (
                  <span className={`status ${activeContact.online ? 'online' : ''}`}>
                    {activeContact.online ? 'Online' : 'Offline'}
                  </span>
                )}
              </div>
            </div>

            {/* Messages */}
            <div className="messages-container">
              {messages.map((msg) => (
                <div key={msg.id} className={`message-row ${msg.isMe ? 'me' : 'them'}`}>
                  <div
                    className="message-bubble"
                    onDoubleClick={() => handleReaction(msg.id, '👍')}
                  >
                    <div>{msg.content}</div>
                    <div className="message-time">
                      {formatTime(msg.timestamp)}
                      {msg.encrypted && <span className="message-encrypted">🔒</span>}
                    </div>
                    {reactions[msg.id] && reactions[msg.id].length > 0 && (
                      <div className="message-reactions">
                        {reactions[msg.id].map((r, i) => (
                          <span
                            key={i}
                            className="reaction-chip"
                            onClick={() => handleReaction(msg.id, r.emoji)}
                          >
                            {r.emoji} {r.count > 1 ? r.count : ''}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className="message-input-area">
              <div className="message-input-row">
                <input
                  type="text"
                  placeholder="Type a message..."
                  value={inputText}
                  onChange={(e) => setInputText(e.target.value)}
                  onKeyDown={handleKeyDown}
                  autoFocus
                />
                <button
                  className="send-btn"
                  onClick={handleSendMessage}
                  disabled={!inputText.trim()}
                >
                  <svg viewBox="0 0 20 20" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
                    <path d="M3.5 17L17.5 10L3.5 3V8.5L12 10L3.5 11.5V17Z" />
                  </svg>
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default App;
