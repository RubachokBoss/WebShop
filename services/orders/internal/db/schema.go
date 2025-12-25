package db

import (
	"context"
	"database/sql"
)

// Migrate создаёт нужные таблицы; можно смело звать на старте
func Migrate(ctx context.Context, db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS orders (
	id SERIAL PRIMARY KEY,
	user_id TEXT NOT NULL,
	amount BIGINT NOT NULL,
	description TEXT,
	status TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS outbox (
	id UUID PRIMARY KEY,
	order_id INT NOT NULL REFERENCES orders(id),
	user_id TEXT NOT NULL,
	amount BIGINT NOT NULL,
	payload JSONB NOT NULL,
	published_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS outbox_published_idx ON outbox(published_at, created_at);
`
	_, err := db.ExecContext(ctx, schema)
	return err
}

