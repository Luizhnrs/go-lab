package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserCRUD(t *testing.T) {
	db, err := openDatabase("file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	server := httptest.NewServer(newAPI(db))
	defer server.Close()

	created := request(t, http.MethodPost, server.URL+"/users", `{
		"name": "Ada Lovelace",
		"email": "ada@example.com"
	}`)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("criação: esperado %d, recebido %d: %s",
			http.StatusCreated, created.StatusCode, created.Body)
	}

	var createdUser user
	if err := json.Unmarshal(created.Body, &createdUser); err != nil {
		t.Fatal(err)
	}
	if createdUser.ID == 0 || createdUser.Email != "ada@example.com" {
		t.Fatalf("usuário criado inválido: %+v", createdUser)
	}

	listed := request(t, http.MethodGet, server.URL+"/users", "")
	if listed.StatusCode != http.StatusOK {
		t.Fatalf("listagem: esperado %d, recebido %d", http.StatusOK, listed.StatusCode)
	}

	updated := request(t, http.MethodPut, server.URL+"/users/1", `{
		"name": "Ada Byron",
		"email": "ada.byron@example.com"
	}`)
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("atualização: esperado %d, recebido %d: %s",
			http.StatusOK, updated.StatusCode, updated.Body)
	}

	deleted := request(t, http.MethodDelete, server.URL+"/users/1", "")
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("exclusão: esperado %d, recebido %d", http.StatusNoContent, deleted.StatusCode)
	}

	missing := request(t, http.MethodGet, server.URL+"/users/1", "")
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("consulta após exclusão: esperado %d, recebido %d",
			http.StatusNotFound, missing.StatusCode)
	}
}

func TestDuplicateEmail(t *testing.T) {
	db, err := openDatabase("file:duplicatedb?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	server := httptest.NewServer(newAPI(db))
	defer server.Close()

	body := `{"name":"Grace Hopper","email":"grace@example.com"}`
	first := request(t, http.MethodPost, server.URL+"/users", body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("primeira criação: esperado %d, recebido %d", http.StatusCreated, first.StatusCode)
	}

	second := request(t, http.MethodPost, server.URL+"/users", body)
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("e-mail duplicado: esperado %d, recebido %d", http.StatusConflict, second.StatusCode)
	}
}

type response struct {
	StatusCode int
	Body       []byte
}

func request(t *testing.T, method, url, body string) response {
	t.Helper()

	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(res.Body); err != nil {
		t.Fatal(err)
	}
	return response{StatusCode: res.StatusCode, Body: buffer.Bytes()}
}
