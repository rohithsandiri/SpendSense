package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rohithsandiri/spendsense-backend/internal/httputil"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
	"github.com/rohithsandiri/spendsense-backend/internal/middleware"
	"github.com/rohithsandiri/spendsense-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	DB   *pgxpool.Pool
	Auth *middleware.Auth
}

func New(db *pgxpool.Pool, auth *middleware.Auth) *Handler {
	return &Handler{DB: db, Auth: auth}
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string          `json:"token"`
	User  models.AuthUser `json:"user"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if err := httputil.Decode(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))

	var exists bool
	if err := h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, email).Scan(&exists); err != nil {
		logger.Error("Register error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}
	if exists {
		httputil.Error(w, http.StatusBadRequest, "User already exists")
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(body.Password), 10)
	if err != nil {
		logger.Error("Register error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	logger.Info("Attempting to register new user: " + email)
	var user models.AuthUser
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO users (name, email, password) VALUES ($1, $2, $3)
		 RETURNING id, name, email, role`,
		strings.TrimSpace(body.Name), email, string(hashed),
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role)
	if err != nil {
		logger.Error("Register error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	token, err := h.Auth.Sign(user.ID, user.Role)
	if err != nil {
		logger.Error("Register error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	logger.Info("New user registered: " + email)
	httputil.JSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body credentials
	if err := httputil.Decode(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))

	var (
		user     models.AuthUser
		password string
		isActive bool
	)
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, email, role, password, is_active FROM users WHERE email=$1`, email,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role, &password, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.Error(w, http.StatusBadRequest, "Invalid credentials")
		return
	}
	if err != nil {
		logger.Error("Login error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	if !isActive {
		httputil.Error(w, http.StatusForbidden, "Account is disabled")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(password), []byte(body.Password)) != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid credentials")
		return
	}

	token, err := h.Auth.Sign(user.ID, user.Role)
	if err != nil {
		logger.Error("Login error: " + err.Error())
		httputil.Error(w, http.StatusInternalServerError, "Server error")
		return
	}

	logger.Info("User logged in: " + email)
	httputil.JSON(w, http.StatusOK, authResponse{Token: token, User: user})
}
