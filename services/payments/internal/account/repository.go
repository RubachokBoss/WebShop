package account

import (
	"context"
	"database/sql"
)

// DBTX shared subset of *sql.DB / *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateIfAbsent(ctx context.Context, userID string) (bool, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO accounts(user_id, balance) VALUES ($1, 0)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING user_id
	`, userID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) Deposit(ctx context.Context, userID string, amount int64) (int64, error) {
	var balance int64
	err := r.db.QueryRowContext(ctx, `
		UPDATE accounts SET balance = balance + $1
		WHERE user_id = $2
		RETURNING balance
	`, amount, userID).Scan(&balance)
	return balance, err
}

func (r *Repository) Balance(ctx context.Context, userID string) (int64, error) {
	var balance int64
	err := r.db.QueryRowContext(ctx, `
		SELECT balance FROM accounts WHERE user_id=$1
	`, userID).Scan(&balance)
	return balance, err
}

func (r *Repository) BalanceForUpdate(ctx context.Context, tx DBTX, userID string) (int64, error) {
	var balance int64
	err := tx.QueryRowContext(ctx, `
		SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE
	`, userID).Scan(&balance)
	return balance, err
}

func (r *Repository) Deduct(ctx context.Context, tx DBTX, userID string, amount int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE accounts SET balance = balance - $1 WHERE user_id=$2
	`, amount, userID)
	return err
}

