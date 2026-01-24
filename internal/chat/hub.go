package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type Hub struct {
	clients     map[*Client]bool
	userClients map[int]*Client
	broadcast   chan *BroadcastMessage
	Register    chan *Client
	Unregister  chan *Client
	Publish     chan *Message

	redis  *redis.Client
	pubsub *redis.PubSub
	repo   *Repository
}

func NewHub(redisClient *redis.Client, repo *Repository) *Hub {
	return &Hub{
		broadcast:   make(chan *BroadcastMessage),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		userClients: make(map[int]*Client),
		Publish:     make(chan *Message),
		redis:       redisClient,
		repo:        repo,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true
			h.userClients[client.UserID] = client
			// 🟢 Listen to "user:MY_ID"
			if h.pubsub != nil {
				h.pubsub.Subscribe(context.Background(), fmt.Sprintf("user:%d", client.UserID))
			}

		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				if h.pubsub != nil {
					h.pubsub.Unsubscribe(context.Background(), fmt.Sprintf("user:%d", client.UserID))
				}
				delete(h.clients, client)
				delete(h.userClients, client.UserID)
				close(client.Send)
			}

		case msg := <-h.Publish:
			// 1. Save to DB (The Source of Truth)
			err := h.repo.SaveMessage(context.Background(), msg.ConversationID, msg.UserID, msg.Content)
			if err != nil {
				log.Printf("❌ DB Error: %v\n", err)
				continue
			}

			// 2. 🟢 THE CORRECT WAY: Ask the DB "Who is in this room?"
			participantIDs, err := h.repo.GetConversationParticipants(context.Background(), msg.ConversationID)
			if err != nil {
				log.Printf("❌ Failed to fetch participants: %v", err)
				continue
			}

			// 3. Prepare Payload
			jsonMsg, _ := json.Marshal(map[string]interface{}{
				"conversation_id": msg.ConversationID,
				"username":        msg.Username,
				"content":         msg.Content,
				"sender_id":       msg.UserID,
			})

			// 4. 🟢 FAN-OUT: Send to every participant found in the DB
			for _, targetID := range participantIDs {
				// Optional: Skip sending to self if you want
				// if targetID == msg.UserID { continue }

				targetChannel := fmt.Sprintf("user:%d", targetID)
				h.redis.Publish(context.Background(), targetChannel, jsonMsg)
			}

		case message := <-h.broadcast:
			// Deliver to the specific connected client
			if client, ok := h.userClients[message.TargetID]; ok {
				select {
				case client.Send <- message.Payload:
				default:
					close(client.Send)
					delete(h.clients, client)
					delete(h.userClients, message.TargetID)
				}
			}
		}
	}
}

func (h *Hub) SubscribeToRedis() {
	// Start with NO subscriptions. We add them dynamically in Run().
	h.pubsub = h.redis.Subscribe(context.Background())
	ch := h.pubsub.Channel()

	for msg := range ch {
		var targetID int
		if n, _ := fmt.Sscanf(msg.Channel, "user:%d", &targetID); n == 1 {
			h.broadcast <- &BroadcastMessage{
				TargetID: targetID,
				Payload:  []byte(msg.Payload),
			}
		}
	}
}
