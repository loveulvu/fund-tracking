package main

import (
	"net/http"
	"os"
	"strings"
)

func requireUpdateAPIKey(r *http.Request) bool {
	expectedKey := configuredUpdateAPIKey()
	if expectedKey == "" {
		return false
	}

	providedKey := strings.TrimSpace(r.Header.Get("X-Update-Key"))
	return providedKey != "" && providedKey == expectedKey
}

func configuredUpdateAPIKey() string {
	return strings.TrimSpace(os.Getenv("UPDATE_API_KEY"))
}
