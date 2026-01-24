package user

import (
	"context"
	"database/sql"
	"errors"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, user *User) (*User, error) {
	var id int
	query := "INSERT INTO users (username, password) VALUES ($1, $2) RETURNING id"

	err := r.db.QueryRowContext(ctx, query, user.Username, user.Password).Scan(&id)
	if err != nil {
		return nil, err
	}

	user.ID = id
	return user, nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u := &User{}
	query := "SELECT id, username, password FROM users WHERE username = $1"

	err := r.db.QueryRowContext(ctx, query, username).Scan(&u.ID, &u.Username, &u.Password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return u, nil
}
func (r *Repository) SearchUsers(ctx context.Context, query string) ([]User, error) {
	// We limit to 10 to keep it fast
	q := `SELECT id, username FROM users WHERE username ILIKE $1 LIMIT 10`
	rows, err := r.db.QueryContext(ctx, q, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
