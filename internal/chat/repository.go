package chat

import (
	"context"
	"database/sql"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SaveMessage(ctx context.Context, userID int, content string) error {
	query := "INSERT INTO messages (user_id, content) VALUES ($1, $2)"
	_, err := r.db.ExecContext(ctx, query, userID, content)
	return err
}

func (r *Repository) GetRecentMessages(ctx context.Context) ([]*Message, error) {
	query := `
		SELECT m.id, m.content, m.created_at, m.user_id, u.username
		FROM messages m
		JOIN users u ON m.user_id = u.id
		ORDER BY m.created_at DESC 
		LIMIT 50
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		if err := rows.Scan(&msg.ID, &msg.Content, &msg.CreatedAt, &msg.UserID, &msg.Username); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
