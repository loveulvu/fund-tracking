package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WatchlistItem struct {
	UserID         string    `bson:"userId" json:"userId"`
	FundCode       string    `bson:"fundCode" json:"fundCode"`
	FundName       string    `bson:"fundName" json:"fundName"`
	AlertThreshold float64   `bson:"alertThreshold" json:"alertThreshold"`
	AddedAt        time.Time `bson:"addedAt" json:"addedAt"`
}
type AddWatchlistRequest struct {
	FundCode       string   `json:"fundCode"`
	FundName       string   `json:"fundName"`
	AlertThreshold *float64 `json:"alertThreshold"`
}

var errWatchlistExists = errors.New("watchlist item already exists")

func watchlistHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	claims, ok := getAuthClaims(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		getWatchlistHandler(w, r, claims)
	case http.MethodPost:
		addWatchlistHandler(w, r, claims)
	case http.MethodDelete:
		deleteWatchlistHandler(w, r, claims)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
func getWatchlistHandler(w http.ResponseWriter, r *http.Request, claims *AuthClaims) {
	items, err := findWatchlistByUserID(claims.UserID)
	if err != nil {
		http.Error(w, "Failed to fetch watchlist", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(items)
}
func addWatchlistHandler(w http.ResponseWriter, r *http.Request, claims *AuthClaims) {
	var req AddWatchlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.FundCode = strings.TrimSpace(req.FundCode)
	req.FundName = strings.TrimSpace(req.FundName)
	if req.FundCode == "" || req.FundName == "" {
		http.Error(w, "fundCode and fundName are required", http.StatusBadRequest)
		return
	}
	alertThreshold := 5.0
	if req.AlertThreshold != nil {
		alertThreshold = *req.AlertThreshold
	}
	item := WatchlistItem{
		UserID:         claims.UserID,
		FundCode:       req.FundCode,
		FundName:       req.FundName,
		AlertThreshold: alertThreshold,
		AddedAt:        time.Now().UTC(),
	}
	createdItem, err := insertWatchlistItem(item)
	if err != nil {
		if errors.Is(err, errWatchlistExists) {
			http.Error(w, "Fund already in watchlist", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to add to watchlist: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdItem)
}
func insertWatchlistItem(item WatchlistItem) (WatchlistItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return WatchlistItem{}, err
	}
	defer client.Disconnect(ctx)
	filter := bson.M{
		"userId":   item.UserID,
		"fundCode": item.FundCode,
	}
	err = collection.FindOne(ctx, filter).Err()
	if err == nil {
		return WatchlistItem{}, errWatchlistExists
	}
	if err != mongo.ErrNoDocuments {
		return WatchlistItem{}, err
	}
	_, err = collection.InsertOne(ctx, item)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return WatchlistItem{}, errWatchlistExists
		}
		return WatchlistItem{}, err
	}

	return item, nil
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
func deleteWatchlistHandler(w http.ResponseWriter, r *http.Request, claims *AuthClaims) {
	fundCode := strings.TrimPrefix(r.URL.Path, "/api/watchlist/")
	fundCode = strings.TrimSpace(strings.Trim(fundCode, "/"))
	if fundCode == "" {
		http.Error(w, "fundCode is required", http.StatusBadRequest)
		return
	}
	deleted, err := deleteWatchlistItem(claims.UserID, fundCode)
	if err != nil {
		http.Error(w, "Failed to delete watchlist item", http.StatusInternalServerError)
		return
	}
	if !deleted {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Watchlist item not found",
		})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Successfully removed from watchlist",
	})
}
func deleteWatchlistItem(userID string, fundCode string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, collection, err := getWatchlistCollection(ctx)
	if err != nil {
		return false, err
	}
	defer client.Disconnect(ctx)
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
