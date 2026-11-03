package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohithsandiri/spendsense-backend/internal/admin"
	"github.com/rohithsandiri/spendsense-backend/internal/analytics"
	"github.com/rohithsandiri/spendsense-backend/internal/auth"
	"github.com/rohithsandiri/spendsense-backend/internal/budget"
	"github.com/rohithsandiri/spendsense-backend/internal/expense"
	"github.com/rohithsandiri/spendsense-backend/internal/export"
	"github.com/rohithsandiri/spendsense-backend/internal/group"
	"github.com/rohithsandiri/spendsense-backend/internal/httputil"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
	"github.com/rohithsandiri/spendsense-backend/internal/middleware"
	"github.com/rohithsandiri/spendsense-backend/internal/notification"
)

// New builds the full /api router, mirroring the original Express app.
func New(db *pgxpool.Pool, jwtSecret string) http.Handler {
	a := middleware.New(jwtSecret)

	authH := auth.New(db, a)
	expenseH := expense.New(db)
	budgetH := budget.New(db)
	groupH := group.New(db)
	analyticsH := analytics.New(db)
	notificationH := notification.New(db)
	adminH := admin.New(db)
	exportH := export.New(db)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(cors)
	r.Use(requestLogger)

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]string{"message": "SpendSense API is running"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			httputil.JSON(w, http.StatusOK, map[string]any{"status": "UP", "timestamp": time.Now()})
		})

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
		})

		r.Route("/expenses", func(r chi.Router) {
			r.Use(a.Protect)
			r.Post("/", expenseH.Add)
			r.Get("/", expenseH.List)
			r.Put("/{id}", expenseH.Update)
			r.Delete("/{id}", expenseH.Delete)
		})

		r.Route("/budgets", func(r chi.Router) {
			r.Use(a.Protect)
			r.Post("/", budgetH.Set)
			r.Get("/", budgetH.List)
			r.Delete("/{id}", budgetH.Delete)
		})

		r.Route("/groups", func(r chi.Router) {
			r.Use(a.Protect)
			r.Post("/", groupH.Create)
			r.Get("/", groupH.List)
			r.Post("/{id}/members", groupH.AddMember)
			r.Post("/{id}/expenses", groupH.AddExpense)
			r.Get("/{id}/expenses", groupH.ListExpenses)
			r.Put("/{id}/expenses/{expenseId}/split/{memberId}/pay", groupH.MarkSplitPaid)
			r.Post("/{id}/settle", groupH.SettleUp)
		})

		r.Route("/analytics", func(r chi.Router) {
			r.Use(a.Protect)
			r.Get("/category-breakdown", analyticsH.CategoryBreakdown)
			r.Get("/monthly-trend", analyticsH.MonthlyTrend)
			r.Get("/income-vs-expense", analyticsH.IncomeVsExpense)
		})

		r.Route("/notifications", func(r chi.Router) {
			r.Use(a.Protect)
			r.Get("/", notificationH.List)
			r.Put("/{id}/read", notificationH.MarkRead)
			r.Put("/read-all", notificationH.MarkAllRead)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(a.Protect)
			r.Use(a.AdminOnly)
			r.Get("/users", adminH.GetUsers)
			r.Put("/users/{id}/toggle", adminH.ToggleStatus)
			r.Get("/expenses", adminH.GetExpenses)
			r.Get("/notifications", adminH.GetNotifications)
		})

		r.Route("/export", func(r chi.Router) {
			r.Use(a.Protect)
			r.Get("/csv", exportH.CSV)
		})
	})

	return r
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requestLogger mirrors the Express finish-logging middleware.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		msg := "HTTP " + r.Method + " " + r.URL.Path
		attrs := []any{"method", r.Method, "url", r.URL.Path, "status", ww.Status()}
		if ww.Status() >= 400 {
			logger.Error(msg, attrs...)
		} else {
			logger.Info(msg, attrs...)
		}
	})
}
