package export

import (
	"fmt"
	"net/http"
	"strings"
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

// CSV streams the user's expenses as a CSV attachment.
func (h *Handler) CSV(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT date, type, category, amount, tags, notes FROM expenses WHERE user_id=$1 ORDER BY date DESC`, user.ID)
	if err != nil {
		logger.Error("Export CSV error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	var b strings.Builder
	b.WriteString("Date,Type,Category,Amount,Tags,Notes\n")
	for rows.Next() {
		var (
			date     time.Time
			typ      string
			category string
			amount   float64
			tags     []string
			notes    string
		)
		if err := rows.Scan(&date, &typ, &category, &amount, &tags, &notes); err != nil {
			logger.Error("Export CSV error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		fmt.Fprintf(&b, "%s,%s,%s,%v,%s,%s\n",
			date.Format("1/2/2006"), typ, category, amount, strings.Join(tags, " | "), notes)
	}

	logger.Info("CSV exported for user: " + user.ID)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=spendsense_expenses.csv")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(strings.TrimRight(b.String(), "\n")))
}
