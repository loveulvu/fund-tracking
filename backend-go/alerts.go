package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type alertCheckResponse struct {
	Status         string               `json:"status"`
	Checked        int                  `json:"checked"`
	Triggered      int                  `json:"triggered"`
	Skipped        int                  `json:"skipped"`
	Failed         int                  `json:"failed"`
	Logged         int                  `json:"logged"`
	Duplicate      int                  `json:"duplicate"`
	TriggeredItems []alertTriggeredItem `json:"triggered_items"`
	SkippedItems   []alertSkippedItem   `json:"skipped_items"`
	FailedItems    []alertFailedItem    `json:"failed_items"`
	LoggedItems    []alertTriggeredItem `json:"logged_items"`
	DuplicateItems []alertTriggeredItem `json:"duplicate_items"`
	DurationMs     int64                `json:"duration_ms"`
}

type alertTriggeredItem struct {
	UserID         string  `json:"userId"`
	FundCode       string  `json:"fundCode"`
	FundName       string  `json:"fundName"`
	DayGrowth      float64 `json:"dayGrowth"`
	AlertThreshold float64 `json:"alertThreshold"`
	NetValueDate   string  `json:"netValueDate"`
	Reason         string  `json:"reason"`
}

type alertSkippedItem struct {
	UserID   string `json:"userId,omitempty"`
	FundCode string `json:"fundCode,omitempty"`
	FundName string `json:"fundName,omitempty"`
	Reason   string `json:"reason"`
}

type alertFailedItem struct {
	UserID   string `json:"userId,omitempty"`
	FundCode string `json:"fundCode,omitempty"`
	FundName string `json:"fundName,omitempty"`
	Reason   string `json:"reason"`
}

type alertLogDocument struct {
	UserID         string    `bson:"userId"`
	FundCode       string    `bson:"fundCode"`
	FundName       string    `bson:"fundName"`
	DayGrowth      float64   `bson:"dayGrowth"`
	AlertThreshold float64   `bson:"alertThreshold"`
	NetValueDate   string    `bson:"netValueDate"`
	Reason         string    `bson:"reason"`
	Status         string    `bson:"status"`
	CreatedAt      time.Time `bson:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"`
	Source         string    `bson:"source"`
}

type alertSendResponse struct {
	Status       string          `json:"status"`
	Selected     int             `json:"selected"`
	Sent         int             `json:"sent"`
	Failed       int             `json:"failed"`
	Skipped      int             `json:"skipped"`
	SentItems    []alertSendItem `json:"sent_items"`
	FailedItems  []alertSendItem `json:"failed_items"`
	SkippedItems []alertSendItem `json:"skipped_items"`
	DurationMs   int64           `json:"duration_ms"`
	DryRun       bool            `json:"dry_run"`
}

type alertSendItem struct {
	UserID         string  `json:"userId,omitempty"`
	FundCode       string  `json:"fundCode,omitempty"`
	FundName       string  `json:"fundName,omitempty"`
	DayGrowth      float64 `json:"dayGrowth,omitempty"`
	AlertThreshold float64 `json:"alertThreshold,omitempty"`
	NetValueDate   string  `json:"netValueDate,omitempty"`
	Reason         string  `json:"reason,omitempty"`
}

type alertSendErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	DryRun bool   `json:"dry_run"`
}

type resendConfig struct {
	APIKey   string
	From     string
	FromName string
}

type resendEmailResponse struct {
	ID string `json:"id"`
}

func alertsCheckHandler(w http.ResponseWriter, r *http.Request) {
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

	watchlistItems, err := findWatchlistItemsForAlertCheck(ctx)
	if err != nil {
		http.Error(w, "Failed to load watchlist items", http.StatusInternalServerError)
		return
	}

	response := buildAlertCheckResponse(ctx, watchlistItems, start)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func alertsSendHandler(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	alertLogs, err := findAlertLogsForEmail(ctx)
	if err != nil {
		http.Error(w, "Failed to load alert logs for email", http.StatusInternalServerError)
		return
	}

	if len(alertLogs) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(alertSendResponse{
			Status:       "success",
			Selected:     0,
			Sent:         0,
			Failed:       0,
			Skipped:      0,
			SentItems:    []alertSendItem{},
			FailedItems:  []alertSendItem{},
			SkippedItems: []alertSendItem{},
			DurationMs:   time.Since(start).Milliseconds(),
			DryRun:       false,
		})
		return
	}

	emailReadyLogs, skippedItems, failedItems := prepareAlertLogsForEmail(ctx, alertLogs)

	if len(emailReadyLogs) == 0 {
		response := alertSendResponse{
			Status:       determineAlertSendStatus(len(alertLogs), 0, len(skippedItems), len(failedItems)),
			Selected:     len(alertLogs),
			Sent:         0,
			Failed:       len(failedItems),
			Skipped:      len(skippedItems),
			SentItems:    []alertSendItem{},
			FailedItems:  failedItems,
			SkippedItems: skippedItems,
			DurationMs:   time.Since(start).Milliseconds(),
			DryRun:       false,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
		return
	}

	resendConfig, err := getResendConfig()
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(alertSendErrorResponse{
			Status: "failed",
			Error:  err.Error(),
			DryRun: false,
		})
		return
	}

	response := buildAlertSendResponse(ctx, emailReadyLogs, resendConfig, start)
	response = mergeAlertSendPreparation(response, len(alertLogs), skippedItems, failedItems)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func buildAlertCheckResponse(ctx context.Context, watchlistItems []bson.M, start time.Time) alertCheckResponse {
	triggeredItems := make([]alertTriggeredItem, 0)
	skippedItems := make([]alertSkippedItem, 0)
	failedItems := make([]alertFailedItem, 0)
	loggedItems := make([]alertTriggeredItem, 0)
	duplicateItems := make([]alertTriggeredItem, 0)
	checked := 0

	for _, item := range watchlistItems {
		userID := stringifyAlertValue(item["userId"])
		fundCode := strings.TrimSpace(stringifyAlertValue(item["fundCode"]))
		fundName := strings.TrimSpace(stringifyAlertValue(item["fundName"]))

		if !isValidFundCode(fundCode) {
			skippedItems = append(skippedItems, alertSkippedItem{
				UserID:   userID,
				FundCode: fundCode,
				FundName: fundName,
				Reason:   "invalid fundCode",
			})
			continue
		}

		alertThreshold, err := parseAlertNumber(item["alertThreshold"], "alertThreshold")
		if err != nil {
			skippedItems = append(skippedItems, alertSkippedItem{
				UserID:   userID,
				FundCode: fundCode,
				FundName: fundName,
				Reason:   err.Error(),
			})
			continue
		}
		if alertThreshold == 0 {
			skippedItems = append(skippedItems, alertSkippedItem{
				UserID:   userID,
				FundCode: fundCode,
				FundName: fundName,
				Reason:   "alertThreshold is zero",
			})
			continue
		}

		fund, found, err := findFundForAlertCheck(ctx, fundCode)
		if err != nil {
			failedItems = append(failedItems, alertFailedItem{
				UserID:   userID,
				FundCode: fundCode,
				FundName: fundName,
				Reason:   "fund lookup failed: " + err.Error(),
			})
			continue
		}
		if !found {
			failedItems = append(failedItems, alertFailedItem{
				UserID:   userID,
				FundCode: fundCode,
				FundName: fundName,
				Reason:   "fund_data not found",
			})
			continue
		}

		dayGrowth, err := parseAlertNumber(fund["day_growth"], "day_growth")
		if err != nil {
			failedItems = append(failedItems, alertFailedItem{
				UserID:   userID,
				FundCode: fundCode,
				FundName: fundName,
				Reason:   err.Error(),
			})
			continue
		}

		checked++
		if math.Abs(dayGrowth) >= math.Abs(alertThreshold) {
			if fundName == "" {
				fundName = strings.TrimSpace(stringifyAlertValue(fund["fund_name"]))
			}
			triggeredItem := alertTriggeredItem{
				UserID:         userID,
				FundCode:       fundCode,
				FundName:       fundName,
				DayGrowth:      dayGrowth,
				AlertThreshold: alertThreshold,
				NetValueDate:   stringifyAlertValue(fund["net_value_date"]),
				Reason:         "abs(day_growth) >= abs(alertThreshold)",
			}
			triggeredItems = append(triggeredItems, triggeredItem)

			logged, err := logAlertTrigger(ctx, triggeredItem)
			if err != nil {
				failedItems = append(failedItems, alertFailedItem{
					UserID:   userID,
					FundCode: fundCode,
					FundName: fundName,
					Reason:   "alert log failed: " + err.Error(),
				})
				continue
			}
			if logged {
				loggedItems = append(loggedItems, triggeredItem)
			} else {
				duplicateItems = append(duplicateItems, triggeredItem)
			}
		}
	}

	return alertCheckResponse{
		Status:         "success",
		Checked:        checked,
		Triggered:      len(triggeredItems),
		Skipped:        len(skippedItems),
		Failed:         len(failedItems),
		Logged:         len(loggedItems),
		Duplicate:      len(duplicateItems),
		TriggeredItems: triggeredItems,
		SkippedItems:   skippedItems,
		FailedItems:    failedItems,
		LoggedItems:    loggedItems,
		DuplicateItems: duplicateItems,
		DurationMs:     time.Since(start).Milliseconds(),
	}
}

func logAlertTrigger(ctx context.Context, item alertTriggeredItem) (bool, error) {
	collection := mongoClient.Database("fund_tracking").Collection("alert_logs")
	filter := bson.M{
		"userId":         item.UserID,
		"fundCode":       item.FundCode,
		"netValueDate":   item.NetValueDate,
		"alertThreshold": item.AlertThreshold,
	}

	err := collection.FindOne(ctx, filter).Err()
	if err == nil {
		return false, nil
	}
	if err != mongo.ErrNoDocuments {
		return false, err
	}

	now := time.Now().UTC()
	document := alertLogDocument{
		UserID:         item.UserID,
		FundCode:       item.FundCode,
		FundName:       item.FundName,
		DayGrowth:      item.DayGrowth,
		AlertThreshold: item.AlertThreshold,
		NetValueDate:   item.NetValueDate,
		Reason:         item.Reason,
		Status:         "pending_email",
		CreatedAt:      now,
		UpdatedAt:      now,
		Source:         "alerts_check",
	}

	if _, err := collection.InsertOne(ctx, document); err != nil {
		return false, err
	}
	return true, nil
}

func prepareAlertLogsForEmail(ctx context.Context, logs []bson.M) ([]bson.M, []alertSendItem, []alertSendItem) {
	emailReadyLogs := make([]bson.M, 0, len(logs))
	skippedItems := make([]alertSendItem, 0)
	failedItems := make([]alertSendItem, 0)

	for _, logItem := range logs {
		item := alertSendItemFromLog(logItem)
		logID := logItem["_id"]
		status := stringifyAlertValue(logItem["status"])

		if status == "email_ready" {
			if strings.TrimSpace(stringifyAlertValue(logItem["email"])) == "" {
				if err := markAlertLogSkippedNoEmail(ctx, logID); err != nil {
					item.Reason = "status update failed: " + err.Error()
					failedItems = append(failedItems, item)
					continue
				}
				item.Reason = "missing alert log email"
				skippedItems = append(skippedItems, item)
				continue
			}
			emailReadyLogs = append(emailReadyLogs, logItem)
			continue
		}

		email, found, err := findUserEmailForAlert(ctx, item.UserID)
		if err != nil {
			item.Reason = "user lookup failed: " + err.Error()
			failedItems = append(failedItems, item)
			continue
		}
		if !found || email == "" {
			if err := markAlertLogSkippedNoEmail(ctx, logID); err != nil {
				item.Reason = "status update failed: " + err.Error()
				failedItems = append(failedItems, item)
				continue
			}
			item.Reason = "missing user email"
			skippedItems = append(skippedItems, item)
			continue
		}
		if err := markAlertLogEmailReady(ctx, logID, email); err != nil {
			item.Reason = "status update failed: " + err.Error()
			failedItems = append(failedItems, item)
			continue
		}
		logItem["email"] = email
		logItem["status"] = "email_ready"
		emailReadyLogs = append(emailReadyLogs, logItem)
	}

	return emailReadyLogs, skippedItems, failedItems
}

func buildAlertSendResponse(ctx context.Context, logs []bson.M, config resendConfig, start time.Time) alertSendResponse {
	sentItems := make([]alertSendItem, 0)
	failedItems := make([]alertSendItem, 0)
	skippedItems := make([]alertSendItem, 0)

	for _, logItem := range logs {
		item := alertSendItemFromLog(logItem)
		logID := logItem["_id"]
		email := strings.TrimSpace(stringifyAlertValue(logItem["email"]))
		if email == "" {
			if err := markAlertLogSkippedNoEmail(ctx, logID); err != nil {
				item.Reason = "status update failed: " + err.Error()
				failedItems = append(failedItems, item)
				continue
			}
			item.Reason = "missing alert log email"
			skippedItems = append(skippedItems, item)
			continue
		}

		providerMessageID, err := sendAlertEmailWithResend(ctx, config, email, item)
		if err != nil {
			lastError := compactAlertError(err)
			if updateErr := markAlertLogEmailFailed(ctx, logID, lastError); updateErr != nil {
				item.Reason = "status update failed: " + updateErr.Error()
			} else {
				item.Reason = lastError
			}
			failedItems = append(failedItems, item)
			continue
		}

		if err := markAlertLogEmailSent(ctx, logID, providerMessageID); err != nil {
			item.Reason = "status update failed: " + err.Error()
			failedItems = append(failedItems, item)
			continue
		}
		item.Reason = "email sent"
		sentItems = append(sentItems, item)
	}

	return alertSendResponse{
		Status:       determineAlertSendStatus(len(logs), len(sentItems), len(skippedItems), len(failedItems)),
		Selected:     len(logs),
		Sent:         len(sentItems),
		Failed:       len(failedItems),
		Skipped:      len(skippedItems),
		SentItems:    sentItems,
		FailedItems:  failedItems,
		SkippedItems: skippedItems,
		DurationMs:   time.Since(start).Milliseconds(),
		DryRun:       false,
	}
}

func mergeAlertSendPreparation(response alertSendResponse, selected int, skippedItems []alertSendItem, failedItems []alertSendItem) alertSendResponse {
	response.Selected = selected
	response.SkippedItems = append(skippedItems, response.SkippedItems...)
	response.FailedItems = append(failedItems, response.FailedItems...)
	response.Skipped = len(response.SkippedItems)
	response.Failed = len(response.FailedItems)
	response.Status = determineAlertSendStatus(response.Selected, response.Sent, response.Skipped, response.Failed)
	return response
}

func determineAlertSendStatus(selected int, sent int, skipped int, failed int) string {
	if selected > 0 && failed == selected {
		return "failed"
	}
	if failed > 0 {
		return "partial_success"
	}
	return "success"
}

func findAlertLogsForEmail(ctx context.Context) ([]bson.M, error) {
	collection := mongoClient.Database("fund_tracking").Collection("alert_logs")
	filter := bson.M{
		"status": bson.M{
			"$in": []string{"pending_email", "triggered", "email_ready"},
		},
	}
	findOptions := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	items := make([]bson.M, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func getResendConfig() (resendConfig, error) {
	config := resendConfig{
		APIKey:   strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		From:     strings.TrimSpace(os.Getenv("ALERT_EMAIL_FROM")),
		FromName: strings.TrimSpace(os.Getenv("ALERT_EMAIL_FROM_NAME")),
	}
	missing := make([]string, 0)
	if config.APIKey == "" {
		missing = append(missing, "RESEND_API_KEY")
	}
	if config.From == "" {
		missing = append(missing, "ALERT_EMAIL_FROM")
	}
	if len(missing) > 0 {
		return resendConfig{}, fmt.Errorf("missing required email environment variables: %s", strings.Join(missing, ", "))
	}
	return config, nil
}

func sendAlertEmailWithResend(ctx context.Context, config resendConfig, recipient string, item alertSendItem) (string, error) {
	payload := bson.M{
		"from":    formatAlertEmailFrom(config),
		"to":      []string{recipient},
		"subject": buildAlertEmailSubject(item),
		"html":    buildAlertEmailHTML(item),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("resend returned HTTP %d", resp.StatusCode)
	}

	var result resendEmailResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode resend response")
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", fmt.Errorf("resend response missing id")
	}
	return result.ID, nil
}

func formatAlertEmailFrom(config resendConfig) string {
	if config.FromName == "" {
		return config.From
	}
	return fmt.Sprintf("%s <%s>", config.FromName, config.From)
}

func buildAlertEmailSubject(item alertSendItem) string {
	return fmt.Sprintf("基金提醒：%s 触发 %.2f%% 阈值", item.FundName, item.AlertThreshold)
}

func buildAlertEmailHTML(item alertSendItem) string {
	return fmt.Sprintf(`
<div style="font-family: Arial, sans-serif; max-width: 640px; margin: 0 auto; padding: 20px;">
  <h2 style="color: #1f2937;">基金阈值提醒</h2>
  <p>你关注的基金已触发提醒条件。</p>
  <table style="width: 100%%; border-collapse: collapse;">
    <tr><td style="padding: 8px; color: #6b7280;">基金名称</td><td style="padding: 8px;">%s</td></tr>
    <tr><td style="padding: 8px; color: #6b7280;">基金代码</td><td style="padding: 8px;">%s</td></tr>
    <tr><td style="padding: 8px; color: #6b7280;">日涨跌幅</td><td style="padding: 8px;">%.2f%%</td></tr>
    <tr><td style="padding: 8px; color: #6b7280;">提醒阈值</td><td style="padding: 8px;">%.2f%%</td></tr>
    <tr><td style="padding: 8px; color: #6b7280;">净值日期</td><td style="padding: 8px;">%s</td></tr>
    <tr><td style="padding: 8px; color: #6b7280;">触发原因</td><td style="padding: 8px;">%s</td></tr>
  </table>
</div>`,
		html.EscapeString(item.FundName),
		html.EscapeString(item.FundCode),
		item.DayGrowth,
		item.AlertThreshold,
		html.EscapeString(item.NetValueDate),
		html.EscapeString(item.Reason),
	)
}

func findUserEmailForAlert(ctx context.Context, userID string) (string, bool, error) {
	collection := mongoClient.Database("fund_tracking").Collection("users")
	filters := make([]bson.M, 0, 2)
	if objectID, err := primitive.ObjectIDFromHex(userID); err == nil {
		filters = append(filters, bson.M{"_id": objectID})
	}
	filters = append(filters, bson.M{"_id": userID})

	findOptions := options.FindOne().SetProjection(bson.M{"email": 1})
	for _, filter := range filters {
		var user bson.M
		err := collection.FindOne(ctx, filter, findOptions).Decode(&user)
		if err == nil {
			email := strings.TrimSpace(stringifyAlertValue(user["email"]))
			return email, email != "", nil
		}
		if err != mongo.ErrNoDocuments {
			return "", false, err
		}
	}
	return "", false, nil
}

func markAlertLogEmailReady(ctx context.Context, logID any, email string) error {
	now := time.Now().UTC()
	return updateAlertLogEmailStatus(ctx, logID, bson.M{
		"status":        "email_ready",
		"email":         email,
		"updatedAt":     now,
		"lastAttemptAt": now,
		"lastError":     "",
	})
}

func markAlertLogSkippedNoEmail(ctx context.Context, logID any) error {
	now := time.Now().UTC()
	return updateAlertLogEmailStatus(ctx, logID, bson.M{
		"status":        "skipped_no_email",
		"updatedAt":     now,
		"lastAttemptAt": now,
		"lastError":     "missing user email",
	})
}

func updateAlertLogEmailStatus(ctx context.Context, logID any, setFields bson.M) error {
	if logID == nil {
		return fmt.Errorf("missing alert log id")
	}
	collection := mongoClient.Database("fund_tracking").Collection("alert_logs")
	result, err := collection.UpdateOne(ctx, bson.M{"_id": logID}, bson.M{
		"$set": setFields,
		"$inc": bson.M{"retryCount": 1},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("alert log not found")
	}
	return nil
}

func markAlertLogEmailSent(ctx context.Context, logID any, providerMessageID string) error {
	now := time.Now().UTC()
	return updateAlertLogEmailStatusWithStatus(ctx, logID, "email_ready", bson.M{
		"status":            "email_sent",
		"sentAt":            now,
		"updatedAt":         now,
		"lastAttemptAt":     now,
		"lastError":         "",
		"emailProvider":     "resend",
		"providerMessageId": providerMessageID,
	})
}

func markAlertLogEmailFailed(ctx context.Context, logID any, lastError string) error {
	now := time.Now().UTC()
	return updateAlertLogEmailStatusWithStatus(ctx, logID, "email_ready", bson.M{
		"status":        "email_failed",
		"updatedAt":     now,
		"lastAttemptAt": now,
		"lastError":     lastError,
	})
}

func updateAlertLogEmailStatusWithStatus(ctx context.Context, logID any, currentStatus string, setFields bson.M) error {
	if logID == nil {
		return fmt.Errorf("missing alert log id")
	}
	collection := mongoClient.Database("fund_tracking").Collection("alert_logs")
	result, err := collection.UpdateOne(ctx, bson.M{"_id": logID, "status": currentStatus}, bson.M{
		"$set": setFields,
		"$inc": bson.M{"retryCount": 1},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("alert log not found or status changed")
	}
	return nil
}

func compactAlertError(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "email send failed"
	}
	if len(message) > 180 {
		message = message[:180]
	}
	return message
}

func requireUpdateAPIKeyHeader(r *http.Request) bool {
	expectedKey := os.Getenv("UPDATE_API_KEY")
	if expectedKey == "" {
		return true
	}
	return r.Header.Get("X-Update-Key") == expectedKey
}

func findWatchlistItemsForAlertCheck(ctx context.Context) ([]bson.M, error) {
	collection := mongoClient.Database("fund_tracking").Collection("watchlists")
	findOptions := options.Find().SetProjection(bson.M{
		"_id":            0,
		"userId":         1,
		"fundCode":       1,
		"fundName":       1,
		"alertThreshold": 1,
	})

	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	items := make([]bson.M, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func findFundForAlertCheck(ctx context.Context, fundCode string) (bson.M, bool, error) {
	collection := getFundCollection()
	findOptions := options.FindOne().SetProjection(bson.M{
		"_id":            0,
		"fund_code":      1,
		"fund_name":      1,
		"day_growth":     1,
		"net_value_date": 1,
	})

	var fund bson.M
	err := collection.FindOne(ctx, bson.M{"fund_code": fundCode}, findOptions).Decode(&fund)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, false, nil
		}
		return nil, false, err
	}
	return fund, true, nil
}

func parseAlertNumber(value any, fieldName string) (float64, error) {
	var parsed float64
	var err error

	switch typed := value.(type) {
	case nil:
		return 0, fmt.Errorf("%s is empty", fieldName)
	case float64:
		parsed = typed
	case float32:
		parsed = float64(typed)
	case int:
		parsed = float64(typed)
	case int32:
		parsed = float64(typed)
	case int64:
		parsed = float64(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("%s is empty", fieldName)
		}
		parsed, err = strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, fmt.Errorf("%s is not numeric", fieldName)
		}
	case primitive.Decimal128:
		parsed, err = strconv.ParseFloat(typed.String(), 64)
		if err != nil {
			return 0, fmt.Errorf("%s is not numeric", fieldName)
		}
	default:
		parsed, err = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		if err != nil {
			return 0, fmt.Errorf("%s is not numeric", fieldName)
		}
	}

	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("%s is not finite", fieldName)
	}
	return parsed, nil
}

func alertSendItemFromLog(logItem bson.M) alertSendItem {
	dayGrowth, _ := parseAlertNumber(logItem["dayGrowth"], "dayGrowth")
	alertThreshold, _ := parseAlertNumber(logItem["alertThreshold"], "alertThreshold")
	return alertSendItem{
		UserID:         stringifyAlertValue(logItem["userId"]),
		FundCode:       stringifyAlertValue(logItem["fundCode"]),
		FundName:       stringifyAlertValue(logItem["fundName"]),
		DayGrowth:      dayGrowth,
		AlertThreshold: alertThreshold,
		NetValueDate:   stringifyAlertValue(logItem["netValueDate"]),
	}
}

func stringifyAlertValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case primitive.ObjectID:
		return typed.Hex()
	case time.Time:
		return typed.Format(time.RFC3339)
	default:
		return fmt.Sprint(value)
	}
}
