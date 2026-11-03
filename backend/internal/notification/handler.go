package notification

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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, user_id, message, type, is_read, created_at, updated_at
		 FROM notifications WHERE user_id=$1 ORDER BY created_at DESC`, user.ID)
	if err != nil {
		logger.Error("Get notifications error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	defer rows.Close()

	notifications := []models.Notification{}
	for rows.Next() {
		var (
			n      models.Notification
			userID string
		)
		if err := rows.Scan(&n.ID, &userID, &n.Message, &n.Type, &n.IsRead, &n.CreatedAt, &n.UpdatedAt); err != nil {
			logger.Error("Get notifications error: " + err.Error())
			httputil.Error(w, http.StatusInternalServerError, "Server error")
			return
		}
		n.User = userID
		notifications = append(notifications, n)
	}
	httputil.JSON(w, http.StatusOK, notifications)
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id := chi.URLParam(r, "id")

	tag, err := h.DB.Exec(r.Context(),
		`UPDATE notifications SET is_read=TRUE, updated_at=now() WHERE id=$1 AND user_id=$2`, id, user.ID)
	if err != nil {
		logger.Error("Mark as read error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	if tag.RowsAffected() == 0 {
		httputil.Error(w, http.StatusNotFound, "Notification not found")
		return
	}

	logger.Info("Notification marked as read: " + id)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	if _, err := h.DB.Exec(r.Context(),
		`UPDATE notifications SET is_read=TRUE, updated_at=now() WHERE user_id=$1 AND is_read=FALSE`, user.ID); err != nil {
		logger.Error("Mark all as read error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	logger.Info("All notifications marked as read for user: " + user.ID)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}
