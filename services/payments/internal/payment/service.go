package payment

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/example/webshop/payments/internal/account"
	"github.com/example/webshop/payments/internal/inbox"
	"github.com/example/webshop/payments/internal/outbox"
)

type PaymentTask struct {
	MessageID string `json:"message_id"`
	OrderID   int64  `json:"order_id"`
	UserID    string `json:"user_id"`
	Amount    int64  `json:"amount"`
}

type PaymentResult struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

type Service struct {
	db         *sql.DB
	accounts   *account.Repository
	payments   *Repository
	inbox      *inbox.Repository
	outboxRepo *outbox.Repository
}

func NewService(db *sql.DB, accounts *account.Repository, payments *Repository, inbox *inbox.Repository, outboxRepo *outbox.Repository) *Service {
	return &Service{db: db, accounts: accounts, payments: payments, inbox: inbox, outboxRepo: outboxRepo}
}

// ProcessPayment — транзакционный инбокс+аутбокс с идемпотентностью, чтоб не ловить дубль списаний
func (s *Service) ProcessPayment(ctx context.Context, task PaymentTask) error {
	msgID, err := uuid.Parse(task.MessageID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Дедуп по инбоксу — одно сообщение, один заход
	ok, err := s.inbox.TryInsert(ctx, tx, msgID)
	if err != nil {
		return err
	}
	if !ok {
		return tx.Commit()
	}

	// Не списываем дважды за один заказ
	exists, err := s.payments.Exists(ctx, tx, task.OrderID)
	if err != nil {
		return err
	}
	if exists {
		return tx.Commit()
	}

	status := StatusCancelled
	reason := ""

	balance, err := s.accounts.BalanceForUpdate(ctx, tx, task.UserID)
	if err == sql.ErrNoRows {
		reason = "account not found"
	} else if err != nil {
		return err
	} else if balance < task.Amount {
		reason = "insufficient funds"
	} else {
		if err := s.accounts.Deduct(ctx, tx, task.UserID, task.Amount); err != nil {
			return err
		}
		status = StatusFinished
	}

	if err := s.payments.Insert(ctx, tx, task.OrderID, task.UserID, task.Amount, status); err != nil {
		return err
	}

	outID := uuid.New()
	payload, _ := json.Marshal(PaymentResult{
		OrderID: task.OrderID,
		Status:  status,
		Reason:  reason,
	})
	if err := s.outboxRepo.Insert(ctx, tx, outID, payload); err != nil {
		return err
	}

	return tx.Commit()
}

