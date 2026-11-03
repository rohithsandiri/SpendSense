package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rohithsandiri/spendsense-backend/internal/httputil"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
)

type ctxKey string

const userKey ctxKey = "user"

// AuthUser is the identity extracted from a valid JWT.
type AuthUser struct {
	ID   string
	Role string
}

// Auth validates JWTs and exposes Protect / AdminOnly middleware.
type Auth struct {
	Secret []byte
}

func New(secret string) *Auth {
	return &Auth{Secret: []byte(secret)}
}

// Sign issues a 7-day token carrying id and role, matching the Express payload.
func (a *Auth) Sign(id, role string) (string, error) {
	claims := jwt.MapClaims{
		"id":   id,
		"role": role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.Secret)
}

// Protect rejects requests without a valid Bearer token.
func (a *Auth) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[1] == "" {
			httputil.Error(w, http.StatusUnauthorized, "No token, access denied")
			return
		}

		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.Secret, nil
		})
		if err != nil {
			logger.Error("Auth middleware error: " + err.Error())
			httputil.Error(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		id, _ := claims["id"].(string)
		role, _ := claims["role"].(string)
		ctx := context.WithValue(r.Context(), userKey, AuthUser{ID: id, Role: role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly must be chained after Protect.
func (a *Auth) AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFrom(r.Context()).Role != "admin" {
			httputil.Error(w, http.StatusForbidden, "Access denied, admin only")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFrom returns the authenticated user stored by Protect.
func UserFrom(ctx context.Context) AuthUser {
	u, _ := ctx.Value(userKey).(AuthUser)
	return u
}
