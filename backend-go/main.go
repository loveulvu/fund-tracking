package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client
var fundCollection *mongo.Collection

type Fund struct {
	FundCode string `json:"fund_code" bson:"fund_code"`
	FundName string `json:"fund_name" bson:"fund_name"`

	NetValue         float64  `json:"net_value" bson:"net_value"`
	DayGrowth        float64  `json:"day_growth" bson:"day_growth"`
	WeekGrowth       *float64 `json:"week_growth" bson:"week_growth"`
	MonthGrowth      *float64 `json:"month_growth" bson:"month_growth"`
	ThreeMonthGrowth *float64 `json:"three_month_growth" bson:"three_month_growth"`
	SixMonthGrowth   *float64 `json:"six_month_growth" bson:"six_month_growth"`
	YearGrowth       *float64 `json:"year_growth" bson:"year_growth"`
	ThreeYearGrowth  *float64 `json:"three_year_growth" bson:"three_year_growth"`

	FundType     string `json:"fund_type" bson:"fund_type"`
	FundCompany  string `json:"fund_company" bson:"fund_company"`
	FundManager  string `json:"fund_manager" bson:"fund_manager"`
	FundScale    string `json:"fund_scale" bson:"fund_scale"`
	NetValueDate string `json:"net_value_date" bson:"net_value_date"`

	UpdateTime any `json:"update_time" bson:"update_time"`

	IsSeed    bool `json:"is_seed" bson:"is_seed"`
	IsWatched bool `json:"is_watched" bson:"is_watched"`
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Update-Key")
}
func initMongo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return err
	}
	mongoClient = client
	fundCollection = client.Database("fund_tracking").Collection("fund_data")
	return nil
}
func getFundCollection() *mongo.Collection {
	return fundCollection
}
func getenvDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

//	func getFundCollection(ctx context.Context) (*mongo.Client, *mongo.Collection, error) {
//		uri := os.Getenv("MONGO_URI")
//		if uri == "" {
//			uri = "mongodb://127.0.0.1:27017"
//		}
//		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
//		if err != nil {
//			return nil, nil, err
//		}
//		collection := client.Database("fund_tracking").Collection("fund_data")
//		return client, collection, nil
//	}
func findFundsByFilter(filter bson.M) ([]Fund, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := getFundCollection()
	findOptions := options.Find().SetProjection(bson.M{
		"_id": 0,
	})
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	funds := make([]Fund, 0)
	if err := cursor.All(ctx, &funds); err != nil {
		return nil, err
	}
	return funds, nil
}
func loadFundsFromMongoDB() ([]Fund, error) {
	return findFundsByFilter(bson.M{})
}
func findFundByCodeInMongoDB(code string) (Fund, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := getFundCollection()
	filter := bson.M{"fund_code": code}
	findOptions := options.FindOne().SetProjection(bson.M{
		"_id": 0,
	})
	var fund Fund
	err := collection.FindOne(ctx, filter, findOptions).Decode(&fund)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Fund{}, false, nil
		}
		return Fund{}, false, err
	}
	return fund, true, nil
}

func searchFundsInMongoDB(query string) ([]Fund, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"fund_code": bson.M{"$regex": query, "$options": "i"}},
			{"fund_name": bson.M{"$regex": query, "$options": "i"}},
		},
	}
	return findFundsByFilter(filter)
}
func searchHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "search query is required")
		return
	}
	ctx := r.Context()
	cacheKey := searchCacheKey(query)
	if redisClient != nil {
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			w.Header().Set("Content-Type", "application/json;charset=utf-8")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cached))
			return
		}
		if err != redis.Nil {
			appLogger.Warn("redis_get_failed", "key", cacheKey, "error", err)
		}

	}
	funds, err := searchFundsInMongoDB(query)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	data, err := json.Marshal(funds)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if redisClient != nil {
		if err := redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err(); err != nil {
			appLogger.Warn("redis_set_failed", "key", cacheKey, "error", err)
		}
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}
func fundsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	funds, err := loadFundsFromMongoDB()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(funds); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
}
func fundDetailHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/fund/"), "/")
	if code == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "search query is required")
		return
	}
	ctx := r.Context()
	cacheKey := "fund:detail:" + code
	if redisClient != nil {
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			w.Header().Set("Content-Type", "application/json;charset=utf-8")
			w.Header().Set("X-Cache", "HIT")
			w.Write([]byte(cached))
			return
		}
		if err != redis.Nil {
			appLogger.Warn("redis_get_failed", "key", cacheKey, "error", err)
		}

	}
	fund, ok, err := findFundByCodeInMongoDB(code)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if !ok {
		writeJSONError(w, http.StatusNotFound, "not_found", "fund not found")
		return
	}
	data, err := json.Marshal(fund)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if redisClient != nil {
		if err := redisClient.Set(ctx, cacheKey, data, 60*time.Second).Err(); err != nil {
			appLogger.Warn("redis_set_failed", "key", cacheKey, "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)

}
func versionHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	commit := os.Getenv("GIT_COMMIT")
	if commit == "" {
		commit = os.Getenv("RAILWAY_GIT_COMMIT_SHA")
	}
	if commit == "" {
		commit = os.Getenv("SOURCE_VERSION")
	}

	shortCommit := commit
	if len(shortCommit) > 7 {
		shortCommit = shortCommit[:7]
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]any{
		"service":     "fund-tracking-go-api",
		"version":     getenvDefault("APP_VERSION", "dev"),
		"commit":      shortCommit,
		"commit_full": commit,
		"built_at":    os.Getenv("APP_BUILT_AT"),
		"server_time": time.Now().Unix(),
	})
}
func mongoHealthHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if mongoClient == nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := mongoClient.Ping(ctx, nil); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "MongoDB connected",
	})
}
func main() {
	if err := initMongo(); err != nil {
		appLogger.Error("mongo_init_failed", "error", err)
		os.Exit(1)
	}
	defer mongoClient.Disconnect(context.Background())
	if err := initRedis(); err != nil {
		appLogger.Warn("redis_init_failed", "error", err)
	} else {
		appLogger.Info("redis_connected")
		defer redisClient.Close()
	}
	http.HandleFunc("/api/health/mongo", mongoHealthHandler)
	http.HandleFunc("/api/version", versionHandler)
	http.HandleFunc("/api/auth/register", registerHandler)
	http.HandleFunc("/api/auth/login", loginHandler)
	http.HandleFunc("/api/auth/verify-email-code", verifyEmailCodeHandler)
	http.HandleFunc("/api/auth/resend-email-code", resendEmailCodeHandler)
	http.HandleFunc("/api/update", updateFundsHandler)
	http.HandleFunc("/api/update/async", updateFundsAsyncHandler)
	http.HandleFunc("/api/update/tasks/", updateTaskStatusHandler)
	http.HandleFunc("/api/funds/enrich", enrichFundsHandler)
	http.HandleFunc("/api/funds/performance", performanceFundsHandler)
	http.HandleFunc("/api/funds/import", authMiddleware(importFundHandler))
	http.HandleFunc("/api/alerts/check", alertsCheckHandler)
	http.HandleFunc("/api/alerts/send", alertsSendHandler)
	http.HandleFunc("/api/auth/me", authMiddleware(meHandler))
	http.HandleFunc("/api/watchlist", authMiddleware(watchlistHandler))
	http.HandleFunc("/api/watchlist/", authMiddleware(watchlistHandler))
	http.HandleFunc("/api/funds/search", searchHandler)
	http.HandleFunc("/api/funds", fundsHandler)
	http.HandleFunc("/api/fund/", fundDetailHandler)
	http.HandleFunc("/api/search_proxy", searchHandler)
	port := os.Getenv("PORT")
	host := "127.0.0.1"
	if port == "" {
		port = "8081"
	} else {
		host = "0.0.0.0"
	}
	addr := host + ":" + port
	appLogger.Info("server_started", "addr", addr)
	err := http.ListenAndServe(addr, logHTTPMiddleware(http.DefaultServeMux))
	if err != nil {
		appLogger.Error("server_stopped", "error", err)
		os.Exit(1)
	}
}
