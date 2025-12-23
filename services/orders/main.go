package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
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

	// #region agent log
	agentLog("H1", "orders/main.go:34", "waitForDB.start", map[string]any{"db": cfg.DBURL})
	// #endregion
	if err := waitForDB(ctx, dbConn, 30*time.Second); err != nil {
		// #region agent log
		agentLog("H1", "orders/main.go:37", "waitForDB.fail", map[string]any{"err": err.Error()})
		// #endregion
		log.Fatalf("db not ready: %v", err)
	}
	// #region agent log
	agentLog("H1", "orders/main.go:42", "waitForDB.ok", map[string]any{})
	// #endregion

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
		// #region agent log
		agentLog("H3", "orders/main.go:65", "declareQueues.fail", map[string]any{"err": err.Error()})
		// #endregion
		log.Fatalf("declare queues: %v", err)
	}
	// #region agent log
	agentLog("H3", "orders/main.go:70", "declareQueues.ok", map[string]any{})
	// #endregion

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
		// #region agent log
		agentLog("H2", "orders/main.go:87", "rabbit.dial.try", map[string]any{"url": url})
		// #endregion
		conn, err := amqp.Dial(url)
		if err == nil {
			// #region agent log
			agentLog("H2", "orders/main.go:91", "rabbit.dial.ok", map[string]any{})
			// #endregion
			return conn, nil
		}
		if time.Now().After(deadline) {
			// #region agent log
			agentLog("H2", "orders/main.go:96", "rabbit.dial.timeout", map[string]any{"err": err.Error()})
			// #endregion
			return nil, err
		}
		time.Sleep(time.Second)
	}
}

// agentLog appends NDJSON diagnostics to the debug log file.
// #region agent log
func agentLog(hid, loc, msg string, data map[string]any) {
	entry := map[string]any{
		"sessionId":    "debug-session",
		"runId":        "run1",
		"hypothesisId": hid,
		"location":     loc,
		"message":      msg,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(`d:\WebShop\.cursor\debug.log`, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("AGENT %s %s %s data=%v (fileerr=%v)", hid, loc, msg, data, err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
	log.Printf("AGENT %s %s %s data=%v", hid, loc, msg, data)
}

// #endregion

