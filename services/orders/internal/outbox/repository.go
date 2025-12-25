package outbox

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// DBTX — общий интерфейс под DB или транзакцию
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type Message struct {
	ID        uuid.UUID
	Payload   []byte
	CreatedAt time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, tx DBTX, id uuid.UUID, orderID int64, userID string, amount int64, payload []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO outbox(id, order_id, user_id, amount, payload)
		VALUES ($1,$2,$3,$4,$5)
	`, id, orderID, userID, amount, payload)
	return err
}

// FetchPending возвращает пачку сообщений под этот транзакционный лок
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
		if err := rows.Scan(&m.ID, &m.Payload, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *Repository) MarkPublished(ctx context.Context, tx DBTX, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `UPDATE outbox SET published_at = now() WHERE id = $1`, id)
	return err
}
