package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	dbURL := getenv("DATABASE_URL", "postgres://catalog:catalog@catalog-db:5432/catalog?sslmode=disable")
	port := getenv("PORT", "8084")

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := waitForDB(ctx, db, 30*time.Second); err != nil {
		log.Fatalf("db not ready: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	r.Route("/categories", func(r chi.Router) {
		r.Get("/", listCategories(db))
		r.Post("/", createCategory(db))
		r.Put("/{id}", updateCategory(db))
		r.Delete("/{id}", deleteCategory(db))
	})

	r.Route("/products", func(r chi.Router) {
		r.Get("/", listProducts(db))
		r.Get("/{id}", getProduct(db))
		r.Post("/", createProduct(db))
		r.Put("/{id}", updateProduct(db))
		r.Delete("/{id}", deleteProduct(db))
	})

	log.Printf("catalog service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func migrate(ctx context.Context, db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS categories(
	id SERIAL PRIMARY KEY,
	title TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS products(
	id SERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT,
	price_cents BIGINT NOT NULL DEFAULT 0,
	currency TEXT NOT NULL DEFAULT 'RUB',
	category_id INT REFERENCES categories(id),
	image_url TEXT,
	stock INT NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT now()
);
`
	_, err := db.ExecContext(ctx, schema)
	return err
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

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type Category struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type Product struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	CategoryID  *int64 `json:"category_id,omitempty"`
	ImageURL    string `json:"image_url"`
	Stock       int64  `json:"stock"`
}

// Хендлеры — чтобы фронт жил и радовался

func listCategories(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.QueryContext(r.Context(), `SELECT id, title FROM categories ORDER BY title`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var res []Category
		for rows.Next() {
			var c Category
			if err := rows.Scan(&c.ID, &c.Title); err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			res = append(res, c)
		}
		writeJSON(w, res)
	}
}

func createCategory(db *sql.DB) http.HandlerFunc {
	type req struct {
		Title string `json:"title"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Title = strings.TrimSpace(body.Title)
		if body.Title == "" {
			http.Error(w, "title required", http.StatusBadRequest)
			return
		}
		var id int64
		if err := db.QueryRowContext(r.Context(), `INSERT INTO categories(title) VALUES ($1) RETURNING id`, body.Title).Scan(&id); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, Category{ID: id, Title: body.Title})
	}
}

func updateCategory(db *sql.DB) http.HandlerFunc {
	type req struct {
		Title string `json:"title"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Title = strings.TrimSpace(body.Title)
		if body.Title == "" {
			http.Error(w, "title required", http.StatusBadRequest)
			return
		}
		res, err := db.ExecContext(r.Context(), `UPDATE categories SET title=$1 WHERE id=$2`, body.Title, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, Category{ID: id, Title: body.Title})
	}
}

func deleteCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		res, err := db.ExecContext(r.Context(), `DELETE FROM categories WHERE id=$1`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listProducts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID := r.URL.Query().Get("category_id")
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		limit := parseIntDefault(r.URL.Query().Get("limit"), 20)
		offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
		if limit > 100 {
			limit = 100
		}

		sb := strings.Builder{}
		sb.WriteString(`SELECT id,title,description,price_cents,currency,category_id,image_url,stock FROM products WHERE 1=1`)
		var args []any
		idx := 1
		if categoryID != "" {
			sb.WriteString(" AND category_id=$" + strconv.Itoa(idx))
			id, _ := strconv.ParseInt(categoryID, 10, 64)
			args = append(args, id)
			idx++
		}
		if search != "" {
			sb.WriteString(" AND (LOWER(title) LIKE $" + strconv.Itoa(idx) + " OR LOWER(description) LIKE $" + strconv.Itoa(idx) + ")")
			args = append(args, "%"+strings.ToLower(search)+"%")
			idx++
		}
		sb.WriteString(" ORDER BY id DESC")
		sb.WriteString(" LIMIT $" + strconv.Itoa(idx))
		args = append(args, limit)
		idx++
		sb.WriteString(" OFFSET $" + strconv.Itoa(idx))
		args = append(args, offset)

		rows, err := db.QueryContext(r.Context(), sb.String(), args...)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var res []Product
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.PriceCents, &p.Currency, &p.CategoryID, &p.ImageURL, &p.Stock); err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			res = append(res, p)
		}
		writeJSON(w, res)
	}
}

func getProduct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var p Product
		err = db.QueryRowContext(r.Context(), `
			SELECT id,title,description,price_cents,currency,category_id,image_url,stock
			FROM products WHERE id=$1
		`, id).Scan(&p.ID, &p.Title, &p.Description, &p.PriceCents, &p.Currency, &p.CategoryID, &p.ImageURL, &p.Stock)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, p)
	}
}

func createProduct(db *sql.DB) http.HandlerFunc {
	type req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		PriceCents  int64  `json:"price_cents"`
		Currency    string `json:"currency"`
		CategoryID  *int64 `json:"category_id"`
		ImageURL    string `json:"image_url"`
		Stock       int64  `json:"stock"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Title = strings.TrimSpace(body.Title)
		if body.Title == "" || body.PriceCents < 0 || body.Stock < 0 {
			http.Error(w, "invalid fields", http.StatusBadRequest)
			return
		}
		if body.Currency == "" {
			body.Currency = "RUB"
		}
		var id int64
		err := db.QueryRowContext(r.Context(), `
			INSERT INTO products(title,description,price_cents,currency,category_id,image_url,stock)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id
		`, body.Title, body.Description, body.PriceCents, body.Currency, body.CategoryID, body.ImageURL, body.Stock).Scan(&id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, Product{
			ID:          id,
			Title:       body.Title,
			Description: body.Description,
			PriceCents:  body.PriceCents,
			Currency:    body.Currency,
			CategoryID:  body.CategoryID,
			ImageURL:    body.ImageURL,
			Stock:       body.Stock,
		})
	}
}

func updateProduct(db *sql.DB) http.HandlerFunc {
	type req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		PriceCents  int64  `json:"price_cents"`
		Currency    string `json:"currency"`
		CategoryID  *int64 `json:"category_id"`
		ImageURL    string `json:"image_url"`
		Stock       int64  `json:"stock"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Title = strings.TrimSpace(body.Title)
		if body.Title == "" || body.PriceCents < 0 || body.Stock < 0 {
			http.Error(w, "invalid fields", http.StatusBadRequest)
			return
		}
		if body.Currency == "" {
			body.Currency = "RUB"
		}
		res, err := db.ExecContext(r.Context(), `
			UPDATE products
			SET title=$1, description=$2, price_cents=$3, currency=$4, category_id=$5, image_url=$6, stock=$7
			WHERE id=$8
		`, body.Title, body.Description, body.PriceCents, body.Currency, body.CategoryID, body.ImageURL, body.Stock, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, Product{
			ID:          id,
			Title:       body.Title,
			Description: body.Description,
			PriceCents:  body.PriceCents,
			Currency:    body.Currency,
			CategoryID:  body.CategoryID,
			ImageURL:    body.ImageURL,
			Stock:       body.Stock,
		})
	}
}

func deleteProduct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		res, err := db.ExecContext(r.Context(), `DELETE FROM products WHERE id=$1`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// Хелперы, чтоб не копипастить одно и то же
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

