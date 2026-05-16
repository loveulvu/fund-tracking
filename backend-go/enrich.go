package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type enrichFundsResponse struct {
	Status              string              `json:"status"`
	Total               int                 `json:"total"`
	Enriched            int                 `json:"enriched"`
	Skipped             int                 `json:"skipped"`
	Failed              int                 `json:"failed"`
	TargetCodes         []string            `json:"target_codes"`
	EnrichedCodes       []string            `json:"enriched_codes"`
	SkippedCodes        []string            `json:"skipped_codes"`
	SkippedItems        []enrichSkippedItem `json:"skipped_items"`
	FailedItems         []enrichFailedItem  `json:"failed_items"`
	UpdatedFieldsByCode map[string][]string `json:"updated_fields_by_code"`
	DurationMs          int64               `json:"duration_ms"`
}

type enrichFailedItem struct {
	FundCode string `json:"fundCode"`
	Reason   string `json:"reason"`
}

type enrichSkippedItem struct {
	FundCode string `json:"fundCode"`
	Reason   string `json:"reason"`
}

type fundMetadata struct {
	FundType    string
	FundCompany string
	FundManager string
	FundScale   string
}

func enrichFundsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	start := time.Now()

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !requireUpdateAPIKeyHeader(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	targetCodes, skippedCodes, err := buildUpdateFundCodes(ctx)
	if err != nil {
		http.Error(w, "Failed to build enrich fund codes", http.StatusInternalServerError)
		return
	}

	response := buildEnrichFundsResponse(ctx, targetCodes, skippedCodes, start)

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func buildEnrichFundsResponse(ctx context.Context, targetCodes []string, skippedCodes []string, start time.Time) enrichFundsResponse {
	enrichedCodes := make([]string, 0)
	skippedItems := make([]enrichSkippedItem, 0, len(skippedCodes))
	failedItems := make([]enrichFailedItem, 0)
	updatedFieldsByCode := make(map[string][]string)

	for _, code := range skippedCodes {
		skippedItems = append(skippedItems, enrichSkippedItem{
			FundCode: code,
			Reason:   "invalid fundCode",
		})
	}

	for _, fundCode := range targetCodes {
		metadata, err := fetchFundMetadata(fundCode)
		if err != nil {
			failedItems = append(failedItems, enrichFailedItem{
				FundCode: fundCode,
				Reason:   "fetch metadata failed: " + err.Error(),
			})
			continue
		}

		updateFields := buildFundMetadataUpdate(metadata)
		if len(updateFields) == 0 {
			skippedCodes = append(skippedCodes, fundCode)
			skippedItems = append(skippedItems, enrichSkippedItem{
				FundCode: fundCode,
				Reason:   "no valid metadata fields",
			})
			continue
		}

		matched, err := updateFundMetadata(ctx, fundCode, updateFields)
		if err != nil {
			failedItems = append(failedItems, enrichFailedItem{
				FundCode: fundCode,
				Reason:   "update metadata failed: " + err.Error(),
			})
			continue
		}
		if !matched {
			skippedCodes = append(skippedCodes, fundCode)
			skippedItems = append(skippedItems, enrichSkippedItem{
				FundCode: fundCode,
				Reason:   "fund not found in fund_data",
			})
			continue
		}

		enrichedCodes = append(enrichedCodes, fundCode)
		updatedFieldsByCode[fundCode] = sortedMetadataFieldNames(updateFields)
		time.Sleep(300 * time.Millisecond)
	}

	status := "success"
	if len(failedItems) > 0 && len(enrichedCodes) > 0 {
		status = "partial_success"
	}
	if len(enrichedCodes) == 0 && len(failedItems) > 0 {
		status = "failed"
	}

	return enrichFundsResponse{
		Status:              status,
		Total:               len(targetCodes),
		Enriched:            len(enrichedCodes),
		Skipped:             len(skippedCodes),
		Failed:              len(failedItems),
		TargetCodes:         targetCodes,
		EnrichedCodes:       enrichedCodes,
		SkippedCodes:        skippedCodes,
		SkippedItems:        skippedItems,
		FailedItems:         failedItems,
		UpdatedFieldsByCode: updatedFieldsByCode,
		DurationMs:          time.Since(start).Milliseconds(),
	}
}

func fetchFundMetadata(fundCode string) (fundMetadata, error) {
	url := fmt.Sprintf(
		"http://fundmobapi.eastmoney.com/FundMNewApi/FundMNBaseInfo?FCODE=%s&deviceid=Wap&plat=Wap&product=EFund&version=2.0.0",
		fundCode,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fundMetadata{}, err
	}
	req.Header.Set("User-Agent", "Dalvik/2.1.0 (Linux; U; Android 10; SM-G981B Build/QP1A.190711.020)")
	req.Header.Set("Host", "fundmobapi.eastmoney.com")
	req.Header.Set("Connection", "Keep-Alive")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fundMetadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fundMetadata{}, fmt.Errorf("eastmoney API returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fundMetadata{}, err
	}

	var payload struct {
		Success bool            `json:"Success"`
		ErrMsg  string          `json:"ErrMsg"`
		Datas   json.RawMessage `json:"Datas"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return fundMetadata{}, err
	}
	if !payload.Success {
		reason := strings.TrimSpace(payload.ErrMsg)
		if reason == "" {
			reason = "eastmoney API returned unsuccessful response"
		}
		return fundMetadata{}, fmt.Errorf("%s", reason)
	}
	if len(payload.Datas) == 0 || string(payload.Datas) == "null" {
		return fundMetadata{}, nil
	}

	var data map[string]any
	if err := json.Unmarshal(payload.Datas, &data); err != nil {
		return fundMetadata{}, err
	}

	return fundMetadata{
		FundType:    cleanMetadataValue(data["FTYPE"]),
		FundCompany: cleanMetadataValue(data["JJGS"]),
		FundManager: cleanMetadataValue(data["JJJL"]),
		FundScale:   cleanMetadataValue(data["TOTALSCALE"]),
	}, nil
}

func cleanMetadataValue(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "0" {
		return ""
	}
	return text
}

func buildFundMetadataUpdate(metadata fundMetadata) bson.M {
	updateFields := bson.M{}
	if value := cleanMetadataValue(metadata.FundType); value != "" {
		updateFields["fund_type"] = value
	}
	if value := cleanMetadataValue(metadata.FundCompany); value != "" {
		updateFields["fund_company"] = value
	}
	if value := cleanMetadataValue(metadata.FundManager); value != "" {
		updateFields["fund_manager"] = value
	}
	if value := cleanMetadataValue(metadata.FundScale); value != "" {
		updateFields["fund_scale"] = value
	}
	return updateFields
}

func updateFundMetadata(ctx context.Context, fundCode string, updateFields bson.M) (bool, error) {
	collection := getFundCollection()
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"fund_code": fundCode},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		return false, err
	}
	return result.MatchedCount > 0, nil
}

func sortedMetadataFieldNames(updateFields bson.M) []string {
	fieldOrder := []string{"fund_type", "fund_company", "fund_manager", "fund_scale"}
	fields := make([]string, 0, len(updateFields))
	for _, field := range fieldOrder {
		if _, ok := updateFields[field]; ok {
			fields = append(fields, field)
		}
	}
	return fields
}
