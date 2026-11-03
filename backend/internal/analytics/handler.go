package analytics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohithsandiri/spendsense-backend/internal/httputil"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
	"github.com/rohithsandiri/spendsense-backend/internal/middleware"
)

type Handler struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler { return &Handler{DB: db} }

// Shapes below mirror the MongoDB aggregation output the React app consumes.

type categoryRow struct {
	ID    string  `json:"_id"`
	Total float64 `json:"total"`
}

type monthKey struct {
	Month int `json:"month"`
	Year  int `json:"year"`
}

type monthlyRow struct {
	ID    monthKey `json:"_id"`
	Total float64  `json:"total"`
}

type typeKey struct {
	Month int    `json:"month"`
	Year  int    `json:"year"`
	Type  string `json:"type"`
}

type typeRow struct {
	ID    typeKey `json:"_id"`
	Total float64 `json:"total"`
}

func (h *Handler) CategoryBreakdown(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	now := time.Now()
	m := queryInt(r, "month", int(now.Month()))
	y := queryInt(r, "year", now.Year())
	start := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	rows, err := h.DB.Query(r.Context(),
		`SELECT category, SUM(amount) FROM expenses
		 WHERE user_id=$1 AND type='expense' AND date >= $2 AND date < $3
		 GROUP BY category`,
		user.ID, start, end,
	)
	if err != nil {
		logger.Error("Category breakdown error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	out := []categoryRow{}
	for rows.Next() {
		var c categoryRow
		if err := rows.Scan(&c.ID, &c.Total); err != nil {
			logger.Error("Category breakdown error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		out = append(out, c)
	}
	logger.Info("Category breakdown fetched for user: " + user.ID)
	httputil.JSON(w, http.StatusOK, out)
}

func (h *Handler) MonthlyTrend(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT EXTRACT(MONTH FROM date)::int, EXTRACT(YEAR FROM date)::int, SUM(amount)
		 FROM expenses WHERE user_id=$1 AND type='expense'
		 GROUP BY 1, 2 ORDER BY 2, 1`,
		user.ID,
	)
	if err != nil {
		logger.Error("Monthly trend error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	out := []monthlyRow{}
	for rows.Next() {
		var row monthlyRow
		if err := rows.Scan(&row.ID.Month, &row.ID.Year, &row.Total); err != nil {
			logger.Error("Monthly trend error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		out = append(out, row)
	}
	logger.Info("Monthly trend fetched for user: " + user.ID)
	httputil.JSON(w, http.StatusOK, out)
}

func (h *Handler) IncomeVsExpense(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT EXTRACT(MONTH FROM date)::int, EXTRACT(YEAR FROM date)::int, type, SUM(amount)
		 FROM expenses WHERE user_id=$1
		 GROUP BY 1, 2, type ORDER BY 2, 1`,
		user.ID,
	)
	if err != nil {
		logger.Error("Income vs expense error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	out := []typeRow{}
	for rows.Next() {
		var row typeRow
		if err := rows.Scan(&row.ID.Month, &row.ID.Year, &row.ID.Type, &row.Total); err != nil {
			logger.Error("Income vs expense error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		out = append(out, row)
	}
	logger.Info("Income vs expense fetched for user: " + user.ID)
	httputil.JSON(w, http.StatusOK, out)
}

func queryInt(r *http.Request, key string, fallback int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
