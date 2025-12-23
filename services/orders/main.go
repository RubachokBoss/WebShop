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

	"github.com/example/webshop/orders/internal/config"
	"github.com/example/webshop/orders/internal/db"
	httpapi "github.com/example/webshop/orders/internal/http"
	"github.com/example/webshop/orders/internal/mq"
	"github.com/example/webshop/orders/internal/order"
	"github.com/example/webshop/orders/internal/outbox"
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

	rmq, err := waitForRabbit(ctx, cfg.RabbitURL, 40*time.Second)
	if err != nil {
		log.Fatalf("rabbit not ready: %v", err)
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

	orderRepo := order.NewRepository(dbConn)
	outboxRepo := outbox.NewRepository(dbConn)
	svc := order.NewService(dbConn, orderRepo, outboxRepo)

	outboxPub := mq.NewOutboxPublisher(dbConn, outboxRepo, ch)
	statusConsumer := mq.NewPaymentStatusConsumer(svc, ch)

	go outboxPub.Run(ctx)
	go func() {
		if err := statusConsumer.Run(ctx); err != nil {
			log.Fatalf("payment status consumer: %v", err)
		}
	}()

	handler := httpapi.NewHandler(svc)
	r := chi.NewRouter()
	r.Mount("/", handler.Router())

	log.Printf("orders service listening on :%s", cfg.Port)
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

func waitForRabbit(ctx context.Context, url string, timeout time.Duration) (*amqp.Connection, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := amqp.Dial(url)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(time.Second)
	}
}

