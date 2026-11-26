package notifications

import "time"

const (
	EventDeploySucceeded = "deploy.succeeded"
	EventDeployFailed    = "deploy.failed"
	EventBackupSucceeded = "backup.succeeded"
	EventBackupFailed    = "backup.failed"
	EventContainerAlert  = "container.alert"
)

// ChannelConfig stores provider-specific credentials serialized and encrypted
// at rest via pkg/crypto.
type ChannelConfig struct {
	WebhookURL string `json:"webhookUrl,omitempty"`
	BotToken   string `json:"botToken,omitempty"`
	ChatID     string `json:"chatId,omitempty"`
}

// Channel represents a sanitized notification destination exposed over HTTP.
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider"` // "discord" | "telegram"
	Events      []string  `json:"events"`
	Enabled     bool      `json:"enabled"`
	WebhookHint string    `json:"webhookHint,omitempty"`
	ChatID      string    `json:"chatId,omitempty"`
	HasSecret   bool      `json:"hasSecret"`
	CreatedAt   time.Time `json:"createdAt"`
}

// UpsertChannelRequest creates or updates a notification channel.
type UpsertChannelRequest struct {
	Name       string   `json:"name"`
	Provider   string   `json:"provider"`
	WebhookURL string   `json:"webhookUrl"`
	BotToken   string   `json:"botToken"`
	ChatID     string   `json:"chatId"`
	Events     []string `json:"events"`
	Enabled    *bool    `json:"enabled"`
}

