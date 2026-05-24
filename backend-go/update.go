package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var defaultFundCodes = []string{
	"008540",
	"012414",
	"001887",
	"005303",
	"588000",
	"161128",
	"510300",
	"161725",
	"001607",
	"004243",
}

type fundGZResponse struct {
	FundCode     string `json:"fundcode"`
	FundName     string `json:"name"`
	NetValue     string `json:"dwjz"`
	DayGrowth    string `json:"gszzl"`
	NetValueDate string `json:"jzrq"`
	UpdateTime   string `json:"gztime"`
}
type updateFundsResponse struct {
	Status       string   `json:"status"`
	Updated      int      `json:"updated"`
	Failed       []string `json:"failed"`
	Total        int      `json:"total"`
	DurationMs   int64    `json:"duration_ms"`
	TargetCodes  []string `json:"target_codes"`
	UpdatedCodes []string `json:"updated_codes"`
	FailedCodes  []string `json:"failed_codes"`
	SkippedCodes []string `json:"skipped_codes"`
}

func parseFloatOrZero(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return parsed
}
func parseFloatRequired(value string, fieldName string) (float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("%s is empty", fieldName)
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", fieldName, err)
	}
	return parsed, nil
}
func fetchFundBasicInfo(ctx context.Context, fundCode string) (Fund, error) {
	url := fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", fundCode)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Fund{}, err
	}
	req.Header.Set("User-agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return Fund{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Fund{}, fmt.Errorf("fund API returned status %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return Fund{}, err
	}
	body := strings.TrimSpace(string(bodyBytes))
	body = strings.TrimPrefix(body, "jsonpgz(")
	body = strings.TrimSuffix(body, ");")
	var data fundGZResponse
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return Fund{}, err
	}
	netValue, err := parseFloatRequired(data.NetValue, "dwjz")
	if err != nil {
		return Fund{}, err
	}
	return Fund{
		FundCode:     data.FundCode,
		FundName:     data.FundName,
		NetValue:     netValue,
		DayGrowth:    parseFloatOrZero(data.DayGrowth),
		NetValueDate: data.NetValueDate,
		UpdateTime:   data.UpdateTime,
		IsSeed:       true,
	}, nil
}
func validateFetchedFund(requestedCode string, fund Fund) error {
	fundCode := strings.TrimSpace(fund.FundCode)
	if fundCode == "" {
		return fmt.Errorf("fund_code is empty")
	}
	if fundCode != requestedCode {
		return fmt.Errorf("fund_code mismatch: requested %s, got %s", requestedCode, fundCode)
	}
	if strings.TrimSpace(fund.FundName) == "" {
		return fmt.Errorf("fund_name is empty")
	}
	if strings.TrimSpace(fund.NetValueDate) == "" {
		return fmt.Errorf("net_value_date is empty")
	}
	switch value := fund.UpdateTime.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("update_time is empty")
		}
	case nil:
		return fmt.Errorf("update_time is empty")
	default:
		if strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("update_time is empty")
		}
	}
	if fund.NetValue <= 0 {
		return fmt.Errorf("net_value must be greater than 0")
	}
	return nil
}
func upsertFundBasicInfo(ctx context.Context, fund Fund) error {

	collection := getFundCollection()
	filter := bson.M{
		"fund_code": fund.FundCode,
	}
	update := bson.M{
		"$set": bson.M{
			"fund_code":      fund.FundCode,
			"fund_name":      fund.FundName,
			"net_value":      fund.NetValue,
			"day_growth":     fund.DayGrowth,
			"net_value_date": fund.NetValueDate,
			"update_time":    fund.UpdateTime,
			"is_seed":        fund.IsSeed,
		},
	}
	_, err := collection.UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	)
	return err
}
func isValidFundCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
func appendUniqueValidCode(codes []string, skipped []string, seen map[string]bool, skippedSeen map[string]bool, rawCode string) ([]string, []string) {
	code := strings.TrimSpace(rawCode)
	if !isValidFundCode(code) {
		if !skippedSeen[code] {
			skippedSeen[code] = true
			skipped = append(skipped, code)
		}
		return codes, skipped
	}
	if seen[code] {
		return codes, skipped
	}
	seen[code] = true
	codes = append(codes, code)
	return codes, skipped
}
func findWatchlistFundCodes(ctx context.Context) ([]string, error) {
	collection := mongoClient.Database("fund_tracking").Collection("watchlists")
	values, err := collection.Distinct(ctx, "fundCode", bson.M{})
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(values))
	for _, value := range values {
		code, ok := value.(string)
		if !ok {
			code = fmt.Sprint(value)
		}
		codes = append(codes, code)
	}
	return codes, nil
}
func buildUpdateFundCodes(ctx context.Context) ([]string, []string, error) {
	codes := make([]string, 0, len(defaultFundCodes))
	skipped := make([]string, 0)
	seen := make(map[string]bool)
	skippedSeen := make(map[string]bool)

	for _, code := range defaultFundCodes {
		codes, skipped = appendUniqueValidCode(codes, skipped, seen, skippedSeen, code)
	}

	watchlistCodes, err := findWatchlistFundCodes(ctx)
	if err != nil {
		return nil, skipped, err
	}
	for _, code := range watchlistCodes {
		codes, skipped = appendUniqueValidCode(codes, skipped, seen, skippedSeen, code)
	}
	return codes, skipped, nil
}
func updateFundsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	start := time.Now()
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !requireUpdateAPIKey(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	targetCodes, skippedCodes, err := buildUpdateFundCodes(ctx)
	if err != nil {
		http.Error(w, "Failed to build update fund codes", http.StatusInternalServerError)
		return
	}
	updated := 0
	failed := make([]string, 0)
	updatedCodes := make([]string, 0, len(targetCodes))
	failedCodes := make([]string, 0)
	for _, fundCode := range targetCodes {
		fund, err := fetchFundBasicInfo(ctx, fundCode)
		if err != nil {
			failed = append(failed, fundCode+":fetch failed:"+err.Error())
			failedCodes = append(failedCodes, fundCode)
			continue
		}
		if err := validateFetchedFund(fundCode, fund); err != nil {
			failed = append(failed, fundCode+":validation failed:"+err.Error())
			failedCodes = append(failedCodes, fundCode)
			continue
		}
		if err := upsertFundBasicInfo(ctx, fund); err != nil {
			failed = append(failed, fundCode+":upsert failed:"+err.Error())
			failedCodes = append(failedCodes, fundCode)
			continue
		}
		updated++
		updatedCodes = append(updatedCodes, fundCode)
		time.Sleep(300 * time.Millisecond)
	}
	status := "success"
	if len(failed) > 0 && updated > 0 {
		status = "partial_success"
	}
	if updated == 0 && len(failed) > 0 {
		status = "failed"
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	if err := json.NewEncoder(w).Encode(updateFundsResponse{
		Status:       status,
		Updated:      updated,
		Failed:       failed,
		Total:        len(targetCodes),
		DurationMs:   time.Since(start).Milliseconds(),
		TargetCodes:  targetCodes,
		UpdatedCodes: updatedCodes,
		FailedCodes:  failedCodes,
		SkippedCodes: skippedCodes,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
func requireUpdateAPIKey(r *http.Request) bool {
	expectedKey := configuredUpdateAPIKey()
	if expectedKey == "" {
		return false
	}
	providedKey := strings.TrimSpace(r.Header.Get("X-Update-Key"))
	if providedKey == "" {
		providedKey = strings.TrimSpace(r.URL.Query().Get("key"))
	}
	return providedKey != "" && providedKey == expectedKey
}
func configuredUpdateAPIKey() string {
	return strings.TrimSpace(os.Getenv("UPDATE_API_KEY"))
}
