package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gopod/internal/db"
	"gopod/pkg/crypto"
)

func TestNotificationsCRUDAndDelivery(t *testing.T) {
	tmp := t.TempDir()
	if err := crypto.InitMasterKey(tmp); err != nil {
		t.Fatalf("InitMasterKey: %v", err)
	}
	database, err := db.Open(filepath.Join(tmp, "notif.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	var discordGot, telegramGot string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(r.URL.Path, "/discord") {
			discordGot = string(body)
		} else if strings.Contains(r.URL.Path, "/sendMessage") {
			telegramGot = string(body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := NewService(database.DB)
	svc.allowTestHTTP = true
	svc.allowTestLoopback = true
	svc.telegramAPIBase = srv.URL

	ctx := context.Background()

	// SSRF protection verifies blocked hosts even if scheme matches
	if _, err := validateWebhookURL("https://169.254.169.254/latest/meta-data", false, false); err == nil {
		t.Fatal("expected metadata IP webhook to be rejected")
	}

	dCh, err := svc.CreateChannel(ctx, UpsertChannelRequest{
		Name:       "Homelab Discord",
		Provider:   "discord",
		WebhookURL: srv.URL + "/discord/secret-token-123",
		Events:     []string{EventDeployFailed},
	})
	if err != nil {
		t.Fatalf("CreateChannel discord failed: %v", err)
	}
	if strings.Contains(dCh.WebhookHint, "secret-token-123") {
		t.Fatalf("redaction leaked secret webhook token: %+v", dCh)
	}

	tCh, err := svc.CreateChannel(ctx, UpsertChannelRequest{
		Name:     "Ops Telegram",
		Provider: "telegram",
		BotToken: "12345:SECRETTOKEN",
		ChatID:   "-100123456",
		Events:   []string{EventBackupFailed},
	})
	if err != nil {
		t.Fatalf("CreateChannel telegram failed: %v", err)
	}

	if err := svc.TestChannel(ctx, dCh.ID); err != nil {
		t.Fatalf("TestChannel discord failed: %v", err)
	}
	if err := svc.TestChannel(ctx, tCh.ID); err != nil {
		t.Fatalf("TestChannel telegram failed: %v", err)
	}

	if !strings.Contains(discordGot, "GOPOD Notification Test") {
		t.Fatalf("expected discord payload, got: %s", discordGot)
	}
	var tgMap map[string]any
	if err := json.Unmarshal([]byte(telegramGot), &tgMap); err != nil || tgMap["chat_id"] != "-100123456" {
		t.Fatalf("unexpected telegram payload: %s (%v)", telegramGot, err)
	}

	list, err := svc.ListChannels(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListChannels got %d (%v), want 2", len(list), err)
	}
}

