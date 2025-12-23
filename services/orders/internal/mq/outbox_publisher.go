package mq

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/example/webshop/orders/internal/outbox"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OutboxPublisher struct {
	db      *sql.DB
	repo    *outbox.Repository
	channel *amqp.Channel
	limit   int
}

func NewOutboxPublisher(db *sql.DB, repo *outbox.Repository, ch *amqp.Channel) *OutboxPublisher {
	return &OutboxPublisher{db: db, repo: repo, channel: ch, limit: 20}
}

func (p *OutboxPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishBatch(ctx); err != nil {
				log.Printf("orders outbox publish error: %v", err)
			}
		}
	}
}

func (p *OutboxPublisher) publishBatch(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	msgs, err := p.repo.FetchPending(ctx, tx, p.limit)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return tx.Commit()
	}

	for _, msg := range msgs {
		if err := p.channel.PublishWithContext(ctx, "", "order.payments", false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         msg.Payload,
			DeliveryMode: amqp.Persistent,
			MessageId:    msg.ID.String(),
		}); err != nil {
			return err
		}
		if err := p.repo.MarkPublished(ctx, tx, msg.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

