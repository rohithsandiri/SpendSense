package group

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ---- Create / list ----

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		serverError(w, "Create group", err)
		return
	}
	defer tx.Rollback(r.Context())

	var groupID string
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO groups (name, created_by, description) VALUES ($1,$2,$3) RETURNING id`,
		in.Name, user.ID, in.Description,
	).Scan(&groupID); err != nil {
		serverError(w, "Create group", err)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`INSERT INTO group_members (group_id, user_id) VALUES ($1,$2)`, groupID, user.ID); err != nil {
		serverError(w, "Create group", err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, "Create group", err)
		return
	}

	logger.Info("Group created by user: " + user.ID + ", groupName: " + in.Name)
	group, err := h.loadGroup(r.Context(), groupID)
	if err != nil {
		serverError(w, "Create group", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, group)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	ids := []string{}
	rows, err := h.DB.Query(r.Context(),
		`SELECT g.id FROM groups g JOIN group_members gm ON gm.group_id = g.id
		 WHERE gm.user_id=$1 ORDER BY g.created_at DESC`, user.ID)
	if err != nil {
		serverError(w, "Get groups", err)
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			serverError(w, "Get groups", err)
			return
		}
		ids = append(ids, id)
	}
	rows.Close()

	groups := []models.Group{}
	for _, id := range ids {
		g, err := h.loadGroup(r.Context(), id)
		if err != nil {
			serverError(w, "Get groups", err)
			return
		}
		groups = append(groups, g)
	}
	httputil.JSON(w, http.StatusOK, groups)
}

// loadGroup returns a group with populated createdBy and members.
func (h *Handler) loadGroup(ctx context.Context, groupID string) (models.Group, error) {
	var g models.Group
	err := h.DB.QueryRow(ctx,
		`SELECT g.id, g.name, g.description, g.created_at, g.updated_at,
		        u.id, u.name, u.email
		 FROM groups g JOIN users u ON u.id = g.created_by WHERE g.id=$1`, groupID,
	).Scan(&g.ID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt,
		&g.CreatedBy.ID, &g.CreatedBy.Name, &g.CreatedBy.Email)
	if err != nil {
		return g, err
	}

	g.Members = []models.UserRef{}
	rows, err := h.DB.Query(ctx,
		`SELECT u.id, u.name, u.email FROM group_members gm JOIN users u ON u.id = gm.user_id
		 WHERE gm.group_id=$1`, groupID)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	for rows.Next() {
		var m models.UserRef
		if err := rows.Scan(&m.ID, &m.Name, &m.Email); err != nil {
			return g, err
		}
		g.Members = append(g.Members, m)
	}
	return g, nil
}

// ---- Members ----

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	groupID := chi.URLParam(r, "id")
	var in struct {
		Email string `json:"email"`
	}
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var createdBy, groupName string
	err := h.DB.QueryRow(r.Context(), `SELECT created_by, name FROM groups WHERE id=$1`, groupID).Scan(&createdBy, &groupName)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "Group not found")
		return
	}
	if err != nil {
		serverError(w, "Add member", err)
		return
	}
	if createdBy != user.ID {
		httputil.Error(w, http.StatusForbidden, "Only group creator can add members")
		return
	}

	var memberID string
	err = h.DB.QueryRow(r.Context(), `SELECT id FROM users WHERE email=$1`, in.Email).Scan(&memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		serverError(w, "Add member", err)
		return
	}

	var member bool
	if err := h.DB.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`, groupID, memberID).Scan(&member); err != nil {
		serverError(w, "Add member", err)
		return
	}
	if member {
		httputil.Error(w, http.StatusBadRequest, "User is already a member")
		return
	}

	if _, err := h.DB.Exec(r.Context(),
		`INSERT INTO group_members (group_id, user_id) VALUES ($1,$2)`, groupID, memberID); err != nil {
		serverError(w, "Add member", err)
		return
	}
	_ = notify.Create(r.Context(), h.DB, memberID, "You have been added to group: "+groupName, "group_activity")

	logger.Info("Member added to group: " + groupName + ", member: " + in.Email)
	group, err := h.loadGroup(r.Context(), groupID)
	if err != nil {
		serverError(w, "Add member", err)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"message": "Member added successfully", "group": group})
}

// ---- Group expenses ----

func (h *Handler) AddExpense(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	groupID := chi.URLParam(r, "id")

	var in struct {
		Description  string          `json:"description"`
		Amount       float64         `json:"amount"`
		SplitType    string          `json:"splitType"`
		SplitDetails json.RawMessage `json:"splitDetails"`
	}
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var groupName string
	err := h.DB.QueryRow(r.Context(), `SELECT name FROM groups WHERE id=$1`, groupID).Scan(&groupName)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "Group not found")
		return
	}
	if err != nil {
		serverError(w, "Add group expense", err)
		return
	}

	var isMember bool
	if err := h.DB.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`, groupID, user.ID).Scan(&isMember); err != nil {
		serverError(w, "Add group expense", err)
		return
	}
	if !isMember {
		httputil.Error(w, http.StatusForbidden, "You are not a member of this group")
		return
	}

	splits, errMsg := computeSplits(in.SplitType, in.Amount, in.SplitDetails, user.ID)
	if errMsg != "" {
		httputil.Error(w, http.StatusBadRequest, errMsg)
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		serverError(w, "Add group expense", err)
		return
	}
	defer tx.Rollback(r.Context())

	var expenseID string
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO group_expenses (group_id, paid_by, description, amount, split_type)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		groupID, user.ID, in.Description, in.Amount, in.SplitType,
	).Scan(&expenseID); err != nil {
		serverError(w, "Add group expense", err)
		return
	}

	for _, s := range splits {
		var paidAt *time.Time
		if s.IsPaid {
			now := time.Now()
			paidAt = &now
		}
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO split_details (group_expense_id, user_id, amount, percentage, is_paid, paid_at)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			expenseID, s.UserID, s.Amount, s.Percentage, s.IsPaid, paidAt,
		); err != nil {
			serverError(w, "Add group expense", err)
			return
		}
		if s.UserID != user.ID {
			_ = notify.Create(r.Context(), tx, s.UserID,
				fmt.Sprintf("You owe ₹%v for \"%s\" in group \"%s\"", s.Amount, in.Description, groupName), "group_activity")
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, "Add group expense", err)
		return
	}

	logger.Info(fmt.Sprintf("Group expense added: %s, amount: %v, splitType: %s", in.Description, in.Amount, in.SplitType))
	expense, err := h.loadGroupExpense(r.Context(), expenseID)
	if err != nil {
		serverError(w, "Add group expense", err)
		return
	}
	httputil.JSON(w, http.StatusCreated, expense)
}

type computedSplit struct {
	UserID     string
	Amount     float64
	Percentage float64
	IsPaid     bool
}

// computeSplits validates and builds split rows. Returns a user-facing error message on failure.
func computeSplits(splitType string, amount float64, raw json.RawMessage, payerID string) ([]computedSplit, string) {
	switch splitType {
	case "equal":
		var memberIDs []string
		if err := json.Unmarshal(raw, &memberIDs); err != nil || len(memberIDs) == 0 {
			return nil, "Invalid split details"
		}
		n := float64(len(memberIDs))
		splitAmount := round2(amount / n)
		pct := round2(100 / n)
		out := make([]computedSplit, 0, len(memberIDs))
		for _, id := range memberIDs {
			out = append(out, computedSplit{UserID: id, Amount: splitAmount, Percentage: pct, IsPaid: id == payerID})
		}
		return out, ""

	case "unequal":
		var items []struct {
			UserID string  `json:"userId"`
			Amount float64 `json:"amount"`
		}
		if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
			return nil, "Invalid split details"
		}
		total := 0.0
		for _, it := range items {
			total += it.Amount
		}
		if math.Round(total) != math.Round(amount) {
			return nil, fmt.Sprintf("Split amounts must add up to ₹%v", amount)
		}
		out := make([]computedSplit, 0, len(items))
		for _, it := range items {
			out = append(out, computedSplit{
				UserID: it.UserID, Amount: it.Amount,
				Percentage: round2((it.Amount / amount) * 100), IsPaid: it.UserID == payerID,
			})
		}
		return out, ""

	case "percentage":
		var items []struct {
			UserID     string  `json:"userId"`
			Percentage float64 `json:"percentage"`
		}
		if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
			return nil, "Invalid split details"
		}
		total := 0.0
		for _, it := range items {
			total += it.Percentage
		}
		if math.Round(total) != 100 {
			return nil, "Percentages must add up to 100%"
		}
		out := make([]computedSplit, 0, len(items))
		for _, it := range items {
			out = append(out, computedSplit{
				UserID: it.UserID, Amount: round2((it.Percentage / 100) * amount),
				Percentage: it.Percentage, IsPaid: it.UserID == payerID,
			})
		}
		return out, ""
	}
	return nil, "Invalid split type"
}

func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	ids := []string{}
	rows, err := h.DB.Query(r.Context(),
		`SELECT id FROM group_expenses WHERE group_id=$1 ORDER BY date DESC`, groupID)
	if err != nil {
		serverError(w, "Get group expenses", err)
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			serverError(w, "Get group expenses", err)
			return
		}
		ids = append(ids, id)
	}
	rows.Close()

	expenses := []models.GroupExpense{}
	for _, id := range ids {
		e, err := h.loadGroupExpense(r.Context(), id)
		if err != nil {
			serverError(w, "Get group expenses", err)
			return
		}
		expenses = append(expenses, e)
	}
	httputil.JSON(w, http.StatusOK, expenses)
}

func (h *Handler) loadGroupExpense(ctx context.Context, expenseID string) (models.GroupExpense, error) {
	var e models.GroupExpense
	err := h.DB.QueryRow(ctx,
		`SELECT ge.id, ge.group_id, ge.description, ge.amount, ge.split_type, ge.date, ge.created_at, ge.updated_at,
		        u.id, u.name, u.email
		 FROM group_expenses ge JOIN users u ON u.id = ge.paid_by WHERE ge.id=$1`, expenseID,
	).Scan(&e.ID, &e.Group, &e.Description, &e.Amount, &e.SplitType, &e.Date, &e.CreatedAt, &e.UpdatedAt,
		&e.PaidBy.ID, &e.PaidBy.Name, &e.PaidBy.Email)
	if err != nil {
		return e, err
	}

	e.SplitDetails = []models.SplitDetail{}
	rows, err := h.DB.Query(ctx,
		`SELECT sd.id, sd.amount, sd.percentage, sd.is_paid, sd.paid_at, u.id, u.name, u.email
		 FROM split_details sd JOIN users u ON u.id = sd.user_id WHERE sd.group_expense_id=$1`, expenseID)
	if err != nil {
		return e, err
	}
	defer rows.Close()
	for rows.Next() {
		var s models.SplitDetail
		if err := rows.Scan(&s.ID, &s.Amount, &s.Percentage, &s.IsPaid, &s.PaidAt,
			&s.User.ID, &s.User.Name, &s.User.Email); err != nil {
			return e, err
		}
		e.SplitDetails = append(e.SplitDetails, s)
	}
	return e, nil
}

// ---- Settlements ----

func (h *Handler) MarkSplitPaid(w http.ResponseWriter, r *http.Request) {
	expenseID := chi.URLParam(r, "expenseId")
	memberID := chi.URLParam(r, "memberId")

	var (
		amount    float64
		desc      string
		paidBy    string
		groupName string
	)
	err := h.DB.QueryRow(r.Context(),
		`SELECT sd.amount, ge.description, ge.paid_by, g.name
		 FROM split_details sd
		 JOIN group_expenses ge ON ge.id = sd.group_expense_id
		 JOIN groups g ON g.id = ge.group_id
		 WHERE sd.group_expense_id=$1 AND sd.user_id=$2`, expenseID, memberID,
	).Scan(&amount, &desc, &paidBy, &groupName)
	if errors.Is(err, pgx.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "Split not found")
		return
	}
	if err != nil {
		serverError(w, "Mark split as paid", err)
		return
	}

	if _, err := h.DB.Exec(r.Context(),
		`UPDATE split_details SET is_paid=TRUE, paid_at=now() WHERE group_expense_id=$1 AND user_id=$2`,
		expenseID, memberID); err != nil {
		serverError(w, "Mark split as paid", err)
		return
	}
	_ = notify.Create(r.Context(), h.DB, paidBy,
		fmt.Sprintf("A member has paid their share of ₹%v for \"%s\" in group \"%s\"", amount, desc, groupName), "group_activity")

	logger.Info("Split marked as paid for expense: " + expenseID + ", member: " + memberID)
	httputil.JSON(w, http.StatusOK, map[string]string{"message": "Split marked as paid"})
}

func (h *Handler) SettleUp(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	groupID := chi.URLParam(r, "id")
	var in struct {
		MemberID string `json:"memberId"`
	}
	if err := httputil.Decode(r, &in); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		serverError(w, "Settle up", err)
		return
	}
	defer tx.Rollback(r.Context())

	// Settle all unpaid splits where the current user owes memberId (memberId paid the expense).
	var total float64
	var count int
	err = tx.QueryRow(r.Context(),
		`WITH settled AS (
		   UPDATE split_details sd SET is_paid=TRUE, paid_at=now()
		   FROM group_expenses ge
		   WHERE sd.group_expense_id = ge.id
		     AND ge.group_id=$1 AND ge.paid_by=$2
		     AND sd.user_id=$3 AND sd.is_paid=FALSE
		   RETURNING sd.amount
		 )
		 SELECT COALESCE(SUM(amount),0), COUNT(*) FROM settled`,
		groupID, in.MemberID, user.ID,
	).Scan(&total, &count)
	if err != nil {
		serverError(w, "Settle up", err)
		return
	}

	if count == 0 {
		httputil.Error(w, http.StatusBadRequest, "No pending splits found")
		return
	}

	var groupName, settlerName string
	_ = tx.QueryRow(r.Context(), `SELECT name FROM groups WHERE id=$1`, groupID).Scan(&groupName)
	_ = tx.QueryRow(r.Context(), `SELECT name FROM users WHERE id=$1`, user.ID).Scan(&settlerName)
	if settlerName == "" {
		settlerName = "A member"
	}
	_ = notify.Create(r.Context(), tx, in.MemberID,
		fmt.Sprintf("%s has settled up ₹%.2f with you in group \"%s\"", settlerName, total, groupName), "group_activity")

	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, "Settle up", err)
		return
	}

	logger.Info(fmt.Sprintf("Settle up: user %s settled ₹%v with %s in group %s", user.ID, total, in.MemberID, groupID))
	httputil.JSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("Settled up ₹%.2f successfully", total)})
}

func serverError(w http.ResponseWriter, ctx string, err error) {
	logger.Error(ctx + " error: " + err.Error())
	httputil.Error(w, http.StatusInternalServerError, "Server error")
}
