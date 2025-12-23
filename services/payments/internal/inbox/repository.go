package inbox

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// DBTX shared subset.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// TryInsert returns true if message id was newly inserted (not processed before).
func (r *Repository) TryInsert(ctx context.Context, tx DBTX, id uuid.UUID) (bool, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO inbox(message_id) VALUES ($1)
		ON CONFLICT DO NOTHING
	`, id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

