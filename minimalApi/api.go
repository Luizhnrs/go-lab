package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

type user struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type userInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type api struct {
	db *sql.DB
}

func newAPI(db *sql.DB) http.Handler {
	a := &api{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("POST /users", a.createUser)
	mux.HandleFunc("GET /users", a.listUsers)
	mux.HandleFunc("GET /users/{id}", a.getUser)
	mux.HandleFunc("PUT /users/{id}", a.updateUser)
	mux.HandleFunc("DELETE /users/{id}", a.deleteUser)
	return loggingMiddleware(mux)
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeUserInput(w, r)
	if !ok {
		return
	}

	result, err := a.db.ExecContext(r.Context(),
		`INSERT INTO users (name, email) VALUES (?, ?)`,
		input.Name, input.Email,
	)
	if err != nil {
		a.writeDatabaseError(w, err)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível obter o usuário criado")
		return
	}

	created, err := a.findUser(r, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível consultar o usuário criado")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/users/%d", id))
	writeJSON(w, http.StatusCreated, created)
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, name, email, created_at, updated_at
		FROM users
		ORDER BY id
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível listar os usuários")
		return
	}
	defer rows.Close()

	users := make([]user, 0)
	for rows.Next() {
		var item user
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.CreatedAt, &item.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "não foi possível ler os usuários")
			return
		}
		users = append(users, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível ler os usuários")
		return
	}

	writeJSON(w, http.StatusOK, users)
}

func (a *api) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	found, err := a.findUser(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "usuário não encontrado")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível consultar o usuário")
		return
	}

	writeJSON(w, http.StatusOK, found)
}

func (a *api) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	input, ok := decodeUserInput(w, r)
	if !ok {
		return
	}

	result, err := a.db.ExecContext(r.Context(), `
		UPDATE users
		SET name = ?, email = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Name, input.Email, id)
	if err != nil {
		a.writeDatabaseError(w, err)
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível atualizar o usuário")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "usuário não encontrado")
		return
	}

	updated, err := a.findUser(r, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível consultar o usuário atualizado")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *api) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	result, err := a.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível excluir o usuário")
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível excluir o usuário")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "usuário não encontrado")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *api) findUser(r *http.Request, id int64) (user, error) {
	var found user
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id, name, email, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id).Scan(
		&found.ID,
		&found.Name,
		&found.Email,
		&found.CreatedAt,
		&found.UpdatedAt,
	)
	return found, err
}

func (a *api) writeDatabaseError(w http.ResponseWriter, err error) {
	if strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		writeError(w, http.StatusConflict, "já existe um usuário com este e-mail")
		return
	}
	writeError(w, http.StatusInternalServerError, "erro ao acessar o banco de dados")
}

func decodeUserInput(w http.ResponseWriter, r *http.Request) (userInput, bool) {
	var input userInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return userInput{}, false
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "o nome é obrigatório")
		return userInput{}, false
	}
	if len(input.Name) > 120 {
		writeError(w, http.StatusBadRequest, "o nome deve ter no máximo 120 caracteres")
		return userInput{}, false
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 {
		writeError(w, http.StatusBadRequest, "o e-mail é inválido")
		return userInput{}, false
	}

	return input, true
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s %s\n", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
