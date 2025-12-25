package order

import (
	"context"
	"database/sql"
	"time"
)

const (
	StatusNew       = "NEW"
	StatusFinished  = "FINISHED"
	StatusCancelled = "CANCELLED"
)

// DBTX прикидывается и *sql.DB, и *sql.Tx — общий контракт
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Order struct {
	ID          int64
	UserID      string
	Amount      int64
	Description string
	Status      string
	CreatedAt   time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, tx DBTX, userID string, amount int64, description string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO orders(user_id, amount, description, status)
		VALUES ($1,$2,$3,$4)
		RETURNING id
	`, userID, amount, description, StatusNew).Scan(&id)
	return id, err
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, amount, description, status, created_at
		FROM orders
		WHERE ($1 = '' OR user_id = $1)
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Amount, &o.Description, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, o)
	}
	return res, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id int64) (Order, error) {
	var o Order
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, amount, description, status, created_at
		FROM orders WHERE id=$1
	`, id).Scan(&o.ID, &o.UserID, &o.Amount, &o.Description, &o.Status, &o.CreatedAt)
	return o, err
}

func (r *Repository) UpdateStatus(ctx context.Context, tx DBTX, id int64, fromStatus, toStatus string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE orders
		SET status = $1
		WHERE id = $2 AND status = $3
	`, toStatus, id, fromStatus)
	return err
}

