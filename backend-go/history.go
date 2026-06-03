package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const fundHistoryRetentionDays = 30

type FundDailySnapshot struct {
	FundCode     string    `json:"fund_code" bson:"fund_code"`
	FundName     string    `json:"fund_name" bson:"fund_name"`
	NetValue     float64   `json:"net_value" bson:"net_value"`
	DayGrowth    float64   `json:"day_growth" bson:"day_growth"`
	NetValueDate string    `json:"net_value_date" bson:"net_value_date"`
	UpdateTime   any       `json:"update_time" bson:"update_time"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
}

type FundHistoryPoint struct {
	Date      string  `json:"date" bson:"net_value_date"`
	NetValue  float64 `json:"net_value" bson:"net_value"`
	DayGrowth float64 `json:"day_growth" bson:"day_growth"`
}

func getFundDailySnapshotCollection() *mongo.Collection {
	return mongoClient.Database("fund_tracking").Collection("fund_daily_snapshots")
}

func ensureFundDailySnapshotIndexes(ctx context.Context) error {
	collection := getFundDailySnapshotCollection()
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "fund_code", Value: 1},
			{Key: "net_value_date", Value: 1},
		},
		Options: options.Index().SetName("uniq_fund_code_net_value_date").SetUnique(true),
	})
	return err
}

func upsertFundDailySnapshot(parentCtx context.Context, fund Fund) error {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	now := time.Now().UTC()
	collection := getFundDailySnapshotCollection()
	filter := bson.M{
		"fund_code":      fund.FundCode,
		"net_value_date": fund.NetValueDate,
	}
	update := bson.M{
		"$set": bson.M{
			"fund_code":      fund.FundCode,
			"fund_name":      fund.FundName,
			"net_value":      fund.NetValue,
			"day_growth":     fund.DayGrowth,
			"net_value_date": fund.NetValueDate,
			"update_time":    fund.UpdateTime,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
		},
	}

	_, err := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func cleanupOldFundDailySnapshots(parentCtx context.Context) error {
	ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
	defer cancel()

	cutoffDate := time.Now().UTC().AddDate(0, 0, -fundHistoryRetentionDays).Format("2006-01-02")
	_, err := getFundDailySnapshotCollection().DeleteMany(ctx, bson.M{
		"net_value_date": bson.M{"$lt": cutoffDate},
	})
	return err
}

func findFundHistory(parentCtx context.Context, fundCode string, rangeValue string) ([]FundHistoryPoint, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	days := 7
	if rangeValue == "30d" {
		days = 30
	}

	cutoffDate := time.Now().UTC().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	findOptions := options.Find().
		SetProjection(bson.M{
			"_id":            0,
			"net_value_date": 1,
			"net_value":      1,
			"day_growth":     1,
		}).
		SetSort(bson.D{{Key: "net_value_date", Value: 1}})

	cursor, err := getFundDailySnapshotCollection().Find(ctx, bson.M{
		"fund_code":      fundCode,
		"net_value_date": bson.M{"$gte": cutoffDate},
	}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	points := make([]FundHistoryPoint, 0)
	if err := cursor.All(ctx, &points); err != nil {
		return nil, err
	}
	return points, nil
}

func findPurchaseSnapshot(parentCtx context.Context, fundCode string, purchaseDate string) (FundDailySnapshot, bool, error) {
	if snapshot, found, err := findPurchaseSnapshotWithFilter(parentCtx, fundCode, bson.M{"$lte": purchaseDate}, -1); err != nil || found {
		return snapshot, found, err
	}
	return findPurchaseSnapshotWithFilter(parentCtx, fundCode, bson.M{"$gte": purchaseDate}, 1)
}

func findPurchaseSnapshotWithFilter(parentCtx context.Context, fundCode string, dateFilter bson.M, sortDirection int) (FundDailySnapshot, bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	filter := bson.M{
		"fund_code":      fundCode,
		"net_value_date": dateFilter,
	}
	findOptions := options.FindOne().
		SetProjection(bson.M{"_id": 0}).
		SetSort(bson.D{{Key: "net_value_date", Value: sortDirection}})

	var snapshot FundDailySnapshot
	err := getFundDailySnapshotCollection().FindOne(ctx, filter, findOptions).Decode(&snapshot)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return FundDailySnapshot{}, false, nil
		}
		return FundDailySnapshot{}, false, err
	}
	return snapshot, true, nil
}

func fundHistoryGinHandler(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	rangeValue := strings.TrimSpace(c.DefaultQuery("range", "7d"))

	if !isValidFundCode(code) {
		Fail(c, http.StatusBadRequest, "invalid_request", "fund code must be a 6-digit code")
		return
	}
	if rangeValue != "7d" && rangeValue != "30d" {
		Fail(c, http.StatusBadRequest, "invalid_range", "range must be 7d or 30d")
		return
	}

	points, err := findFundHistory(c.Request.Context(), code, rangeValue)
	if err != nil {
		appLogger.Error("fund_history_query_failed", "fund_code", code, "range", rangeValue, "error", err)
		Fail(c, http.StatusInternalServerError, "internal_error", "failed to load fund history")
		return
	}

	c.JSON(http.StatusOK, points)
}
