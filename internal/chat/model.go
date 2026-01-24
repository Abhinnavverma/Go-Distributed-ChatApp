package chat

import "time"

// ---------------------------------------------
// 🗄️ Database & API Models
// ---------------------------------------------

type Conversation struct {
	ID        int       `json:"id"`
	Type      string    `json:"type"` // 'private' or 'group'
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	UserID         int       `json:"user_id"`
	Username       string    `json:"username"` // 🟢 Denormalized for UI speed (Fetched via JOIN)
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// ---------------------------------------------
// ⚡ Internal Hub Models
// ---------------------------------------------

// BroadcastMessage is used internally to pipe Redis messages to the Hub
type BroadcastMessage struct {
	TargetID int    // 0 = Everyone, >0 = Private Message
	Payload  []byte // The actual JSON data
}

// WSMessage is the simplified JSON the frontend SENDS to us.
// They don't send ID, CreatedAt, or Username (we figure those out).
type WSMessage struct {
	Content        string `json:"content"`
	ConversationID int    `json:"conversation_id"`
}
