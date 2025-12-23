package order

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/example/webshop/orders/internal/outbox"
)

type Service struct {
	db     *sql.DB
	repo   *Repository
	outbox *outbox.Repository
}

type PaymentTask struct {
	MessageID string `json:"message_id"`
	OrderID   int64  `json:"order_id"`
	UserID    string `json:"user_id"`
	Amount    int64  `json:"amount"`
}

func NewService(db *sql.DB, repo *Repository, outboxRepo *outbox.Repository) *Service {
	return &Service{db: db, repo: repo, outbox: outboxRepo}
}

func (s *Service) CreateOrder(ctx context.Context, userID string, amount int64, description string) (Order, []byte, uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Order{}, nil, uuid.Nil, err
	}
	defer tx.Rollback()

	orderID, err := s.repo.Create(ctx, tx, userID, amount, description)
	if err != nil {
		return Order{}, nil, uuid.Nil, err
	}

	messageID := uuid.New()
	payload, _ := json.Marshal(PaymentTask{
		MessageID: messageID.String(),
		OrderID:   orderID,
		UserID:    userID,
		Amount:    amount,
	})

	if err := s.outbox.Insert(ctx, tx, messageID, orderID, userID, amount, payload); err != nil {
		return Order{}, nil, uuid.Nil, err
	}

	if err := tx.Commit(); err != nil {
		return Order{}, nil, uuid.Nil, err
	}

	return Order{
		ID:          orderID,
		UserID:      userID,
		Amount:      amount,
		Description: description,
		Status:      StatusNew,
	}, payload, messageID, nil
}

func (s *Service) ListOrders(ctx context.Context, userID string) ([]Order, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) GetOrder(ctx context.Context, id int64) (Order, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) ApplyPaymentResult(ctx context.Context, orderID int64, status string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	target := StatusCancelled
	if status == StatusFinished {
		target = StatusFinished
	}

	if err := s.repo.UpdateStatus(ctx, tx, orderID, StatusNew, target); err != nil {
		return err
	}

	return tx.Commit()
}

