package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnrichFundsHandlerRequiresUpdateKeyHeader(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodPost, "/api/funds/enrich", nil)
	response := httptest.NewRecorder()

	enrichFundsHandler(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestEnrichFundsHandlerRejectsQueryKey(t *testing.T) {
	t.Setenv("UPDATE_API_KEY", "test-update-key")

	request := httptest.NewRequest(http.MethodPost, "/api/funds/enrich?key=test-update-key", nil)
	response := httptest.NewRecorder()

	enrichFundsHandler(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestBuildFundMetadataUpdateSkipsEmptyValues(t *testing.T) {
	metadata := fundMetadata{
		FundType:    "指数型",
		FundCompany: "",
		FundManager: "0",
		FundScale:   "12.34亿",
	}

	update := buildFundMetadataUpdate(metadata)

	if _, ok := update["fund_company"]; ok {
		t.Fatal("expected empty fund_company to be skipped")
	}
	if _, ok := update["fund_manager"]; ok {
		t.Fatal("expected invalid fund_manager to be skipped")
	}
	if update["fund_type"] != "指数型" {
		t.Fatalf("expected fund_type to be set, got %v", update["fund_type"])
	}
	if update["fund_scale"] != "12.34亿" {
		t.Fatalf("expected fund_scale to be set, got %v", update["fund_scale"])
	}
}
