package budget

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohithsandiri/spendsense-backend/internal/httputil"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
	"github.com/rohithsandiri/spendsense-backend/internal/middleware"
	"github.com/rohithsandiri/spendsense-backend/internal/models"
)

type Handler struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler { return &Handler{DB: db} }

type budgetInput struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Month    int     `json:"month"`
	Year     int     `json:"year"`
}

const budgetCols = `id, user_id, category, limit_amount, month, year, created_at, updated_at`

func scanBudget(row pgx.Row) (models.Budget, error) {
	var b models.Budget
	err := row.Scan(&b.ID, &b.User, &b.Category, &b.Limit, &b.Month, &b.Year, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

// Set upserts a budget for (user, category, month, year), matching the original update-or-create.
func (h *Handler) Set(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	var in budgetInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var existed bool
	if err := h.DB.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM budgets WHERE user_id=$1 AND category=$2 AND month=$3 AND year=$4)`,
		user.ID, in.Category, in.Month, in.Year,
	).Scan(&existed); err != nil {
		logger.Error("Set budget error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	budget, err := scanBudget(h.DB.QueryRow(r.Context(),
		`INSERT INTO budgets (user_id, category, limit_amount, month, year)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (user_id, category, month, year)
		 DO UPDATE SET limit_amount=EXCLUDED.limit_amount, updated_at=now()
		 RETURNING `+budgetCols,
		user.ID, in.Category, in.Limit, in.Month, in.Year,
	))
	if err != nil {
		logger.Error("Set budget error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	if existed {
		logger.Info("Budget updated for user: " + user.ID + ", category: " + in.Category)
		httputil.JSON(w, http.StatusOK, budget)
		return
	}
	logger.Info("Budget set for user: " + user.ID + ", category: " + in.Category)
	httputil.JSON(w, http.StatusCreated, budget)
}

// List returns budgets for a month/year with computed spent and percentage.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	now := time.Now()
	month := queryInt(r, "month", int(now.Month()))
	year := queryInt(r, "year", now.Year())

	rows, err := h.DB.Query(r.Context(),
		`SELECT `+budgetCols+` FROM budgets WHERE user_id=$1 AND month=$2 AND year=$3`,
		user.ID, month, year,
	)
	if err != nil {
		logger.Error("Get budgets error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	budgets := []models.Budget{}
	for rows.Next() {
		b, err := scanBudget(rows)
		if err != nil {
			logger.Error("Get budgets error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		budgets = append(budgets, b)
	}

	result := []models.BudgetProgress{}
	for _, b := range budgets {
		start := time.Date(b.Year, time.Month(b.Month), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)
		var spent float64
		if err := h.DB.QueryRow(r.Context(),
			`SELECT COALESCE(SUM(amount),0) FROM expenses
			 WHERE user_id=$1 AND type='expense' AND category=$2 AND date >= $3 AND date < $4`,
			b.User, b.Category, start, end,
		).Scan(&spent); err != nil {
			logger.Error("Get budgets error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		percentage := 0.0
		if b.Limit > 0 {
			percentage = math.Min((spent/b.Limit)*100, 100)
		}
		percentage = math.Round(percentage*100) / 100
		result = append(result, models.BudgetProgress{Budget: b, Spent: spent, Percentage: percentage})
	}

	httputil.JSON(w, http.StatusOK, result)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id := chi.URLParam(r, "id")

	tag, err := h.DB.Exec(r.Context(), `DELETE FROM budgets WHERE id=$1 AND user_id=$2`, id, user.ID)
	if err != nil {
		logger.Error("Delete budget error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.Error(w, http.StatusNotFound, "Budget not found")
		return
	}

	logger.Info("Budget deleted for user: " + user.ID + ", budgetId: " + id)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "Budget deleted successfully"})
}

func queryInt(r *http.Request, key string, fallback int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
