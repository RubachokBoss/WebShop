package mq

import (
	"context"
	"encoding/json"
	"log"

	"github.com/example/webshop/payments/internal/payment"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OrderConsumer struct {
	svc     *payment.Service
	channel *amqp.Channel
}

func NewOrderConsumer(svc *payment.Service, ch *amqp.Channel) *OrderConsumer {
	return &OrderConsumer{svc: svc, channel: ch}
}

func (c *OrderConsumer) Run(ctx context.Context) error {
	msgs, err := c.channel.Consume("order.payments", "payments-worker", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				log.Printf("order.payments channel closed")
				return nil
			}
			var task payment.PaymentTask
			if err := json.Unmarshal(d.Body, &task); err != nil {
				log.Printf("bad payment task: %v", err)
				_ = d.Nack(false, false)
				continue
			}
			if err := c.svc.ProcessPayment(ctx, task); err != nil {
				log.Printf("process payment: %v", err)
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

