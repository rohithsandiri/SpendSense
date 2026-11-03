package expense

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohithsandiri/spendsense-backend/internal/httputil"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
	"github.com/rohithsandiri/spendsense-backend/internal/middleware"
	"github.com/rohithsandiri/spendsense-backend/internal/models"
	"github.com/rohithsandiri/spendsense-backend/internal/notify"
)

type Handler struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler { return &Handler{DB: db} }

type expenseInput struct {
	Type     string     `json:"type"`
	Amount   float64    `json:"amount"`
	Category string     `json:"category"`
	Tags     []string   `json:"tags"`
	Notes    string     `json:"notes"`
	Date     *time.Time `json:"date"`
}

func scanExpense(row pgx.Row) (models.Expense, error) {
	var e models.Expense
	err := row.Scan(&e.ID, &e.User, &e.Type, &e.Amount, &e.Category, &e.Tags, &e.Notes, &e.Date, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

const expenseCols = `id, user_id, type, amount, category, tags, notes, date, created_at, updated_at`

func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	var in expenseInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	date := time.Now()
	if in.Date != nil {
		date = *in.Date
	}

	expense, err := scanExpense(h.DB.QueryRow(r.Context(),
		`INSERT INTO expenses (user_id, type, amount, category, tags, notes, date)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+expenseCols,
		user.ID, in.Type, in.Amount, in.Category, in.Tags, in.Notes, date,
	))
	if err != nil {
		logger.Error("Add expense error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	logger.Info(fmt.Sprintf("New %s added by user: %s, amount: %v, category: %s", in.Type, user.ID, in.Amount, in.Category))

	if in.Type == "expense" {
		h.checkBudgetAndAlert(r, user.ID, in.Category, in.Amount)
	}

	httputil.JSON(w, http.StatusCreated, expense)
}

// checkBudgetAndAlert replicates the budget breach/warning and large-transaction notifications.
func (h *Handler) checkBudgetAndAlert(r *http.Request, userID, category string, amount float64) {
	ctx := r.Context()
	now := time.Now()

	var limit float64
	err := h.DB.QueryRow(ctx,
		`SELECT limit_amount FROM budgets WHERE user_id=$1 AND category=$2 AND month=$3 AND year=$4`,
		userID, category, int(now.Month()), now.Year(),
	).Scan(&limit)
	if err == nil && limit > 0 {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)
		var spent float64
		if err := h.DB.QueryRow(ctx,
			`SELECT COALESCE(SUM(amount),0) FROM expenses
			 WHERE user_id=$1 AND type='expense' AND category=$2 AND date >= $3 AND date < $4`,
			userID, category, start, end,
		).Scan(&spent); err == nil {
			percentage := (spent / limit) * 100
			switch {
			case percentage >= 100:
				_ = notify.Create(ctx, h.DB, userID,
					fmt.Sprintf("Budget breached for %s! Spent ₹%v of ₹%v", category, spent, limit), "budget_breach")
				logger.Warn("Budget breached for user: " + userID + ", category: " + category)
			case percentage >= 80:
				_ = notify.Create(ctx, h.DB, userID,
					fmt.Sprintf("Warning! You have used 80%% of your %s budget", category), "budget_warning")
				logger.Warn("Budget warning for user: " + userID + ", category: " + category)
			}
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		logger.Error("Budget check error: " + err.Error())
	}

	if amount >= 10000 {
		_ = notify.Create(ctx, h.DB, userID,
			fmt.Sprintf("Large transaction detected! ₹%v spent on %s", amount, category), "large_transaction")
		logger.Warn(fmt.Sprintf("Large transaction detected for user: %s, amount: %v", userID, amount))
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	q := r.URL.Query()

	sql := `SELECT ` + expenseCols + ` FROM expenses WHERE user_id=$1`
	args := []any{user.ID}
	add := func(clause string, val any) {
		args = append(args, val)
		sql += fmt.Sprintf(clause, len(args))
	}
	if v := q.Get("category"); v != "" {
		add(" AND category=$%d", v)
	}
	if v := q.Get("type"); v != "" {
		add(" AND type=$%d", v)
	}
	if v := q.Get("tag"); v != "" {
		add(" AND $%d = ANY(tags)", v)
	}
	if s, e := q.Get("startDate"), q.Get("endDate"); s != "" && e != "" {
		if sd, err := time.Parse(time.RFC3339, s); err == nil {
			add(" AND date >= $%d", sd)
		}
		if ed, err := time.Parse(time.RFC3339, e); err == nil {
			add(" AND date <= $%d", ed)
		}
	}
	sql += " ORDER BY date DESC"

	rows, err := h.DB.Query(r.Context(), sql, args...)
	if err != nil {
		logger.Error("Get expenses error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	expenses := []models.Expense{}
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			logger.Error("Get expenses error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		expenses = append(expenses, e)
	}
	httputil.JSON(w, http.StatusOK, expenses)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id := chi.URLParam(r, "id")

	var in expenseInput
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}

	expense, err := scanExpense(h.DB.QueryRow(r.Context(),
		`UPDATE expenses SET type=$1, amount=$2, category=$3, tags=$4, notes=$5,
		   date=COALESCE($6, date), updated_at=now()
		 WHERE id=$7 AND user_id=$8 RETURNING `+expenseCols,
		in.Type, in.Amount, in.Category, in.Tags, in.Notes, in.Date, id, user.ID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "Expense not found")
		return
	}
	if err != nil {
		logger.Error("Update expense error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	logger.Info("Expense updated by user: " + user.ID + ", expenseId: " + id)
	httputil.JSON(w, http.StatusOK, expense)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id := chi.URLParam(r, "id")

	tag, err := h.DB.Exec(r.Context(), `DELETE FROM expenses WHERE id=$1 AND user_id=$2`, id, user.ID)
	if err != nil {
		logger.Error("Delete expense error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.Error(w, http.StatusNotFound, "Expense not found")
		return
	}

	logger.Info("Expense deleted by user: " + user.ID + ", expenseId: " + id)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "Expense deleted successfully"})
}
