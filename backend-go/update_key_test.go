package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUpdateAPIKeyFailsClosedWhenMissing(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "")

	request := httptest.NewRequest(http.MethodGet, "/api/update", nil)
	if requireUpdateAPIKey(request) {
		t.Fatal("expected missing UPDATE_API_KEY to reject empty request")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/update", nil)
	request.Header.Set("X-Update-Key", "any-key")
	if requireUpdateAPIKey(request) {
		t.Fatal("expected missing UPDATE_API_KEY to reject header key")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/update?key=any-key", nil)
	if requireUpdateAPIKey(request) {
		t.Fatal("expected missing UPDATE_API_KEY to reject query key")
	}
}

func TestRequireUpdateAPIKeyHeaderFailsClosedWhenMissing(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "   ")

	request := httptest.NewRequest(http.MethodGet, "/api/alerts/check", nil)
	if requireUpdateAPIKeyHeader(request) {
		t.Fatal("expected blank UPDATE_API_KEY to reject empty request")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/alerts/check", nil)
	request.Header.Set("X-Update-Key", "any-key")
	if requireUpdateAPIKeyHeader(request) {
		t.Fatal("expected blank UPDATE_API_KEY to reject header key")
	}
}

func TestRequireUpdateAPIKeyAcceptsConfiguredHeaderAndQuery(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodGet, "/api/update", nil)
	request.Header.Set("X-Update-Key", "test-update-key")
	if !requireUpdateAPIKey(request) {
		t.Fatal("expected matching header key to pass")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/update?key=test-update-key", nil)
	if !requireUpdateAPIKey(request) {
		t.Fatal("expected matching query key to pass")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/update?key=wrong-key", nil)
	if requireUpdateAPIKey(request) {
		t.Fatal("expected wrong query key to be rejected")
	}
}

func TestRequireUpdateAPIKeyHeaderOnlyAcceptsConfiguredHeader(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodGet, "/api/alerts/send", nil)
	request.Header.Set("X-Update-Key", "test-update-key")
	if !requireUpdateAPIKeyHeader(request) {
		t.Fatal("expected matching header key to pass")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/alerts/send?key=test-update-key", nil)
	if requireUpdateAPIKeyHeader(request) {
		t.Fatal("expected query key to be rejected for header-only auth")
	}
}
