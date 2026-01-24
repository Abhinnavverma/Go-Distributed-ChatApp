package chat

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB // Utilizing your existing DB wrapper if preferred, or just *sql.DB
}

// Ensure the struct matches what you pass in main.go
func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

// ---------------------------------------------
// 📦 Data Models (Matching your new Schema)
// ---------------------------------------------

// ---------------------------------------------
// 🧠 Core Logic
// ---------------------------------------------

// CreatePrivateConversation finds an existing private chat or creates a new one.
// This is the "Find or Create" logic we discussed.
func (r *Repository) CreatePrivateConversation(ctx context.Context, user1ID, user2ID int) (int, error) {
	// 1. First, check if a private conversation already exists between these two
	var conversationID int
	query := `
        SELECT c.id 
        FROM conversations c
        JOIN participants p1 ON c.id = p1.conversation_id
        JOIN participants p2 ON c.id = p2.conversation_id
        WHERE c.type = 'private' 
        AND p1.user_id = $1 
        AND p2.user_id = $2
    `
	// Note: We use r.db.Conn (accessing the raw sql.DB connection)
	err := r.db.QueryRowContext(ctx, query, user1ID, user2ID).Scan(&conversationID)

	if err == nil {
		// ✅ Found it! Return existing ID
		return conversationID, nil
	} else if err != sql.ErrNoRows {
		// ❌ Real DB error
		return 0, fmt.Errorf("error searching for conversation: %w", err)
	}

	// 2. Not found? Create a NEW one using a Transaction.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // Safety net: Rollback if we don't commit

	// A. Create the Conversation Row
	err = tx.QueryRowContext(ctx, "INSERT INTO conversations (type) VALUES ('private') RETURNING id").Scan(&conversationID)
	if err != nil {
		return 0, fmt.Errorf("failed to create conversation: %w", err)
	}

	// B. Add User 1 (You)
	_, err = tx.ExecContext(ctx, "INSERT INTO participants (conversation_id, user_id) VALUES ($1, $2)", conversationID, user1ID)
	if err != nil {
		return 0, fmt.Errorf("failed to add user 1: %w", err)
	}

	// C. Add User 2 (Them)
	_, err = tx.ExecContext(ctx, "INSERT INTO participants (conversation_id, user_id) VALUES ($1, $2)", conversationID, user2ID)
	if err != nil {
		return 0, fmt.Errorf("failed to add user 2: %w", err)
	}

	// 3. Commit the transaction
	if err = tx.Commit(); err != nil {
		return 0, err
	}

	return conversationID, nil
}

// SaveMessage now requires a conversationID
func (r *Repository) SaveMessage(ctx context.Context, conversationID int, senderID int, content string) error {
	query := `INSERT INTO messages (conversation_id, sender_id, content) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, conversationID, senderID, content)
	return err
}

// GetConversationMessages fetches history for a specific room
func (r *Repository) GetConversationMessages(ctx context.Context, conversationID int) ([]*Message, error) {
	query := `
        SELECT m.id, m.conversation_id, m.content, m.created_at, m.sender_id, u.username
        FROM messages m
        JOIN users u ON m.sender_id = u.id
        WHERE m.conversation_id = $1
        ORDER BY m.created_at ASC 
        LIMIT 50
    `
	// Note: Changed ORDER BY to ASC so older messages appear at top (standard for chat history)

	rows, err := r.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Content, &msg.CreatedAt, &msg.UserID, &msg.Username); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (r *Repository) GetConversationParticipants(ctx context.Context, conversationID int) ([]int, error) {
	query := `SELECT user_id FROM participants WHERE conversation_id = $1`

	rows, err := r.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []int
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}
