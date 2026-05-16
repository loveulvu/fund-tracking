package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type performanceFundsResponse struct {
	Status              string                   `json:"status"`
	Total               int                      `json:"total"`
	Updated             int                      `json:"updated"`
	Skipped             int                      `json:"skipped"`
	Failed              int                      `json:"failed"`
	TargetCodes         []string                 `json:"target_codes"`
	UpdatedCodes        []string                 `json:"updated_codes"`
	SkippedCodes        []string                 `json:"skipped_codes"`
	FailedItems         []performanceFailedItem  `json:"failed_items"`
	SkippedItems        []performanceSkippedItem `json:"skipped_items"`
	UpdatedFieldsByCode map[string][]string      `json:"updated_fields_by_code"`
	DurationMs          int64                    `json:"duration_ms"`
}

type performanceFailedItem struct {
	FundCode string `json:"fundCode"`
	Reason   string `json:"reason"`
}

type performanceSkippedItem struct {
	FundCode string `json:"fundCode"`
	Reason   string `json:"reason"`
}

var performanceTitleToField = map[string]string{
	"Z":  "week_growth",
	"Y":  "month_growth",
	"3Y": "three_month_growth",
	"6Y": "six_month_growth",
	"1N": "year_growth",
	"3N": "three_year_growth",
}

var performanceFieldOrder = []string{
	"week_growth",
	"month_growth",
	"three_month_growth",
	"six_month_growth",
	"year_growth",
	"three_year_growth",
}

func performanceFundsHandler(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Failed to build performance fund codes", http.StatusInternalServerError)
		return
	}

	response := buildPerformanceFundsResponse(ctx, targetCodes, skippedCodes, start)

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func buildPerformanceFundsResponse(ctx context.Context, targetCodes []string, skippedCodes []string, start time.Time) performanceFundsResponse {
	updatedCodes := make([]string, 0)
	skippedItems := make([]performanceSkippedItem, 0, len(skippedCodes))
	failedItems := make([]performanceFailedItem, 0)
	updatedFieldsByCode := make(map[string][]string)

	for _, code := range skippedCodes {
		skippedItems = append(skippedItems, performanceSkippedItem{
			FundCode: code,
			Reason:   "invalid fundCode",
		})
	}

	for _, fundCode := range targetCodes {
		performance, err := fetchFundPerformance(fundCode)
		if err != nil {
			failedItems = append(failedItems, performanceFailedItem{
				FundCode: fundCode,
				Reason:   "fetch performance failed: " + err.Error(),
			})
			continue
		}

		updateFields := buildFundPerformanceUpdate(performance)
		if len(updateFields) == 0 {
			skippedCodes = append(skippedCodes, fundCode)
			skippedItems = append(skippedItems, performanceSkippedItem{
				FundCode: fundCode,
				Reason:   "no valid performance fields",
			})
			continue
		}

		matched, err := updateFundPerformance(ctx, fundCode, updateFields)
		if err != nil {
			failedItems = append(failedItems, performanceFailedItem{
				FundCode: fundCode,
				Reason:   "update performance failed: " + err.Error(),
			})
			continue
		}
		if !matched {
			skippedCodes = append(skippedCodes, fundCode)
			skippedItems = append(skippedItems, performanceSkippedItem{
				FundCode: fundCode,
				Reason:   "fund not found in fund_data",
			})
			continue
		}

		updatedCodes = append(updatedCodes, fundCode)
		updatedFieldsByCode[fundCode] = sortedPerformanceFieldNames(updateFields)
		time.Sleep(300 * time.Millisecond)
	}

	return performanceFundsResponse{
		Status:              determinePerformanceStatus(len(updatedCodes), len(failedItems)),
		Total:               len(targetCodes),
		Updated:             len(updatedCodes),
		Skipped:             len(skippedCodes),
		Failed:              len(failedItems),
		TargetCodes:         targetCodes,
		UpdatedCodes:        updatedCodes,
		SkippedCodes:        skippedCodes,
		FailedItems:         failedItems,
		SkippedItems:        skippedItems,
		UpdatedFieldsByCode: updatedFieldsByCode,
		DurationMs:          time.Since(start).Milliseconds(),
	}
}

func fetchFundPerformance(fundCode string) (map[string]float64, error) {
	url := fmt.Sprintf(
		"https://fundmobapi.eastmoney.com/FundMNewApi/FundMNPeriodIncrease?AppVersion=6.3.8&FCODE=%s&MobileKey=3EA024C2-7F22-408B-95E4-383D38160FB3&OSVersion=14.3&deviceid=3EA024C2-7F22-408B-95E4-383D38160FB3&passportid=3061335960830820&plat=Iphone&product=EFund&version=6.3.6",
		fundCode,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eastmoney API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return parseFundPerformanceResponse(body)
}

func parseFundPerformanceResponse(body []byte) (map[string]float64, error) {
	var payload struct {
		Success bool `json:"Success"`
		ErrCode any  `json:"ErrCode"`
		ErrMsg  any  `json:"ErrMsg"`
		Datas   []struct {
			Title string `json:"title"`
			Syl   any    `json:"syl"`
		} `json:"Datas"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		reason := strings.TrimSpace(fmt.Sprint(payload.ErrMsg))
		if reason == "" || reason == "<nil>" {
			reason = fmt.Sprintf("eastmoney API returned unsuccessful response: %v", payload.ErrCode)
		}
		return nil, fmt.Errorf("%s", reason)
	}

	performance := make(map[string]float64)
	for _, item := range payload.Datas {
		field, ok := performanceTitleToField[strings.TrimSpace(item.Title)]
		if !ok {
			continue
		}
		value, ok := parsePerformanceValue(item.Syl)
		if !ok {
			continue
		}
		performance[field] = value
	}
	return performance, nil
}

func parsePerformanceValue(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}

	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "--" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func buildFundPerformanceUpdate(performance map[string]float64) bson.M {
	updateFields := bson.M{}
	for _, field := range performanceFieldOrder {
		value, ok := performance[field]
		if ok {
			updateFields[field] = value
		}
	}
	if len(updateFields) > 0 {
		updateFields["performanceUpdatedAt"] = time.Now().UTC()
	}
	return updateFields
}

func updateFundPerformance(ctx context.Context, fundCode string, updateFields bson.M) (bool, error) {
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

func sortedPerformanceFieldNames(updateFields bson.M) []string {
	fields := make([]string, 0, len(updateFields))
	for _, field := range performanceFieldOrder {
		if _, ok := updateFields[field]; ok {
			fields = append(fields, field)
		}
	}
	if _, ok := updateFields["performanceUpdatedAt"]; ok {
		fields = append(fields, "performanceUpdatedAt")
	}
	return fields
}

func determinePerformanceStatus(updated int, failed int) string {
	if failed > 0 && updated == 0 {
		return "failed"
	}
	if failed > 0 && updated > 0 {
		return "partial_success"
	}
	return "success"
}
