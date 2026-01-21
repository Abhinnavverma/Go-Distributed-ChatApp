package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Hub acts as the central router.
// It maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// 1. The State: A map of registered clients.
	// Note: We use a map[*Client]bool because map lookups are O(1).
	// The boolean value is meaningless (just true), we only care about the key.
	clients map[*Client]bool

	// 2. The Pipes (Channels)
	// These are the ONLY way to interact with the Hub.

	// Inbound messages from the clients.
	broadcast chan []byte

	// new channel to send data to redis
	publish chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
	redis      *redis.Client
}

func newHub(redisClient *redis.Client) *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		publish:    make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		redis:      redisClient,
	}
}

// run is the Infinite Loop that manages the state.
// This runs in its OWN Goroutine. It is the only thing that touches h.clients.
// Therefore, h.clients is Thread-Safe by design.
func (h *Hub) run() {
	go h.subscribeToRedis()
	for {
		// select is the "Traffic Cop". It waits for activity on ANY channel.
		select {
		// CASE 1: Someone connects
		case client := <-h.register:
			h.clients[client] = true

		// CASE 2: Someone disconnects
		case client := <-h.unregister:
			// Always check if they exist to avoid double-deletion panics
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send) // Close their write channel to stop the writePump
			}

		case message := <-h.publish:
			// "general-chat" is the room name. Everyone listens to this.
			err := h.redis.Publish(context.Background(), "general-chat", message).Err()
			if err != nil {
				fmt.Printf("Redis Publish Error: %v\n", err)
			}

		// 👇 OLD LOGIC: This channel now receives messages FROM REDIS
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
func (h *Hub) subscribeToRedis() {
	// 1. Tell Redis: "I want to listen to 'general-chat'"
	pubsub := h.redis.Subscribe(context.Background(), "general-chat")
	defer pubsub.Close()

	// 2. The Loop: Wait for messages
	ch := pubsub.Channel()
	for msg := range ch {
		// 3. When a message arrives, send it to the 'broadcast' channel
		// This triggers the Fan-Out logic in the run() loop above.
		h.broadcast <- []byte(msg.Payload)
	}
}
