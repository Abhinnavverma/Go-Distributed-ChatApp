package chat

import (
	"encoding/json"
	myMiddleware "go-chat/internal/middleware"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for now (Dev mode)
	},
}

// We define an interface for what we need from the User Service
// This keeps packages loosely coupled
type TokenValidator interface {
	ValidateToken(tokenString string) (int, string, error)
	// Returns userID, username, error
}

type Handler struct {
	hub       *Hub
	validator TokenValidator
}

func NewHandler(hub *Hub, validator TokenValidator) *Handler {
	return &Handler{
		hub:       hub,
		validator: validator,
	}
}

func (h *Handler) ServeWs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(myMiddleware.UserKey).(int)
	username, ok2 := r.Context().Value(myMiddleware.UsernameKey).(string)

	if !ok || !ok2 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// ... (Rest of the function is identical) ...
	// Upgrade connection, Create Client, Send History...
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		Hub:      h.hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		UserID:   userID,
		Username: username,
	}
	client.Hub.Register <- client

	// Send History Logic...
	msgs, err := h.hub.repo.GetRecentMessages(r.Context())
	if err == nil {
		for i := len(msgs) - 1; i >= 0; i-- {
			msg := msgs[i]
			jsonMsg, _ := json.Marshal(map[string]string{
				"username": msg.Username,
				"content":  msg.Content,
			})
			client.Send <- jsonMsg
		}
	}

	go client.WritePump()
	go client.ReadPump()
}
