package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	url := "http://localhost:8080/n8n/webhook"
	token := os.Getenv("API_TOKEN")

	req, err := newLeadRequest(url, token)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("Error: decode response: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Result: %v\n", result)
}

func newLeadRequest(url, token string) (*http.Request, error) {
	lead := map[string]interface{}{
		"source":         "n8n-test",
		"title":          "Manual Lead from n8n",
		"location":       "Richmond, BC",
		"project_value":  750000,
		"priority_score": 9,
		"notes":          "Test lead for n8n integration verification.",
	}

	body, err := json.Marshal(lead)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}
