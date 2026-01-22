package chat

import "time"

type Message struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"` // We join this from the Users table
}
