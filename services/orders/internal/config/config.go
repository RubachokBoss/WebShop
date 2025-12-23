package config

import "os"

type Config struct {
	Port      string
	DBURL     string
	RabbitURL string
}

func Load() Config {
	return Config{
		Port:      getenv("PORT", "8081"),
		DBURL:     getenv("DATABASE_URL", "postgres://orders:orders@orders-db:5432/orders?sslmode=disable"),
		RabbitURL: getenv("RABBIT_URL", "amqp://guest:guest@rabbitmq:5672/"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

