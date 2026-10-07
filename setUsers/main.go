package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"useDB/stores"
)

// Server holds service dependencies
type Server struct {
	dbStore *stores.Queries
}

// Request payload struct
type CreateUserRequest struct {
	FullName string `json:"fullname"`
	Email    string `json:"email"`
}

// Response payload struct
type CreateUserResponse struct {
	ID       pgtype.UUID `json:"id"`
	FullName string      `json:"name"`
	Email    string      `json:"email"`
}

func main() {
	ctx := context.Background()

	// 1. Get Write DB connection URL from environment variable
	dbURL := os.Getenv("WRITE_DB_URL")
	if dbURL == "" {
		// Fallback for local testing outside container
		dbURL = "postgres://postgres:postgres@localhost:5432/mydb?sslmode=disable"
	}

	// 2. Initialize pgx connection pool to the Write Primary DB
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to Write DB: %v\n", err)
	}
	defer dbPool.Close()

	// Verify connection
	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("Write DB ping failed: %v\n", err)
	}
	log.Println("Connected to Write Primary DB successfully.")

	// 3. Initialize server with sqlc queries store
	server := &Server{
		dbStore: stores.New(dbPool),
	}

	// 4. Setup HTTP routes (Go 1.22+ method-matching syntax)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", server.handleCreateUser)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("setUsers service running on port :%s ...\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v\n", err)
	}
}

// HTTP Handler for POST /users
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest

	// Decode JSON body from incoming client request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON request body"}`, http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.FullName == "" || req.Email == "" {
		http.Error(w, `{"error": "name and email are required fields"}`, http.StatusBadRequest)
		return
	}

	// Context timeout for database execution
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Call generated sqlc method on Write DB
	user, err := s.dbStore.CreateUser(ctx, stores.CreateUserParams{
		FullName: req.FullName,
		Email:    req.Email,
	})
	if err != nil {
		log.Printf("Failed to create user in DB: %v\n", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Send back JSON response
	resp := CreateUserResponse{
		ID:       user.ID,
		FullName: user.FullName,
		Email:    user.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v\n", err)
	}
}
