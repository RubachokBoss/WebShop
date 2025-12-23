package payment

import (
	"context"
	"database/sql"
)

// DBTX shared subset of *sql.DB / *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const (
	StatusFinished  = "FINISHED"
	StatusCancelled = "CANCELLED"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Exists(ctx context.Context, tx DBTX, orderID int64) (bool, error) {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM payments WHERE order_id=$1`, orderID).Scan(&status)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) Insert(ctx context.Context, tx DBTX, orderID int64, userID string, amount int64, status string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO payments(order_id, user_id, amount, status)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT DO NOTHING
	`, orderID, userID, amount, status)
	return err
}

