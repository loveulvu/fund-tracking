package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	Ready        int             `json:"ready"`
	Skipped      int             `json:"skipped"`
	Failed       int             `json:"failed"`
	ReadyItems   []alertSendItem `json:"ready_items"`
	SkippedItems []alertSendItem `json:"skipped_items"`
	FailedItems  []alertSendItem `json:"failed_items"`
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

	pendingLogs, err := findPendingAlertLogsForEmail(ctx)
	if err != nil {
		http.Error(w, "Failed to load pending alert logs", http.StatusInternalServerError)
		return
	}

	response := buildAlertSendDryRunResponse(ctx, pendingLogs, start)

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

func buildAlertSendDryRunResponse(ctx context.Context, logs []bson.M, start time.Time) alertSendResponse {
	readyItems := make([]alertSendItem, 0)
	skippedItems := make([]alertSendItem, 0)
	failedItems := make([]alertSendItem, 0)

	for _, logItem := range logs {
		item := alertSendItemFromLog(logItem)
		logID := logItem["_id"]
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
		item.Reason = "dry-run email ready"
		readyItems = append(readyItems, item)
	}

	status := "success"
	if len(failedItems) > 0 && len(readyItems)+len(skippedItems) > 0 {
		status = "partial_success"
	}
	if len(logs) > 0 && len(failedItems) == len(logs) {
		status = "failed"
	}

	return alertSendResponse{
		Status:       status,
		Selected:     len(logs),
		Ready:        len(readyItems),
		Skipped:      len(skippedItems),
		Failed:       len(failedItems),
		ReadyItems:   readyItems,
		SkippedItems: skippedItems,
		FailedItems:  failedItems,
		DurationMs:   time.Since(start).Milliseconds(),
		DryRun:       true,
	}
}

func findPendingAlertLogsForEmail(ctx context.Context) ([]bson.M, error) {
	collection := mongoClient.Database("fund_tracking").Collection("alert_logs")
	filter := bson.M{
		"status": bson.M{
			"$in": []string{"pending_email", "triggered"},
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
