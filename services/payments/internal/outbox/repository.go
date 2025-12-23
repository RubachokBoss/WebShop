package outbox

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// DBTX shared subset.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type Message struct {
	ID      uuid.UUID
	Payload []byte
	Created time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, tx DBTX, id uuid.UUID, payload []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO outbox(id, payload) VALUES ($1, $2)
	`, id, payload)
	return err
}

func (r *Repository) FetchPending(ctx context.Context, tx DBTX, limit int) ([]Message, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, payload, created_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Payload, &m.Created); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) MarkPublished(ctx context.Context, tx DBTX, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `UPDATE outbox SET published_at = now() WHERE id=$1`, id)
	return err
}

