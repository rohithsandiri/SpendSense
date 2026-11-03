package apitest

import (
	"net/http"
	"testing"
	"time"
)

func currentMonthYear() (int, int) {
	now := time.Now()
	return int(now.Month()), now.Year()
}

func TestBudgets(t *testing.T) {
	reset(t)
	token := registerUser(t, "Tester", "tester@example.com", "password123")
	month, year := currentMonthYear()

	t.Run("creates a new budget", func(t *testing.T) {
		status, body := do(t, http.MethodPost, "/api/budgets", token,
			map[string]any{"category": "Food", "limit": 5000, "month": month, "year": year})
		if status != http.StatusCreated {
			t.Fatalf("status = %d, want 201", status)
		}
		if body["category"] != "Food" {
			t.Errorf("category = %v, want Food", body["category"])
		}
		if body["limit"].(float64) != 5000 {
			t.Errorf("limit = %v, want 5000", body["limit"])
		}
	})

	t.Run("gets all budgets for a user", func(t *testing.T) {
		status, arr := doArray(t, http.MethodGet, "/api/budgets", token, nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if len(arr) != 1 {
			t.Fatalf("len = %d, want 1", len(arr))
		}
	})
}

func TestExpenses(t *testing.T) {
	t.Run("adds a new expense", func(t *testing.T) {
		reset(t)
		token := registerUser(t, "Tester", "tester@example.com", "password123")
		status, body := do(t, http.MethodPost, "/api/expenses", token,
			map[string]any{"type": "expense", "amount": 200, "category": "Food", "notes": "Lunch"})
		if status != http.StatusCreated {
			t.Fatalf("status = %d, want 201", status)
		}
		if body["amount"].(float64) != 200 {
			t.Errorf("amount = %v, want 200", body["amount"])
		}
		if body["category"] != "Food" {
			t.Errorf("category = %v, want Food", body["category"])
		}
	})

	t.Run("triggers a budget breach notification", func(t *testing.T) {
		reset(t)
		token := registerUser(t, "Tester", "tester@example.com", "password123")
		month, year := currentMonthYear()

		do(t, http.MethodPost, "/api/budgets", token,
			map[string]any{"category": "Food", "limit": 100, "month": month, "year": year})
		do(t, http.MethodPost, "/api/expenses", token,
			map[string]any{"type": "expense", "amount": 150, "category": "Food", "notes": "Expensive Dinner"})

		status, arr := doArray(t, http.MethodGet, "/api/notifications", token, nil)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		var breach map[string]any
		for _, n := range arr {
			if n["type"] == "budget_breach" {
				breach = n
			}
		}
		if breach == nil {
			t.Fatal("expected a budget_breach notification")
		}
		if msg, _ := breach["message"].(string); !contains(msg, "Budget breached for Food") {
			t.Errorf("message = %q, want it to contain 'Budget breached for Food'", msg)
		}
	})

	t.Run("triggers a large transaction notification", func(t *testing.T) {
		reset(t)
		token := registerUser(t, "Tester", "tester@example.com", "password123")

		do(t, http.MethodPost, "/api/expenses", token,
			map[string]any{"type": "expense", "amount": 12000, "category": "Shopping", "notes": "New Laptop"})

		_, arr := doArray(t, http.MethodGet, "/api/notifications", token, nil)
		found := false
		for _, n := range arr {
			if n["type"] == "large_transaction" {
				found = true
			}
		}
		if !found {
			t.Error("expected a large_transaction notification")
		}
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
