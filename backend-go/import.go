package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type importFundRequest struct {
	FundCode string `json:"fundCode"`
}

type importFundResponse struct {
	Status         string   `json:"status"`
	Result         string   `json:"result"`
	FundCode       string   `json:"fundCode"`
	Fund           Fund     `json:"fund"`
	MetadataFields []string `json:"metadata_fields"`
	Warnings       []string `json:"warnings"`
}

func importFundGinHandler(c *gin.Context) {
	if _, ok := getGinAuthClaims(c); !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req importFundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "Invalid request body")
		return
	}

	fundCode := strings.TrimSpace(req.FundCode)
	if !isValidFundCode(fundCode) {
		c.String(http.StatusBadRequest, "fundCode must be a 6-digit code")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	response, statusCode, err := importFundByCode(ctx, fundCode)
	if err != nil {
		c.String(statusCode, err.Error())
		return
	}

	c.JSON(statusCode, response)
}

func importFundByCode(ctx context.Context, fundCode string) (importFundResponse, int, error) {
	existingFund, ok, err := findFundByCodeInMongoDB(fundCode)
	if err != nil {
		return importFundResponse{}, http.StatusInternalServerError, err
	}
	if ok {
		return buildExistingImportResponse(existingFund), http.StatusOK, nil
	}

	fund, err := fetchFundBasicInfo(ctx, fundCode)
	if err != nil {
		return importFundResponse{}, http.StatusBadGateway, err
	}
	if err := validateFetchedFund(fundCode, fund); err != nil {
		return importFundResponse{}, http.StatusBadGateway, err
	}

	fund.IsSeed = false
	if err := upsertFundBasicInfo(ctx, fund); err != nil {
		return importFundResponse{}, http.StatusInternalServerError, err
	}

	status := "success"
	warnings := make([]string, 0)
	metadataFields := make([]string, 0)

	metadata, err := fetchFundMetadata(fundCode)
	if err != nil {
		status = "partial_success"
		warnings = append(warnings, "metadata enrichment failed: "+err.Error())
	} else {
		updateFields := buildFundMetadataUpdate(metadata)
		if len(updateFields) == 0 {
			warnings = append(warnings, "no valid metadata fields")
		} else {
			matched, err := updateFundMetadata(ctx, fundCode, updateFields)
			if err != nil {
				status = "partial_success"
				warnings = append(warnings, "metadata update failed: "+err.Error())
			} else if !matched {
				status = "partial_success"
				warnings = append(warnings, "metadata update skipped: fund not found in fund_data")
			} else {
				metadataFields = sortedMetadataFieldNames(updateFields)
				applyMetadataToFund(&fund, metadata)
			}
		}
	}

	return importFundResponse{
		Status:         status,
		Result:         "imported",
		FundCode:       fundCode,
		Fund:           fund,
		MetadataFields: metadataFields,
		Warnings:       warnings,
	}, http.StatusCreated, nil
}

func buildExistingImportResponse(fund Fund) importFundResponse {
	return importFundResponse{
		Status:         "success",
		Result:         "existing",
		FundCode:       fund.FundCode,
		Fund:           fund,
		MetadataFields: []string{},
		Warnings:       []string{},
	}
}

func applyMetadataToFund(fund *Fund, metadata fundMetadata) {
	if value := cleanMetadataValue(metadata.FundType); value != "" {
		fund.FundType = value
	}
	if value := cleanMetadataValue(metadata.FundCompany); value != "" {
		fund.FundCompany = value
	}
	if value := cleanMetadataValue(metadata.FundManager); value != "" {
		fund.FundManager = value
	}
	if value := cleanMetadataValue(metadata.FundScale); value != "" {
		fund.FundScale = value
	}
}

// =========================
// Legacy net/http handlers kept for Gin refactor review.
// Remove before merging gin-refactor.
// =========================
// func importFundHandler(w http.ResponseWriter, r *http.Request) {
// 	enableCORS(w)
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}
//
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
//
// 	if _, ok := getAuthClaims(r); !ok {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}
//
// 	var req importFundRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}
//
// 	fundCode := strings.TrimSpace(req.FundCode)
// 	if !isValidFundCode(fundCode) {
// 		http.Error(w, "fundCode must be a 6-digit code", http.StatusBadRequest)
// 		return
// 	}
//
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()
//
// 	response, statusCode, err := importFundByCode(ctx, fundCode)
// 	if err != nil {
// 		http.Error(w, err.Error(), statusCode)
// 		return
// 	}
//
// 	w.Header().Set("Content-Type", "application/json;charset=utf-8")
// 	w.WriteHeader(statusCode)
// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 		return
// 	}
// }
