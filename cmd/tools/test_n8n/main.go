package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	url := "http://localhost:8080/n8n/webhook"
	token := os.Getenv("API_TOKEN")

	lead := map[string]interface{}{
		"source":         "n8n-test",
		"title":          "Manual Lead from n8n",
		"location":       "Richmond, BC",
		"project_value":  750000,
		"priority_score": 9,
		"notes":          "Test lead for n8n integration verification.",
	}

	body, _ := json.Marshal(lead)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Result: %v\n", result)
}
