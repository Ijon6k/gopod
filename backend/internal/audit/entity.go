package audit

import "time"

// AuditLog represents an infrastructure or security event.
type AuditLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	Category  string    `json:"category"`
	IP        string    `json:"ip"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	TimeAgo   string    `json:"timeAgo"`
}
