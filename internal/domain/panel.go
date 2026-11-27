package domain

import "time"

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

const (
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

type Grant struct {
	UserID int64  `json:"user_id"`
	Server string `json:"server"`
	Role   string `json:"role"`
}

type APIToken struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type JobEvent struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Type    string    `json:"type"`
	Percent int       `json:"percent,omitempty"`
	Message string    `json:"message,omitempty"`
}

type Job struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Server     string     `json:"server"`
	UserID     int64      `json:"user_id,omitempty"`
	Status     string     `json:"status"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Events     []JobEvent `json:"events,omitempty"`
}

type AuditEntry struct {
	ID       int64     `json:"id"`
	Time     time.Time `json:"time"`
	UserID   int64     `json:"user_id,omitempty"`
	Username string    `json:"username"`
	Action   string    `json:"action"`
	Server   string    `json:"server,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	IP       string    `json:"ip,omitempty"`
}
