package notifications

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"gopod/pkg/crypto"
)

var allowedEvents = map[string]bool{
	EventDeploySucceeded: true,
	EventDeployFailed:    true,
	EventBackupSucceeded: true,
	EventBackupFailed:    true,
	EventContainerAlert:  true,
}

// Service manages encrypted notification channels and dispatches alerts.
type Service struct {
	db                *sql.DB
	httpClient        *http.Client
	telegramAPIBase   string
	allowTestHTTP     bool
	allowTestLoopback bool
}

// NewService constructs a notifications Service backed by SQLite.
func NewService(db *sql.DB) *Service {
	return &Service{
		db:              db,
		httpClient:      &http.Client{Timeout: 8 * time.Second},
		telegramAPIBase: "https://api.telegram.org",
	}
}

func validateWebhookURL(raw string, allowTestHTTP, allowLoopback bool) (string, error) {
	clean := strings.TrimSpace(raw)
	u, err := url.Parse(clean)
	if err != nil {
		return "", fmt.Errorf("invalid webhook URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && !(allowTestHTTP && scheme == "http") {
		return "", errors.New("webhook URL must use https://")
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return "", errors.New("webhook URL missing hostname")
	}
	if !allowLoopback {
		if host == "localhost" || host == "metadata.google.internal" || strings.HasSuffix(host, ".local") {
			return "", fmt.Errorf("webhook host %q is not permitted", host)
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
				return "", fmt.Errorf("webhook IP %q is not permitted", host)
			}
		}
	}
	return clean, nil
}

func normalizeEvents(events []string) []string {
	if len(events) == 0 {
		return []string{EventDeployFailed, EventBackupFailed}
	}
	seen := make(map[string]bool, len(events))
	out := make([]string, 0, len(events))
	for _, ev := range events {
		clean := strings.TrimSpace(ev)
		if allowedEvents[clean] && !seen[clean] {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	if len(out) == 0 {
		return []string{EventDeployFailed, EventBackupFailed}
	}
	return out
}

func maskWebhookURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "Configured"
	}
	return fmt.Sprintf("%s://%s/•••", u.Scheme, u.Host)
}

func (s *Service) ListChannels(ctx context.Context) ([]Channel, error) {
	if s.db == nil {
		return []Channel{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, provider, config_enc, events_json, enabled, created_at
		FROM notification_channels
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Channel, 0)
	for rows.Next() {
		var ch Channel
		var cfgEnc, eventsJSON string
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Provider, &cfgEnc, &eventsJSON, &ch.Enabled, &ch.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(eventsJSON), &ch.Events)
		if cfgEnc != "" {
			if dec, derr := crypto.Decrypt(cfgEnc); derr == nil {
				var cfg ChannelConfig
				if json.Unmarshal([]byte(dec), &cfg) == nil {
					if cfg.WebhookURL != "" {
						ch.WebhookHint = maskWebhookURL(cfg.WebhookURL)
						ch.HasSecret = true
					}
					if cfg.ChatID != "" {
						ch.ChatID = cfg.ChatID
					}
					if cfg.BotToken != "" {
						ch.HasSecret = true
					}
				}
			}
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (s *Service) CreateChannel(ctx context.Context, req UpsertChannelRequest) (*Channel, error) {
	if s.db == nil {
		return nil, errors.New("notifications database unavailable")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("channel name is required")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	var cfg ChannelConfig
	switch provider {
	case "discord":
		urlVal, err := validateWebhookURL(req.WebhookURL, s.allowTestHTTP, s.allowTestLoopback)
		if err != nil {
			return nil, err
		}
		cfg.WebhookURL = urlVal
	case "telegram":
		token := strings.TrimSpace(req.BotToken)
		chatID := strings.TrimSpace(req.ChatID)
		if token == "" || chatID == "" {
			return nil, errors.New("telegram botToken and chatId are required")
		}
		if strings.ContainsAny(token, " \t\r\n/") {
			return nil, errors.New("invalid telegram bot token format")
		}
		cfg.BotToken = token
		cfg.ChatID = chatID
	default:
		return nil, fmt.Errorf("unsupported provider %q (supported: discord, telegram)", req.Provider)
	}

	rawCfg, _ := json.Marshal(cfg)
	cfgEnc, err := crypto.Encrypt(string(rawCfg))
	if err != nil {
		return nil, fmt.Errorf("encrypt notification config: %w", err)
	}

	events := normalizeEvents(req.Events)
	eventsJSON, _ := json.Marshal(events)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	id := "notif-" + hex.EncodeToString(b)

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_channels (id, name, provider, config_enc, events_json, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, name, provider, cfgEnc, string(eventsJSON), enabled); err != nil {
		return nil, err
	}

	ch := &Channel{
		ID:        id,
		Name:      name,
		Provider:  provider,
		Events:    events,
		Enabled:   enabled,
		ChatID:    cfg.ChatID,
		HasSecret: true,
		CreatedAt: time.Now(),
	}
	if cfg.WebhookURL != "" {
		ch.WebhookHint = maskWebhookURL(cfg.WebhookURL)
	}
	return ch, nil
}

func (s *Service) UpdateChannel(ctx context.Context, id string, req UpsertChannelRequest) (*Channel, error) {
	if s.db == nil {
		return nil, errors.New("notifications database unavailable")
	}
	var existingProvider, existingCfgEnc string
	var existingEnabled bool
	var existingName, existingEventsJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT name, provider, config_enc, events_json, enabled
		FROM notification_channels WHERE id = ?
	`, id).Scan(&existingName, &existingProvider, &existingCfgEnc, &existingEventsJSON, &existingEnabled)
	if err == sql.ErrNoRows {
		return nil, errors.New("notification channel not found")
	}
	if err != nil {
		return nil, err
	}

	var cfg ChannelConfig
	if dec, derr := crypto.Decrypt(existingCfgEnc); derr == nil {
		_ = json.Unmarshal([]byte(dec), &cfg)
	}

	if req.Name != "" {
		existingName = strings.TrimSpace(req.Name)
	}
	if existingProvider == "discord" && strings.TrimSpace(req.WebhookURL) != "" {
		urlVal, vErr := validateWebhookURL(req.WebhookURL, s.allowTestHTTP, s.allowTestLoopback)
		if vErr != nil {
			return nil, vErr
		}
		cfg.WebhookURL = urlVal
	}
	if existingProvider == "telegram" {
		if strings.TrimSpace(req.BotToken) != "" {
			if strings.ContainsAny(strings.TrimSpace(req.BotToken), " \t\r\n/") {
				return nil, errors.New("invalid telegram bot token format")
			}
			cfg.BotToken = strings.TrimSpace(req.BotToken)
		}
		if strings.TrimSpace(req.ChatID) != "" {
			cfg.ChatID = strings.TrimSpace(req.ChatID)
		}
	}
	if req.Events != nil {
		b, _ := json.Marshal(normalizeEvents(req.Events))
		existingEventsJSON = string(b)
	}
	if req.Enabled != nil {
		existingEnabled = *req.Enabled
	}

	rawCfg, _ := json.Marshal(cfg)
	cfgEnc, err := crypto.Encrypt(string(rawCfg))
	if err != nil {
		return nil, fmt.Errorf("encrypt notification config: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE notification_channels
		SET name = ?, config_enc = ?, events_json = ?, enabled = ?
		WHERE id = ?
	`, existingName, cfgEnc, existingEventsJSON, existingEnabled, id); err != nil {
		return nil, err
	}

	channels, err := s.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range channels {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, errors.New("updated channel not found")
}

func (s *Service) DeleteChannel(ctx context.Context, id string) error {
	if s.db == nil {
		return errors.New("notifications database unavailable")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("notification channel not found")
	}
	return nil
}

// TestChannel delivers a synchronous test alert and surfaces provider errors.
func (s *Service) TestChannel(ctx context.Context, id string) error {
	if s.db == nil {
		return errors.New("notifications database unavailable")
	}
	var name, provider, cfgEnc string
	err := s.db.QueryRowContext(ctx, `
		SELECT name, provider, config_enc FROM notification_channels WHERE id = ?
	`, id).Scan(&name, &provider, &cfgEnc)
	if err == sql.ErrNoRows {
		return errors.New("notification channel not found")
	}
	if err != nil {
		return err
	}
	dec, err := crypto.Decrypt(cfgEnc)
	if err != nil {
		return fmt.Errorf("decrypt channel credentials: %w", err)
	}
	var cfg ChannelConfig
	if err := json.Unmarshal([]byte(dec), &cfg); err != nil {
		return err
	}
	return s.sendProvider(ctx, provider, cfg, "test", "GOPOD Notification Test", fmt.Sprintf("Channel %q is connected and ready.", name), map[string]string{"Status": "OK"})
}

// NotifyAsync fans out matching events asynchronously with a bounded timeout.
func (s *Service) NotifyAsync(event, title, message string, fields map[string]string) {
	if s == nil || s.db == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rows, err := s.db.QueryContext(ctx, `
			SELECT name, provider, config_enc, events_json
			FROM notification_channels WHERE enabled = 1
		`)
		if err != nil {
			return
		}
		defer rows.Close()

		type target struct {
			name     string
			provider string
			cfg      ChannelConfig
		}
		var targets []target
		for rows.Next() {
			var name, provider, cfgEnc, evJSON string
			if err := rows.Scan(&name, &provider, &cfgEnc, &evJSON); err != nil {
				continue
			}
			var subscribed []string
			_ = json.Unmarshal([]byte(evJSON), &subscribed)
			matched := false
			for _, ev := range subscribed {
				if ev == event {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			dec, derr := crypto.Decrypt(cfgEnc)
			if derr != nil {
				continue
			}
			var cfg ChannelConfig
			if json.Unmarshal([]byte(dec), &cfg) != nil {
				continue
			}
			targets = append(targets, target{name: name, provider: provider, cfg: cfg})
		}

		for _, t := range targets {
			if err := s.sendProvider(ctx, t.provider, t.cfg, event, title, message, fields); err != nil {
				log.Printf("[notifications] channel %q (%s) delivery failed: %v", t.name, t.provider, err)
			}
		}
	}()
}

func (s *Service) sendProvider(ctx context.Context, provider string, cfg ChannelConfig, event, title, message string, fields map[string]string) error {
	switch provider {
	case "discord":
		color := 0x5E6AD2
		if strings.Contains(event, "failed") || strings.Contains(event, "alert") {
			color = 0xEF4444
		} else if strings.Contains(event, "succeeded") {
			color = 0x22C55E
		}
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		embedFields := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			if strings.TrimSpace(fields[k]) == "" {
				continue
			}
			embedFields = append(embedFields, map[string]any{
				"name":   k,
				"value":  fields[k],
				"inline": true,
			})
		}
		payload := map[string]any{
			"username": "GOPOD",
			"embeds": []map[string]any{
				{
					"title":       title,
					"description": message,
					"color":       color,
					"fields":      embedFields,
					"timestamp":   time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		return s.postJSON(ctx, cfg.WebhookURL, payload)

	case "telegram":
		var b strings.Builder
		fmt.Fprintf(&b, "*%s*\n%s", title, message)
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.TrimSpace(fields[k]) != "" {
				fmt.Fprintf(&b, "\n• %s: `%s`", k, fields[k])
			}
		}
		endpoint := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(s.telegramAPIBase, "/"), cfg.BotToken)
		payload := map[string]any{
			"chat_id":    cfg.ChatID,
			"text":       b.String(),
			"parse_mode": "Markdown",
		}
		return s.postJSON(ctx, endpoint, payload)

	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
}

func (s *Service) postJSON(ctx context.Context, targetURL string, payload any) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("provider returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

