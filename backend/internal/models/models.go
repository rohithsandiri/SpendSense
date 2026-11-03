package models

import "time"

// UserRef is a populated user reference, matching Mongoose `.populate('... ', 'name email')`.
type UserRef struct {
	ID    string `json:"_id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// AuthUser is the user object returned by register/login (note: `id`, not `_id`).
type AuthUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type Expense struct {
	ID        string    `json:"_id"`
	User      string    `json:"user"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Notes     string    `json:"notes"`
	Date      time.Time `json:"date"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AdminExpense is an expense with a populated user, for the admin view.
type AdminExpense struct {
	ID        string    `json:"_id"`
	User      UserRef   `json:"user"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Notes     string    `json:"notes"`
	Date      time.Time `json:"date"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Budget struct {
	ID        string    `json:"_id"`
	User      string    `json:"user"`
	Category  string    `json:"category"`
	Limit     float64   `json:"limit"`
	Month     int       `json:"month"`
	Year      int       `json:"year"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BudgetProgress adds computed spend to a budget, matching getBudgets output.
type BudgetProgress struct {
	Budget
	Spent      float64 `json:"spent"`
	Percentage float64 `json:"percentage"`
}

type Group struct {
	ID          string    `json:"_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   UserRef   `json:"createdBy"`
	Members     []UserRef `json:"members"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SplitDetail struct {
	ID         string     `json:"_id"`
	User       UserRef    `json:"user"`
	Amount     float64    `json:"amount"`
	Percentage float64    `json:"percentage"`
	IsPaid     bool       `json:"isPaid"`
	PaidAt     *time.Time `json:"paidAt"`
}

type GroupExpense struct {
	ID           string        `json:"_id"`
	Group        string        `json:"group"`
	PaidBy       UserRef       `json:"paidBy"`
	Description  string        `json:"description"`
	Amount       float64       `json:"amount"`
	SplitType    string        `json:"splitType"`
	SplitDetails []SplitDetail `json:"splitDetails"`
	Date         time.Time     `json:"date"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

type Notification struct {
	ID        string    `json:"_id"`
	User      any       `json:"user"` // string id for a user's own list; UserRef for admin
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	IsRead    bool      `json:"isRead"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
