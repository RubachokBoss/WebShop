package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/example/webshop/payments/internal/account"
	"github.com/example/webshop/payments/internal/config"
	"github.com/example/webshop/payments/internal/db"
	httpapi "github.com/example/webshop/payments/internal/http"
	"github.com/example/webshop/payments/internal/inbox"
	"github.com/example/webshop/payments/internal/mq"
	"github.com/example/webshop/payments/internal/outbox"
	"github.com/example/webshop/payments/internal/payment"
)

func main() {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	cfg := config.Load()

	dbConn, err := sql.Open("pgx", cfg.DBURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer dbConn.Close()

	if err := waitForDB(ctx, dbConn, 30*time.Second); err != nil {
		log.Fatalf("db not ready: %v", err)
	}

	if err := db.Migrate(ctx, dbConn); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	rmq, err := amqp.Dial(cfg.RabbitURL)
	if err != nil {
		log.Fatalf("connect rabbitmq: %v", err)
	}
	defer rmq.Close()

	ch, err := rmq.Channel()
	if err != nil {
		log.Fatalf("channel: %v", err)
	}
	defer ch.Close()

	if err := declareQueues(ch); err != nil {
		log.Fatalf("declare queues: %v", err)
	}

	accountRepo := account.NewRepository(dbConn)
	paymentRepo := payment.NewRepository(dbConn)
	inboxRepo := inbox.NewRepository(dbConn)
	outboxRepo := outbox.NewRepository(dbConn)

	paymentSvc := payment.NewService(dbConn, accountRepo, paymentRepo, inboxRepo, outboxRepo)

	orderConsumer := mq.NewOrderConsumer(paymentSvc, ch)
	outboxPublisher := mq.NewOutboxPublisher(dbConn, outboxRepo, ch)

	go func() {
		if err := orderConsumer.Run(ctx); err != nil {
			log.Fatalf("order consumer: %v", err)
		}
	}()
	go outboxPublisher.Run(ctx)

	handler := httpapi.NewHandler(accountRepo)
	r := chi.NewRouter()
	r.Mount("/payments", handler.Router())

	log.Printf("payments service listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func declareQueues(ch *amqp.Channel) error {
	if _, err := ch.QueueDeclare("order.payments", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare("payment.status", true, false, false, false, nil); err != nil {
		return err
	}
	return nil
}

func waitForDB(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return context.DeadlineExceeded
		}
		time.Sleep(time.Second)
	}
}

