package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImportFundRouteRequiresAuth(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/funds/import", bytes.NewBufferString(`{"fundCode":"011839"}`))
	response := httptest.NewRecorder()

	authMiddleware(importFundHandler)(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestImportFundHandlerRejectsInvalidFundCode(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/funds/import", bytes.NewBufferString(`{"fundCode":"人工智能"}`))
	request = request.WithContext(context.WithValue(request.Context(), authClaimsKey, &AuthClaims{
		UserID: "test-user",
		Email:  "test@example.com",
	}))
	response := httptest.NewRecorder()

	importFundHandler(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestBuildExistingImportResponse(t *testing.T) {
	fund := Fund{
		FundCode: "011839",
		FundName: "天弘中证人工智能A",
	}

	response := buildExistingImportResponse(fund)

	if response.Status != "success" {
		t.Fatalf("expected success status, got %s", response.Status)
	}
	if response.Result != "existing" {
		t.Fatalf("expected existing result, got %s", response.Result)
	}
	if response.FundCode != "011839" {
		t.Fatalf("expected fundCode 011839, got %s", response.FundCode)
	}
	if response.Fund.FundName != fund.FundName {
		t.Fatalf("expected fund name %s, got %s", fund.FundName, response.Fund.FundName)
	}
}

func TestApplyMetadataToFundSkipsEmptyValues(t *testing.T) {
	fund := Fund{
		FundCode:    "011839",
		FundCompany: "已有公司",
	}

	applyMetadataToFund(&fund, fundMetadata{
		FundType:    "指数型-股票",
		FundCompany: "",
		FundManager: "0",
		FundScale:   "12095.02",
	})

	if fund.FundType != "指数型-股票" {
		t.Fatalf("expected fund type to be updated, got %s", fund.FundType)
	}
	if fund.FundCompany != "已有公司" {
		t.Fatalf("expected existing company to be preserved, got %s", fund.FundCompany)
	}
	if fund.FundManager != "" {
		t.Fatalf("expected invalid manager to be skipped, got %s", fund.FundManager)
	}
	if fund.FundScale != "12095.02" {
		t.Fatalf("expected fund scale to be updated, got %s", fund.FundScale)
	}
}
