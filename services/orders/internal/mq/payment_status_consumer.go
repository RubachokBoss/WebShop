package mq

import (
	"context"
	"encoding/json"
	"log"

	"github.com/example/webshop/orders/internal/order"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentResult struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

type PaymentStatusConsumer struct {
	svc     *order.Service
	channel *amqp.Channel
}

func NewPaymentStatusConsumer(svc *order.Service, ch *amqp.Channel) *PaymentStatusConsumer {
	return &PaymentStatusConsumer{svc: svc, channel: ch}
}

func (c *PaymentStatusConsumer) Run(ctx context.Context) error {
	msgs, err := c.channel.Consume("payment.status", "orders-status", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				log.Printf("payment.status channel closed")
				return nil
			}
			var res PaymentResult
			if err := json.Unmarshal(d.Body, &res); err != nil {
				log.Printf("bad payment result: %v", err)
				_ = d.Nack(false, false)
				continue
			}
			if err := c.svc.ApplyPaymentResult(ctx, res.OrderID, res.Status); err != nil {
				log.Printf("apply payment result: %v", err)
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

