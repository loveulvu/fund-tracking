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
	"sync"
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
type updateFundResult struct {
	Code  string
	OK    bool
	Stage string
	Err   error
}
type updateTaskStatus string

const (
	updateTaskPending updateTaskStatus = "pending"
	updateTaskRunning updateTaskStatus = "running"
	updateTaskSuccess updateTaskStatus = "success"
	updateTaskFailed  updateTaskStatus = "failed"
)

type updateTaskCreateResponse struct {
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

var updateTasks = make(map[string]*updateTask)
var updateTasksMu sync.Mutex

type updateTask struct {
	ID         string               `json:"id"`
	Status     updateTaskStatus     `json:"status"`
	StartedAt  time.Time            `json:"started_at"`
	FinishedAt *time.Time           `json:"finished_at,omitempty"`
	Response   *updateFundsResponse `json:"response,omitempty"`
	Error      string               `json:"error,omitempty"`
}
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
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

func executeFundUpdate(ctx context.Context) updateFundsResponse {
	start := time.Now()

	targetCodes, skippedCodes, err := buildUpdateFundCodes(ctx)
	if err != nil {
		return updateFundsResponse{
			Status:       "failed",
			Updated:      0,
			Failed:       []string{"build update fund codes failed: " + err.Error()},
			Total:        0,
			DurationMs:   time.Since(start).Milliseconds(),
			TargetCodes:  []string{},
			UpdatedCodes: []string{},
			FailedCodes:  []string{},
			SkippedCodes: skippedCodes,
		}
	}

	updated := 0
	failed := make([]string, 0)
	updatedCodes := make([]string, 0, len(targetCodes))
	failedCodes := make([]string, 0)

	results := runUpdateWorkers(ctx, targetCodes, 3)

	completedCodes := make(map[string]bool)
	for _, result := range results {
		completedCodes[result.Code] = true

		if !result.OK {
			failed = append(failed, result.Code+":"+result.Stage+" failed:"+result.Err.Error())
			failedCodes = append(failedCodes, result.Code)
			continue
		}

		updated++
		updatedCodes = append(updatedCodes, result.Code)
	}

	missingReason := "no result returned"
	if ctx.Err() != nil {
		missingReason = ctx.Err().Error()
	}

	for _, fundCode := range targetCodes {
		if !completedCodes[fundCode] {
			failed = append(failed, fundCode+":timeout failed:"+missingReason)
			failedCodes = append(failedCodes, fundCode)
		}
	}

	status := "success"
	if len(failed) > 0 && updated > 0 {
		status = "partial_success"
	}
	if updated == 0 && len(failed) > 0 {
		status = "failed"
	}

	return updateFundsResponse{
		Status:       status,
		Updated:      updated,
		Failed:       failed,
		Total:        len(targetCodes),
		DurationMs:   time.Since(start).Milliseconds(),
		TargetCodes:  targetCodes,
		UpdatedCodes: updatedCodes,
		FailedCodes:  failedCodes,
		SkippedCodes: skippedCodes,
	}
}
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
func updateSingleFund(ctx context.Context, fundCode string) updateFundResult {
	fund, err := fetchFundBasicInfo(ctx, fundCode)
	if err != nil {
		return updateFundResult{
			Code:  fundCode,
			OK:    false,
			Stage: "fetch",
			Err:   err,
		}
	}
	if err := validateFetchedFund(fundCode, fund); err != nil {
		return updateFundResult{
			Code:  fundCode,
			OK:    false,
			Stage: "validate",
			Err:   err,
		}
	}
	if err := upsertFundBasicInfo(ctx, fund); err != nil {
		return updateFundResult{
			Code:  fundCode,
			OK:    false,
			Stage: "upsert",
			Err:   err,
		}
	}
	return updateFundResult{
		Code: fundCode,
		OK:   true,
	}
}
func runUpdateWorkers(ctx context.Context, targetCodes []string, workerCount int) []updateFundResult {
	if workerCount <= 0 {
		workerCount = 1
	}
	if len(targetCodes) == 0 {
		return nil
	}
	if workerCount > len(targetCodes) {
		workerCount = len(targetCodes)
	}
	jobs := make(chan string, workerCount)
	results := make(chan updateFundResult, len(targetCodes))
	for i := 0; i < workerCount; i++ {
		go func() {
			for fundCode := range jobs {
				results <- updateSingleFund(ctx, fundCode)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, fundCode := range targetCodes {
			select {
			case <-ctx.Done():
				return
			case jobs <- fundCode:
			}
		}
	}()
	var updateResults []updateFundResult
	for i := 0; i < len(targetCodes); i++ {
		select {
		case <-ctx.Done():
			return updateResults
		case result := <-results:
			updateResults = append(updateResults, result)
		}
	}
	return updateResults
}
func updateFundsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if !requireUpdateAPIKey(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	response := executeFundUpdate(ctx)

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
}
func updateFundsAsyncHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return

	}
	if !requireUpdateAPIKey(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}
	taskID := fmt.Sprintf("update_%d", time.Now().UnixNano())
	task := &updateTask{
		ID:        taskID,
		Status:    updateTaskPending,
		StartedAt: time.Now(),
	}
	updateTasksMu.Lock()
	updateTasks[taskID] = task
	updateTasksMu.Unlock()
	go func() {
		updateTasksMu.Lock()
		task.Status = updateTaskRunning
		updateTasksMu.Unlock()
		ctx, cancle := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancle()
		response := executeFundUpdate(ctx)
		finishedAt := time.Now()
		updateTasksMu.Lock()
		task.Response = &response
		task.FinishedAt = &finishedAt
		if response.Status == "success" || response.Status == "partial_success" {
			task.Status = updateTaskSuccess
		} else {
			task.Status = updateTaskFailed
			task.Error = "fund update failed"
		}
		updateTasksMu.Unlock()
	}()
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(updateTaskCreateResponse{
		Status: "accepted",
		TaskID: taskID,
	})
}
func updateTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if !requireUpdateAPIKey(r) {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}
	taskID := strings.TrimPrefix(r.URL.Path, "/api/update/tasks/")
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "task_id is required")
		return
	}
	updateTasksMu.Lock()
	task, ok := updateTasks[taskID]
	updateTasksMu.Unlock()

	if !ok {
		writeJSONError(w, http.StatusNotFound, "not_found", "task not found")
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
}
func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   code,
		Message: message,
	})
}
