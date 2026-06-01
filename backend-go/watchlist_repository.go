package main

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getWatchlistCollection() *mongo.Collection {
	return mongoClient.Database("fund_tracking").Collection("watchlists")
}

func findWatchlistByUserID(parentCtx context.Context, userID string) ([]WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	collection := getWatchlistCollection()

	filter := bson.M{"userId": userID}

	findOptions := options.Find().SetProjection(bson.M{
		"_id": 0,
	})

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	items := make([]WatchlistItem, 0)
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

func insertWatchlistItem(parentCtx context.Context, item WatchlistItem) (WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	collection := getWatchlistCollection()
	filter := bson.M{
		"userId":   item.UserID,
		"fundCode": item.FundCode,
	}

	err := collection.FindOne(ctx, filter).Err()
	if err == nil {
		return WatchlistItem{}, ErrWatchlistExists
	}
	if err != mongo.ErrNoDocuments {
		return WatchlistItem{}, err
	}

	_, err = collection.InsertOne(ctx, item)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return WatchlistItem{}, ErrWatchlistExists
		}
		return WatchlistItem{}, err
	}

	return item, nil
}

func deleteWatchlistItem(parentCtx context.Context, userID string, fundCode string) (bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	collection := getWatchlistCollection()
	filter := bson.M{
		"userId":   userID,
		"fundCode": fundCode,
	}

	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}

	return result.DeletedCount > 0, nil
}

func updateWatchlistThreshold(parentCtx context.Context, userID string, fundCode string, alertThreshold float64) (WatchlistItem, bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	collection := getWatchlistCollection()

	filter := bson.M{
		"userId":   userID,
		"fundCode": fundCode,
	}

	update := bson.M{
		"$set": bson.M{
			"alertThreshold": alertThreshold,
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return WatchlistItem{}, false, err
	}

	if result.MatchedCount == 0 {
		return WatchlistItem{}, false, nil
	}

	findOptions := options.FindOne().SetProjection(bson.M{
		"_id": 0,
	})

	var updatedItem WatchlistItem
	if err := collection.FindOne(ctx, filter, findOptions).Decode(&updatedItem); err != nil {
		return WatchlistItem{}, false, err
	}

	return updatedItem, true, nil
}
