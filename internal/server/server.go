// Package server provides an HTTP and WebSocket API layer on top of
// the Nod client.Client facade. It enables the React web UI to
// communicate with the Go backend over localhost.
//
// Architecture:
//
//	React UI  ──WebSocket──▶  server.Server  ──▶  client.Client
//	                                                  │
//	              REST API  ◀──────────────────────────┘
//
// REST endpoints handle request/response operations (send message,
// list contacts). The WebSocket connection pushes real-time events
// (new message received, peer discovered, typing indicators).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"nod/internal/client"
	"nod/internal/db"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Server wraps a client.Client and exposes it over HTTP + WebSocket.
type Server struct {
	client *client.Client
	mux    *http.ServeMux
	srv    *http.Server
	port   int

	// startNetworking is called after first-time registration to
	// boot the relay/discovery/sync layer without a restart.
	startNetworking func() error

	// WebSocket subscribers
	mu          sync.Mutex
	subscribers map[*wsConn]struct{}
}

// SetStartNetworkingFunc sets the callback used to start networking
// after a user registers for the first time.
func (s *Server) SetStartNetworkingFunc(fn func() error) {
	s.startNetworking = fn
}

// wsConn tracks a single WebSocket connection for broadcasting.
type wsConn struct {
	conn *websocket.Conn
	ctx  context.Context
}

// WSEvent is the envelope for all WebSocket messages sent to the UI.
type WSEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// New creates a Server bound to the given client on the specified port.
func New(c *client.Client, port int) *Server {
	s := &Server{
		client:      c,
		port:        port,
		subscribers: make(map[*wsConn]struct{}),
	}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

// routes registers all HTTP handlers.
func (s *Server) routes() {
	// Health
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Profile
	s.mux.HandleFunc("GET /api/profile", s.handleGetProfile)
	s.mux.HandleFunc("PUT /api/profile", s.handleUpdateProfile)
	s.mux.HandleFunc("POST /api/register", s.handleRegister)

	// Contacts
	s.mux.HandleFunc("GET /api/contacts", s.handleListContacts)
	s.mux.HandleFunc("POST /api/contacts", s.handleAddContact)
	s.mux.HandleFunc("DELETE /api/contacts/{uuid}", s.handleDeleteContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/verify", s.handleVerifyContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/block", s.handleBlockContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/unblock", s.handleUnblockContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/pin", s.handlePinContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/unpin", s.handleUnpinContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/archive", s.handleArchiveContact)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/unarchive", s.handleUnarchiveContact)

	// Messages
	s.mux.HandleFunc("GET /api/contacts/{uuid}/messages", s.handleGetMessages)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/messages", s.handleSendMessage)
	s.mux.HandleFunc("POST /api/contacts/{uuid}/messages/read", s.handleMarkRead)
	s.mux.HandleFunc("GET /api/contacts/{uuid}/unread", s.handleUnreadCount)
	s.mux.HandleFunc("POST /api/messages/delete", s.handleDeleteMessages)
	s.mux.HandleFunc("POST /api/messages/delete-for-everyone", s.handleDeleteForEveryone)
	s.mux.HandleFunc("PUT /api/messages/{id}", s.handleEditMessage)
	s.mux.HandleFunc("GET /api/messages/{id}", s.handleGetMessage)

	// Reactions
	s.mux.HandleFunc("POST /api/contacts/{uuid}/messages/{msgID}/react", s.handleReact)
	s.mux.HandleFunc("GET /api/contacts/{uuid}/reactions", s.handleGetReactions)

	// Typing
	s.mux.HandleFunc("POST /api/contacts/{uuid}/typing", s.handleTyping)

	// Peer status
	s.mux.HandleFunc("GET /api/contacts/{uuid}/online", s.handlePeerOnline)

	// Relay / directory
	s.mux.HandleFunc("POST /api/search", s.handleSearch)
	s.mux.HandleFunc("POST /api/online-users", s.handleOnlineUsers)
	s.mux.HandleFunc("POST /api/list-users", s.handleListUsers)

	// Settings
	s.mux.HandleFunc("GET /api/settings/notifications", s.handleGetNotifications)
	s.mux.HandleFunc("PUT /api/settings/notifications", s.handleSetNotifications)

	// WebSocket
	s.mux.HandleFunc("GET /ws", s.handleWS)

	// Files (F11/F12: File & Image Sharing)
	s.mux.HandleFunc("POST /api/upload", s.handleUpload)
	s.mux.HandleFunc("GET /api/files/{id}", s.handleDownload)
	s.mux.HandleFunc("GET /api/messages/{msgID}/attachments", s.handleGetAttachments)
	s.mux.HandleFunc("GET /api/contacts/{uuid}/attachments", s.handleGetContactAttachments)

	// Search messages (P7: Full-Text Search)
	s.mux.HandleFunc("POST /api/search-messages", s.handleSearchMessages)

	// Data Export (P8)
	s.mux.HandleFunc("POST /api/export", s.handleExport)
}

// Start begins listening on 127.0.0.1:port and pumping client events
// to all connected WebSocket subscribers.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}

	s.srv = &http.Server{
		Handler:     s.corsMiddleware(s.mux),
		ReadTimeout: 15 * time.Second,
		// WriteTimeout must be 0 for WebSocket connections — a non-zero
		// value kills the TCP connection after that duration.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	// Pump client events to WebSocket subscribers.
	go s.eventPump(ctx)

	log.Printf("Nod API server listening on http://%s", addr)
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server: serve error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// Port returns the configured port.
func (s *Server) Port() int {
	return s.port
}

// corsMiddleware adds permissive CORS headers for localhost dev.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── JSON helpers ──

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("server: json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// ── Event Pump ──

func (s *Server) eventPump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-s.client.Events():
			if !ok {
				return
			}
			s.broadcastEvent(evt)
		}
	}
}

func (s *Server) broadcastEvent(evt client.Event) {
	var wsEvt WSEvent

	switch e := evt.(type) {
	case client.PeerDiscoveredEvent:
		wsEvt = WSEvent{Type: "peer_discovered", Data: e.Contact}
	case client.MessageReceivedEvent:
		wsEvt = WSEvent{Type: "message_received", Data: e.Message}
	case client.SearchResultEvent:
		wsEvt = WSEvent{Type: "search_result", Data: e.Users}
	case client.OnlineListEvent:
		wsEvt = WSEvent{Type: "online_list", Data: e.Users}
	case client.UserListEvent:
		wsEvt = WSEvent{Type: "user_list", Data: e.Users}
	case client.TypingEvent:
		wsEvt = WSEvent{Type: "typing", Data: map[string]string{"sender_uuid": e.SenderUUID}}
	case client.ICEConnectedEvent:
		wsEvt = WSEvent{Type: "ice_connected", Data: map[string]interface{}{
			"peer_uuid": e.PeerUUID, "direct": e.Direct,
		}}
	case client.ReadAckEvent:
		wsEvt = WSEvent{Type: "read_ack", Data: map[string]interface{}{
			"sender_uuid": e.SenderUUID, "message_ids": e.MessageIDs,
		}}
	case client.ReactionEvent:
		wsEvt = WSEvent{Type: "reaction", Data: e.Reaction}
	default:
		return
	}

	// Snapshot subscribers under lock, then write outside lock to
	// avoid holding the mutex during blocking network I/O.
	s.mu.Lock()
	snapshot := make([]*wsConn, 0, len(s.subscribers))
	for sub := range s.subscribers {
		snapshot = append(snapshot, sub)
	}
	s.mu.Unlock()

	var failed []*wsConn
	for _, sub := range snapshot {
		if err := wsjson.Write(sub.ctx, sub.conn, wsEvt); err != nil {
			sub.conn.Close(websocket.StatusNormalClosure, "write error")
			failed = append(failed, sub)
		}
	}

	if len(failed) > 0 {
		s.mu.Lock()
		for _, sub := range failed {
			delete(s.subscribers, sub)
		}
		s.mu.Unlock()
	}
}

// ── WebSocket Handler ──

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // localhost only
	})
	if err != nil {
		log.Printf("server: ws accept error: %v", err)
		return
	}

	// Use a connection-scoped context — NOT r.Context(), which is
	// tied to the HTTP request lifecycle and gets cancelled after
	// the server's timeouts.
	connCtx, connCancel := context.WithCancel(context.Background())
	defer connCancel()

	sub := &wsConn{conn: conn, ctx: connCtx}
	s.mu.Lock()
	s.subscribers[sub] = struct{}{}
	s.mu.Unlock()

	// Keep alive — read loop (client can send pings or commands).
	for {
		_, _, err := conn.Read(connCtx)
		if err != nil {
			break
		}
	}

	connCancel()
	s.mu.Lock()
	delete(s.subscribers, sub)
	s.mu.Unlock()
	conn.Close(websocket.StatusNormalClosure, "")
}

// ── REST Handlers ──

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "app": "nod"})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, _ *http.Request) {
	p := s.client.GetProfile()
	if p == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"registered": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"registered":  true,
		"uuid":        p.UUID,
		"username":    p.Username,
		"fingerprint": p.Fingerprint(),
		"public_key":  s.client.GetPublicKeyBase64(),
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := s.client.Register(body.Username)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Start networking if this is a first-time registration.
	if s.startNetworking != nil {
		if err := s.startNetworking(); err != nil {
			log.Printf("server: failed to start networking after registration: %v", err)
		}
		s.startNetworking = nil // only call once
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"uuid":        p.UUID,
		"username":    p.Username,
		"fingerprint": p.Fingerprint(),
	})
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}
	if err := s.client.ChangeUsername(req.Username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	profile := s.client.GetProfile()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"registered":  true,
		"uuid":        profile.UUID,
		"username":    profile.Username,
		"fingerprint": profile.Fingerprint(),
		"public_key":  s.client.GetPublicKeyBase64(),
	})
}

func (s *Server) handleListContacts(w http.ResponseWriter, _ *http.Request) {
	contacts, err := s.client.ListContacts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Augment with online status and unread count.
	type contactResponse struct {
		db.Contact
		Online      bool `json:"online"`
		UnreadCount int  `json:"unread_count"`
	}
	result := make([]contactResponse, 0, len(contacts))
	for _, c := range contacts {
		unread, _ := s.client.GetUnreadCount(c.UUID)
		result = append(result, contactResponse{
			Contact:     c,
			Online:      s.client.IsPeerOnline(c.UUID),
			UnreadCount: unread,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAddContact(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		UUID     string `json:"uuid"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.AddContact(body.Username, body.UUID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.DeleteContact(uuid); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleVerifyContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.SetContactVerified(uuid, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	contactUUID := r.PathValue("uuid")

	// Parse optional query params
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 50
	offset := 0
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	msgs, err := s.client.GetChatHistory(contactUUID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	var body struct {
		Content string `json:"content"`
		ReplyTo string `json:"reply_to"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.SendMessage(uuid, body.Content, body.ReplyTo); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "sent"})
}

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	var body struct {
		MessageIDs []string `json:"message_ids"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.MarkMessagesRead(body.MessageIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Also send read ack to the peer.
	_ = s.client.SendReadAck(uuid, body.MessageIDs)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	count, err := s.client.GetUnreadCount(uuid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (s *Server) handleDeleteMessages(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MessageIDs []string `json:"message_ids"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.DeleteMessagesLocal(body.MessageIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleDeleteForEveryone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ContactUUID string   `json:"contact_uuid"`
		MessageIDs  []string `json:"message_ids"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.DeleteMessagesForEveryone(body.ContactUUID, body.MessageIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleEditMessage(w http.ResponseWriter, r *http.Request) {
	msgID := r.PathValue("id")
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeError(w, http.StatusBadRequest, "content required")
		return
	}
	if err := s.client.EditMessageContent(msgID, body.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	msgID := r.PathValue("id")
	msg, err := s.client.GetMessage(msgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleReact(w http.ResponseWriter, r *http.Request) {
	contactUUID := r.PathValue("uuid")
	messageID := r.PathValue("msgID")
	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.SendReaction(contactUUID, messageID, body.Emoji); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetReactions(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	reactions, err := s.client.GetChatReactions(uuid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, reactions)
}

func (s *Server) handleTyping(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.SendTyping(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePeerOnline(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	writeJSON(w, http.StatusOK, map[string]bool{"online": s.client.IsPeerOnline(uuid)})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.SearchUsers(body.Query); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Results arrive async via WebSocket search_result event.
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "searching"})
}

func (s *Server) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	if err := s.client.GetOnlineUsers(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "fetching"})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if err := s.client.ListUsers(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "fetching"})
}

func (s *Server) handleGetNotifications(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"enabled": s.client.GetNotificationsEnabled(),
	})
}

func (s *Server) handleSetNotifications(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.client.SetNotificationsEnabled(body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBlockContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.BlockContact(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

func (s *Server) handleUnblockContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.UnblockContact(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

func (s *Server) handlePinContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.PinContact(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pinned"})
}

func (s *Server) handleUnpinContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.UnpinContact(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpinned"})
}

func (s *Server) handleArchiveContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.ArchiveContact(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func (s *Server) handleUnarchiveContact(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if err := s.client.UnarchiveContact(uuid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unarchived"})
}

// ── File Handlers (F11/F12: File & Image Sharing) ──

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 50MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	att, err := s.client.SaveUpload(header.Filename, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// If msg_id is provided, link the attachment to a message
	msgID := r.FormValue("msg_id")
	if msgID != "" {
		att.MsgID = msgID
		if err := s.client.SaveAttachment(att); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, att)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path, err := s.client.GetUploadPath(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) handleGetAttachments(w http.ResponseWriter, r *http.Request) {
	msgID := r.PathValue("msgID")
	atts, err := s.client.GetAttachmentsByMsgID(msgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if atts == nil {
		atts = []db.Attachment{}
	}
	writeJSON(w, http.StatusOK, atts)
}

func (s *Server) handleGetContactAttachments(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	msgs, err := s.client.GetChatHistory(uuid, 200, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	attMap, err := s.client.GetAttachmentsForMessages(ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if attMap == nil {
		attMap = make(map[string][]db.Attachment)
	}
	writeJSON(w, http.StatusOK, attMap)
}

// ── Feature P7: Full-Text Search ──

func (s *Server) handleSearchMessages(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Limit <= 0 {
		body.Limit = 50
	}
	msgs, err := s.client.SearchMessages(body.Query, body.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

// ── Feature P8: Data Export ──

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ContactUUID string `json:"contact_uuid"`
		Format      string `json:"format"`
	}
	if err := decodeBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	msgs, err := s.client.GetChatHistory(body.ContactUUID, 10000, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if body.Format == "txt" {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", `attachment; filename="chat_export.txt"`)
		for _, m := range msgs {
			fmt.Fprintf(w, "[%s] %s: %s\n", m.Timestamp.Format("2006-01-02 15:04:05"), m.Sender, m.Content)
		}
		return
	}
	// Default JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="chat_export.json"`)
	json.NewEncoder(w).Encode(msgs)
}
