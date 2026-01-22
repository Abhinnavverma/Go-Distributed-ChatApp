package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte           // From Redis -> Clients
	Register   chan *Client          // New client joins
	Unregister chan *Client          // Client leaves
	Publish    chan *IncomingMessage // Client types -> Redis
	redis      *redis.Client
	repo       *Repository
}

type IncomingMessage struct {
	UserID   int
	Username string
	Content  string
}

func NewHub(redisClient *redis.Client, repo *Repository) *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		Publish:    make(chan *IncomingMessage),
		redis:      redisClient,
		repo:       repo,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true

		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}

		case msg := <-h.Publish:
			// 1. Save to DB
			// Note: In a real app, use a proper context with timeout
			err := h.repo.SaveMessage(context.Background(), msg.UserID, msg.Content)
			if err != nil {
				fmt.Printf("❌ DB Error: %v\n", err)
			}

			// 2. Prepare JSON for Redis
			jsonMsg, _ := json.Marshal(map[string]string{
				"username": msg.Username,
				"content":  msg.Content,
			})

			// 3. Publish to Redis
			h.redis.Publish(context.Background(), "general-chat", jsonMsg)

		case message := <-h.broadcast:
			// Forward message from Redis to all connected clients
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// SubscribeToRedis listens for messages from other instances
func (h *Hub) SubscribeToRedis() {
	pubsub := h.redis.Subscribe(context.Background(), "general-chat")
	ch := pubsub.Channel()

	for msg := range ch {
		h.broadcast <- []byte(msg.Payload)
	}
}
