package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
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
	TriggeredItems []alertTriggeredItem `json:"triggered_items"`
	SkippedItems   []alertSkippedItem   `json:"skipped_items"`
	FailedItems    []alertFailedItem    `json:"failed_items"`
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

func buildAlertCheckResponse(ctx context.Context, watchlistItems []bson.M, start time.Time) alertCheckResponse {
	triggeredItems := make([]alertTriggeredItem, 0)
	skippedItems := make([]alertSkippedItem, 0)
	failedItems := make([]alertFailedItem, 0)
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
			triggeredItems = append(triggeredItems, alertTriggeredItem{
				UserID:         userID,
				FundCode:       fundCode,
				FundName:       fundName,
				DayGrowth:      dayGrowth,
				AlertThreshold: alertThreshold,
				NetValueDate:   stringifyAlertValue(fund["net_value_date"]),
				Reason:         "abs(day_growth) >= abs(alertThreshold)",
			})
		}
	}

	return alertCheckResponse{
		Status:         "success",
		Checked:        checked,
		Triggered:      len(triggeredItems),
		Skipped:        len(skippedItems),
		Failed:         len(failedItems),
		TriggeredItems: triggeredItems,
		SkippedItems:   skippedItems,
		FailedItems:    failedItems,
		DurationMs:     time.Since(start).Milliseconds(),
	}
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
