package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	Conn *sql.DB // Exported so other packages can use it
}

func NewDatabase(dsn string) (*Database, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// Ping check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(25)
	conn.SetConnMaxLifetime(5 * time.Minute)
	return &Database{Conn: conn}, nil
}

func (d *Database) AutoMigrate() error {
	// 1. Define the Tables
	queries := []string{
		// Users Table
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL
		);`,

		// Messages Table
		`CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL REFERENCES users(id),
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	// 2. Execute them
	for _, query := range queries {
		_, err := d.Conn.Exec(query)
		if err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
