package db

import (
	"context"
	"database/sql"
)

// Migrate sets up tables for accounts, payments, inbox/outbox.
func Migrate(ctx context.Context, db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS accounts (
	user_id TEXT PRIMARY KEY,
	balance BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments (
	order_id INT PRIMARY KEY,
	user_id TEXT NOT NULL,
	amount BIGINT NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inbox (
	message_id UUID PRIMARY KEY,
	received_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS outbox (
	id UUID PRIMARY KEY,
	payload JSONB NOT NULL,
	published_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS outbox_published_idx ON outbox(published_at, created_at);
`
	_, err := db.ExecContext(ctx, schema)
	return err
}

