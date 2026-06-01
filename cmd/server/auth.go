package main

import (
	"net/http"
	"strings"
)

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	return strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == token
}
