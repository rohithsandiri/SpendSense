package notify

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is satisfied by both *pgxpool.Pool and pgx.Tx.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Create inserts a notification for a user. notifType must be one of the allowed enum values.
func Create(ctx context.Context, db Execer, userID, message, notifType string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO notifications (user_id, message, type) VALUES ($1, $2, $3)`,
		userID, message, notifType,
	)
	return err
}
