package apitest

import (
	"net/http"
	"testing"
)

func TestRegister(t *testing.T) {
	reset(t)

	t.Run("creates a user and returns a token", func(t *testing.T) {
		status, body := do(t, http.MethodPost, "/api/auth/register", "",
			map[string]string{"name": "Yuvraj", "email": "yuvraj@test.com", "password": "password123"})

		if status != http.StatusCreated {
			t.Fatalf("status = %d, want 201", status)
		}
		if body["token"] == nil || body["token"] == "" {
			t.Error("expected a token in response")
		}
		user, _ := body["user"].(map[string]any)
		if user["email"] != "yuvraj@test.com" {
			t.Errorf("user.email = %v, want yuvraj@test.com", user["email"])
		}
		if _, leaked := user["password"]; leaked {
			t.Error("password must not be present in response")
		}
	})

	t.Run("returns 400 if user already exists", func(t *testing.T) {
		status, body := do(t, http.MethodPost, "/api/auth/register", "",
			map[string]string{"name": "Yuvraj", "email": "yuvraj@test.com", "password": "password123"})
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if body["message"] != "User already exists" {
			t.Errorf("message = %v, want 'User already exists'", body["message"])
		}
	})
}

func TestLogin(t *testing.T) {
	reset(t)
	registerUser(t, "Yuvraj", "yuvraj@test.com", "password123")

	t.Run("logs in with correct credentials", func(t *testing.T) {
		status, body := do(t, http.MethodPost, "/api/auth/login", "",
			map[string]string{"email": "yuvraj@test.com", "password": "password123"})
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if body["token"] == nil || body["token"] == "" {
			t.Error("expected a token")
		}
	})

	t.Run("returns 400 for wrong password", func(t *testing.T) {
		status, body := do(t, http.MethodPost, "/api/auth/login", "",
			map[string]string{"email": "yuvraj@test.com", "password": "wrongpassword"})
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if body["message"] != "Invalid credentials" {
			t.Errorf("message = %v, want 'Invalid credentials'", body["message"])
		}
	})

	t.Run("returns 400 for non-existent user", func(t *testing.T) {
		status, body := do(t, http.MethodPost, "/api/auth/login", "",
			map[string]string{"email": "ghost@test.com", "password": "password123"})
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if body["message"] != "Invalid credentials" {
			t.Errorf("message = %v, want 'Invalid credentials'", body["message"])
		}
	})
}
