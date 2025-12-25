package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbURL := getenv("DATABASE_URL", "postgres://users:users@users-db:5432/users?sslmode=disable")
	jwtSecret := getenv("JWT_SECRET", "changeme")
	port := getenv("PORT", "8085")

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := waitForDB(context.Background(), db, 30*time.Second); err != nil {
		log.Fatalf("db not ready: %v", err)
	}
	if err := migrate(context.Background(), db); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Post("/register", register(db))
	r.Post("/login", login(db, jwtSecret))

	log.Printf("users service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// DB + models

type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func migrate(ctx context.Context, db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users(
	id SERIAL PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
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

// Handlers

func register(db *sql.DB) http.HandlerFunc {
	type req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Email = strings.TrimSpace(strings.ToLower(body.Email))
		body.Name = strings.TrimSpace(body.Name)
		if body.Email == "" || body.Password == "" || body.Name == "" {
			http.Error(w, "email/password/name required", http.StatusBadRequest)
			return
		}
		if len(body.Password) < 6 {
			http.Error(w, "password too short", http.StatusBadRequest)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		var id int64
		err = db.QueryRowContext(r.Context(), `
			INSERT INTO users(email, password_hash, name)
			VALUES ($1,$2,$3)
			RETURNING id
		`, body.Email, string(hash), body.Name).Scan(&id)
		if err != nil {
			http.Error(w, "email exists or db error", http.StatusConflict)
			return
		}
		writeJSON(w, User{ID: id, Email: body.Email, Name: body.Name}, http.StatusCreated)
	}
}

func login(db *sql.DB, jwtSecret string) http.HandlerFunc {
	type req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type resp struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Email = strings.TrimSpace(strings.ToLower(body.Email))
		if body.Email == "" || body.Password == "" {
			http.Error(w, "email/password required", http.StatusBadRequest)
			return
		}
		var u User
		var hash string
		err := db.QueryRowContext(r.Context(), `
			SELECT id, email, name, password_hash FROM users WHERE email=$1
		`, body.Email).Scan(&u.ID, &u.Email, &u.Name, &hash)
		if err == sql.ErrNoRows {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		token, err := signJWT(jwtSecret, u)
		if err != nil {
			http.Error(w, "token error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp{Token: token, User: u}, http.StatusOK)
	}
}

// JWT
func signJWT(secret string, u User) (string, error) {
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(u.ID, 10),
		"email": u.Email,
		"name": u.Name,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// helpers
func writeJSON(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

