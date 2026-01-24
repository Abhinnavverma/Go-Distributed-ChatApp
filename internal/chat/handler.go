package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	myMiddleware "go-chat/internal/middleware" // Check your import path!

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for Dev
	},
}

// Handler now needs the Repo to save/fetch chats
type Handler struct {
	hub  *Hub
	repo *Repository
}

func NewHandler(hub *Hub, repo *Repository) *Handler {
	return &Handler{
		hub:  hub,
		repo: repo,
	}
}

// 1. START CHAT: "I want to talk to User B"
// POST /api/conversations
// Body: { "target_id": 2 }
func (h *Handler) StartConversation(w http.ResponseWriter, r *http.Request) {
	// A. Parse Request
	type StartChatRequest struct {
		TargetUserID int `json:"target_id"`
	}
	var req StartChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// B. Get My ID from Middleware
	userID, ok := r.Context().Value(myMiddleware.UserKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// C. Call Repo (Find or Create)
	conversationID, err := h.repo.CreatePrivateConversation(r.Context(), userID, req.TargetUserID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start chat: %v", err), http.StatusInternalServerError)
		return
	}

	// D. Return the ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"conversation_id": conversationID,
	})
}

// 2. GET HISTORY: "Show me messages for Room 101"
// GET /api/conversations/{id}/messages
func (h *Handler) GetChatHistory(w http.ResponseWriter, r *http.Request) {
	// A. Parse Conversation ID from Query Params or URL
	// Assuming URL: /api/messages?conversation_id=101
	convIDStr := r.URL.Query().Get("conversation_id")
	conversationID, err := strconv.Atoi(convIDStr)
	if err != nil {
		http.Error(w, "Invalid conversation_id", http.StatusBadRequest)
		return
	}

	// B. Fetch from Repo
	messages, err := h.repo.GetConversationMessages(r.Context(), conversationID)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	// C. Return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// 3. WEBSOCKET: "Connect me to the real-time stream"
func (h *Handler) ServeWs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(myMiddleware.UserKey).(int)
	username, ok2 := r.Context().Value(myMiddleware.UsernameKey).(string)

	if !ok || !ok2 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Create Client (Note: We added ID field to Client struct earlier)
	client := &Client{
		Hub:      h.hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		UserID:   userID, // 🟢 Make sure your Client struct has this!
		Username: username,
	}

	// Register to Hub (This triggers the "Dual-Listen" redis subscription we wrote)
	client.Hub.Register <- client

	// 🔴 REMOVED: The old "Send History" loop.
	// History is now fetched via the REST API above.

	// Start pumps
	go client.WritePump()
	go client.ReadPump()
}
