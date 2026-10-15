package activity

import "time"

// EngineEvent is a low-volume operational event emitted by the engine.
// Ring-buffer capped at 20000 rows.
type EngineEvent struct {
	ID        string    `json:"id"`
	TS        time.Time `json:"ts"`
	Level     string    `json:"level"`
	Scope     string    `json:"scope"`
	ServiceID string    `json:"serviceId"`
	ProjectID string    `json:"projectId"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Message   string    `json:"message"`
	Data      string    `json:"data,omitempty"`
}

// ActivityEntry is a user-facing audit trail record.
type ActivityEntry struct {
	ID        string    `json:"id"`
	TS        time.Time `json:"ts"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Entity    string    `json:"entity"`
	EntityID  string    `json:"entityId"`
	ProjectID string    `json:"projectId"`
	ServiceID string    `json:"serviceId"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
}
