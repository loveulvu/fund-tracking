package main

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestImportFundRouteRequiresAuth(t *testing.T) {
	response := performGinRequest(
		http.MethodPost,
		"/api/funds/import",
		bytes.NewBufferString(`{"fundCode":"011839"}`),
		ginAuthMiddleware(),
		importFundGinHandler,
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestImportFundHandlerRejectsInvalidFundCode(t *testing.T) {
	response := performGinRequest(
		http.MethodPost,
		"/api/funds/import",
		bytes.NewBufferString(`{"fundCode":"invalid"}`),
		func(c *gin.Context) {
			c.Set("authClaims", &AuthClaims{
				UserID: "test-user",
				Email:  "test@example.com",
			})
			c.Next()
		},
		importFundGinHandler,
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestBuildExistingImportResponse(t *testing.T) {
	fund := Fund{
		FundCode: "011839",
		FundName: "test fund",
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
		FundCompany: "existing company",
	}

	applyMetadataToFund(&fund, fundMetadata{
		FundType:    "index stock",
		FundCompany: "",
		FundManager: "0",
		FundScale:   "12095.02",
	})

	if fund.FundType != "index stock" {
		t.Fatalf("expected fund type to be updated, got %s", fund.FundType)
	}
	if fund.FundCompany != "existing company" {
		t.Fatalf("expected existing company to be preserved, got %s", fund.FundCompany)
	}
	if fund.FundManager != "" {
		t.Fatalf("expected invalid manager to be skipped, got %s", fund.FundManager)
	}
	if fund.FundScale != "12095.02" {
		t.Fatalf("expected fund scale to be updated, got %s", fund.FundScale)
	}
}
