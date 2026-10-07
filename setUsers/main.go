package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"useDB/stores"
)

// Server holds shared dependencies (our sqlc database queries)
type Server struct {
	db *stores.Queries
}

func main() {
	// 1. Connect to DB
	dbURL := os.Getenv("WRITE_DB_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/mydb?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to Write DB: %v", err)
	}
	defer pool.Close()

	// 2. Setup Server & Routes
	server := &Server{db: stores.New(pool)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", server.handleCreateUser)
	mux.HandleFunc("PUT /users/{id}", server.handleUpdateUser)
	mux.HandleFunc("DELETE /users/{id}", server.handleDeleteUser)

	// 3. Start Server
	log.Println("setUsers running on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

// --- Request Structs ---

type UserReq struct {
	FullName string `json:"fullname"`
	Email    string `json:"email"`
}

// --- Handlers ---

// POST /users
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req UserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	user, err := s.db.CreateUser(r.Context(), stores.CreateUserParams{
		FullName: req.FullName,
		Email:    req.Email,
	})
	if err != nil {
		log.Printf("CreateUser DB Error: %v\n", err)
		http.Error(w, `{"error": "failed to create user"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// PUT /users/{id}
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var id pgtype.UUID
	if err := id.Scan(r.PathValue("id")); err != nil {
		http.Error(w, `{"error": "invalid UUID format"}`, http.StatusBadRequest)
		return
	}

	var req UserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid JSON"}`, http.StatusBadRequest)
		return
	}

	user, err := s.db.UpdateUser(r.Context(), stores.UpdateUserParams{
		ID:       id,
		FullName: req.FullName,
		Email:    req.Email,
	})
	if err != nil {
		http.Error(w, `{"error": "failed to update user"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// DELETE /users/{id}
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	var id pgtype.UUID
	if err := id.Scan(r.PathValue("id")); err != nil {
		http.Error(w, `{"error": "invalid UUID format"}`, http.StatusBadRequest)
		return
	}

	if err := s.db.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, `{"error": "failed to delete user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Helper ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
