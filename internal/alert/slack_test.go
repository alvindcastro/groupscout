package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackAlerter_PostMessage(t *testing.T) {
	ts := "123.456"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat.postMessage" {
			t.Errorf("Expected path /chat.postMessage, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected bearer token, got %s", r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if req["channel"] != "test-channel" {
			t.Errorf("Expected channel test-channel, got %v", req["channel"])
		}

		// Verify Block Kit presence
		if req["blocks"] == nil {
			t.Errorf("Expected blocks in request, got nil")
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": true, "ts": "123.456"}`)
	}))
	defer server.Close()

	alerter := &SlackAlerter{
		botToken: "test-token",
		channel:  "test-channel",
		client:   server.Client(),
		baseURL:  server.URL,
	}

	msg := AlertMessage{
		AirportCode:  "CYVR",
		Cause:        "Heavy Snow",
		State:        "🔴 Active (45 min)",
		Cancelled:    12,
		TotalFlights: 40,
	}
	returnedTS, err := alerter.PostMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
	if returnedTS != ts {
		t.Errorf("Expected ts %s, got %s", ts, returnedTS)
	}
}

func TestSlackAlerter_UpdateMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.update" {
			t.Errorf("Expected path /chat.update, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if req["ts"] != "123.456" {
			t.Errorf("Expected ts 123.456, got %v", req["ts"])
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": true}`)
	}))
	defer server.Close()

	alerter := &SlackAlerter{
		botToken: "test-token",
		channel:  "test-channel",
		client:   server.Client(),
		baseURL:  server.URL,
	}

	err := alerter.UpdateMessage(context.Background(), "123.456", AlertMessage{AirportCode: "CYVR"})
	if err != nil {
		t.Fatalf("UpdateMessage failed: %v", err)
	}
}

func TestSlackAlerter_SendResolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.postMessage" {
			t.Errorf("Expected path /chat.postMessage (threaded resolve), got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if req["thread_ts"] != "123.456" {
			t.Errorf("Expected thread_ts 123.456, got %v", req["thread_ts"])
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ok": true}`)
	}))
	defer server.Close()

	alerter := &SlackAlerter{
		botToken: "test-token",
		channel:  "test-channel",
		client:   server.Client(),
		baseURL:  server.URL,
	}

	summary := ResolveSummary{
		AirportCode:   "CYVR",
		TotalDuration: 120,
		FinalSPS:      15.5,
	}
	err := alerter.SendResolve(context.Background(), "123.456", summary)
	if err != nil {
		t.Fatalf("SendResolve failed: %v", err)
	}
}
