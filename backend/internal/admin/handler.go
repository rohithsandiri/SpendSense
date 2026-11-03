package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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

type adminUser struct {
	ID        string `json:"_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt"`
}

func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, email, role, is_active, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') FROM users ORDER BY created_at DESC`)
	if err != nil {
		logger.Error("Get all users error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	users := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
			logger.Error("Get all users error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		users = append(users, u)
	}
	logger.Info("Admin fetched all users: " + user.ID)
	httputil.JSON(w, http.StatusOK, users)
}

func (h *Handler) ToggleStatus(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())
	id := chi.URLParam(r, "id")

	var (
		isActive bool
		email    string
	)
	err := h.DB.QueryRow(r.Context(),
		`UPDATE users SET is_active = NOT is_active, updated_at=now() WHERE id=$1 RETURNING is_active, email`, id,
	).Scan(&isActive, &email)
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "User not found")
		return
	}

	status := "disabled"
	if isActive {
		status = "enabled"
	}
	logger.Info("Admin " + admin.ID + " " + status + " user: " + email)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "User account " + status + " successfully"})
}

func (h *Handler) GetExpenses(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT e.id, e.type, e.amount, e.category, e.tags, e.notes, e.date, e.created_at, e.updated_at,
		        u.id, u.name, u.email
		 FROM expenses e JOIN users u ON u.id = e.user_id
		 ORDER BY e.created_at DESC`)
	if err != nil {
		logger.Error("Get all expenses error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	expenses := []models.AdminExpense{}
	for rows.Next() {
		var e models.AdminExpense
		if err := rows.Scan(&e.ID, &e.Type, &e.Amount, &e.Category, &e.Tags, &e.Notes, &e.Date, &e.CreatedAt, &e.UpdatedAt,
			&e.User.ID, &e.User.Name, &e.User.Email); err != nil {
			logger.Error("Get all expenses error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		expenses = append(expenses, e)
	}
	logger.Info("Admin fetched all expenses: " + user.ID)
	httputil.JSON(w, http.StatusOK, expenses)
}

func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT n.id, n.message, n.type, n.is_read, n.created_at, n.updated_at,
		        u.id, u.name, u.email
		 FROM notifications n JOIN users u ON u.id = n.user_id
		 ORDER BY n.created_at DESC`)
	if err != nil {
		logger.Error("Get all notifications error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	notifications := []models.Notification{}
	for rows.Next() {
		var (
			n   models.Notification
			ref models.UserRef
		)
		if err := rows.Scan(&n.ID, &n.Message, &n.Type, &n.IsRead, &n.CreatedAt, &n.UpdatedAt,
			&ref.ID, &ref.Name, &ref.Email); err != nil {
			logger.Error("Get all notifications error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		n.User = ref
		notifications = append(notifications, n)
	}
	logger.Info("Admin fetched all notifications: " + user.ID)
	httputil.JSON(w, http.StatusOK, notifications)
}
