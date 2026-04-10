package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestInventoryHandler_UpdatesRoomCount(t *testing.T) {
	// Reset room inventory
	roomInventory = 0

	form := url.Values{}
	form.Add("command", "/inventory")
	form.Add("text", "34")

	req := httptest.NewRequest("POST", "/slack/inventory", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	inventoryHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rr.Code)
	}

	if roomInventory != 34 {
		t.Errorf("expected roomInventory to be 34, got %v", roomInventory)
	}

	expectedBody := "Room count updated to 34"
	if !strings.Contains(rr.Body.String(), expectedBody) {
		t.Errorf("expected body to contain %q, got %q", expectedBody, rr.Body.String())
	}
}

func TestInventoryHandler_InvalidCount(t *testing.T) {
	form := url.Values{}
	form.Add("command", "/inventory")
	form.Add("text", "abc")

	req := httptest.NewRequest("POST", "/slack/inventory", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	inventoryHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", rr.Code)
	}
}

func TestInventoryHandler_ZeroCount(t *testing.T) {
	// Reset room inventory
	roomInventory = 10

	form := url.Values{}
	form.Add("command", "/inventory")
	form.Add("text", "0")

	req := httptest.NewRequest("POST", "/slack/inventory", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	inventoryHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status OK, got %v", rr.Code)
	}

	if roomInventory != 0 {
		t.Errorf("expected roomInventory to be 0, got %v", roomInventory)
	}
}
