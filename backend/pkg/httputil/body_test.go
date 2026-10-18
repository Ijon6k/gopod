package httputil

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBodyRejectsOversizedBody(t *testing.T) {
	payload := `{"data":"` + strings.Repeat("a", int(DefaultJSONBodyLimit)) + `"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(payload))
	rec := httptest.NewRecorder()

	var target map[string]string
	err := DecodeJSONBody(rec, req, &target, 0)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
}

func TestDecodeJSONBodyAcceptsNormalPayload(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(`{"name":"app","count":3}`))
	rec := httptest.NewRecorder()

	var target struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := DecodeJSONBody(rec, req, &target, 0); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if target.Name != "app" || target.Count != 3 {
		t.Fatalf("unexpected decoded payload: %+v", target)
	}
}

func TestDecodeJSONBodyCustomLimit(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(`{"blob":"`+strings.Repeat("x", 64)+`"}`))
	rec := httptest.NewRecorder()

	var target map[string]string
	err := DecodeJSONBody(rec, req, &target, 32)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge with 32-byte limit, got %v", err)
	}
}

func TestWriteDecodeErrorStatusMapping(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteDecodeError(rec, ErrBodyTooLarge)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body must map to 413, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	WriteDecodeError(rec, errors.New("unexpected EOF"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body must map to 400, got %d", rec.Code)
	}
}
