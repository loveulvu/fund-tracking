package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WatchlistItem struct {
	UserID         string  `bson:"userId" json:"userId"`
	FundCode       string  `bson:"fundCode" json:"fundCode"`
	FundName       string  `bson:"fundName" json:"fundName"`
	AlertThreshold float64 `bson:"alertThreshold" json:"alertThreshold"`
	AddedAt        string  `bson:"addedAt" json:"addedAt"`
}

func watchlistHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := getAuthClaims(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	items, err := findWatchlistByUserID(claims.UserID)
	if err != nil {
		http.Error(w, "Failed to fetch watchlist", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(items)
}
func getWatchlistCollection(ctx context.Context) (*mongo.Client, *mongo.Collection, error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	clientOptions := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(30 * time.Second).
		SetConnectTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, nil, err
	}
	collection := client.Database("fund_tracking").Collection("watchlists")
	return client, collection, nil
}
func findWatchlistByUserID(userID string) ([]WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

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
