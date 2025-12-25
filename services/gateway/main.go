package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
)

func main() {
	port := getenv("PORT", "8080")
	ordersURL := getenv("ORDERS_URL", "http://orders-service:8081")
	paymentsURL := getenv("PAYMENTS_URL", "http://payments-service:8082")
	frontendURL := getenv("FRONTEND_URL", "http://frontend:8083")
	catalogURL := getenv("CATALOG_URL", "http://catalog-service:8084")
	usersURL := getenv("USERS_URL", "http://users-service:8085")

	r := chi.NewRouter()
	r.Mount("/orders", http.StripPrefix("/orders", newProxy(ordersURL)))
	r.Mount("/payments", http.StripPrefix("/payments", newProxy(paymentsURL)))
	r.Mount("/catalog", http.StripPrefix("/catalog", newProxy(catalogURL)))
	r.Mount("/users", http.StripPrefix("/users", newProxy(usersURL)))
	// Фронт ловит всё, что не сматчилось выше
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		newProxy(frontendURL).ServeHTTP(w, req)
	})

	log.Printf("gateway listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("gateway server: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newProxy(raw string) http.Handler {
	target, err := url.Parse(raw)
	if err != nil {
		log.Fatalf("invalid proxy url %s: %v", raw, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy
}

