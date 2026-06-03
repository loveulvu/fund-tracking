package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	if err := ensureFundDailySnapshotIndexes(ctx); err != nil {
		return err
	}
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
func findFundsByFilter(parentCtx context.Context, filter bson.M) ([]Fund, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
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
func loadFundsFromMongoDB(ctx context.Context) ([]Fund, error) {
	return findFundsByFilter(ctx, bson.M{})
}
func findFundByCodeInMongoDB(parentCtx context.Context, code string) (Fund, bool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
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

func searchFundsInMongoDB(ctx context.Context, query string) ([]Fund, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"fund_code": bson.M{"$regex": query, "$options": "i"}},
			{"fund_name": bson.M{"$regex": query, "$options": "i"}},
		},
	}
	return findFundsByFilter(ctx, filter)
}
func searchGinHandler(c *gin.Context) {
	query := strings.TrimSpace(c.Query("query"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "search query is required",
		})
		return
	}
	ctx := c.Request.Context()
	cacheKey := searchCacheKey(query)

	if redisClient != nil {
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			c.Header("X-Cache", "HIT")
			c.Data(http.StatusOK, "application/json;charset=utf-8", []byte(cached))
			return
		}
		if err != redis.Nil {
			appLogger.Warn("redis_get_failed", "key", cacheKey, "error", err)
		}
	}

	funds, err := searchFundsInMongoDB(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}

	data, err := json.Marshal(funds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}

	if redisClient != nil {
		if err := redisClient.Set(ctx, cacheKey, data, 5*time.Minute).Err(); err != nil {
			appLogger.Warn("redis_set_failed", "key", cacheKey, "error", err)
		}
	}

	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, "application/json;charset=utf-8", data)
}

func fundsGinHandler(c *gin.Context) {
	funds, err := loadFundsFromMongoDB(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, funds)
}

func fundDetailGinHandler(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_request",
			"message": "fund code is required",
		})
		return
	}
	ctx := c.Request.Context()
	cacheKey := "fund:detail:" + code
	if redisClient != nil {
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil {

			c.Header("X-Cache", "HIT")
			c.Data(http.StatusOK, "application/json;charset=utf-8", []byte(cached))
			return
		}
		if err != redis.Nil {
			appLogger.Warn("redis_get_failed", "key", cacheKey, "error", err)
		}

	}
	fund, ok, err := findFundByCodeInMongoDB(ctx, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "not_found",
			"message": "fund not found",
		})
		return
	}
	data, err := json.Marshal(fund)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}
	if redisClient != nil {
		if err := redisClient.Set(ctx, cacheKey, data, 60*time.Second).Err(); err != nil {
			appLogger.Warn("redis_set_failed", "key", cacheKey, "error", err)
		}
	}

	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, "application/json;charset=utf-8", data)
}
func versionGinHandler(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"service":     "fund-tracking-go-api",
		"version":     getenvDefault("APP_VERSION", "dev"),
		"commit":      shortCommit,
		"commit_full": commit,
		"built_at":    os.Getenv("APP_BUILT_AT"),
		"server_time": time.Now().Unix(),
	})
}
func mongoHealthGinHandler(c *gin.Context) {
	if mongoClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "internal server error",
		})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	if err := mongoClient.Ping(ctx, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "MongoDB connected",
	})
}
func ginCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin == "https://www.fundtracking.online" ||
			origin == "https://fundtracking.online" ||
			origin == "http://localhost:3000" ||
			origin == "http://127.0.0.1:3000" {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Update-Key")
		c.Header("Access-Control-Allow-Credentials", "false")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
func main() {
	loadEnv()

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
	r := gin.Default()
	r.Use(ginCORSMiddleware())

	api := r.Group("/api")

	api.GET("/health/mongo", mongoHealthGinHandler)
	api.GET("/version", versionGinHandler)
	auth := api.Group("/auth")
	auth.POST("/register", registerGinHandler)
	auth.POST("/login", loginGinHandler)
	auth.POST("/verify-email-code", verifyEmailCodeGinHandler)
	auth.POST("/resend-email-code", resendEmailCodeGinHandler)
	auth.GET("/me", ginAuthMiddleware(), meGinHandler)
	api.POST("/update", updateFundsGinHandler)
	api.POST("/update/async", updateFundsAsyncGinHandler)
	api.GET("/update/tasks/:id", updateTaskStatusGinHandler)

	api.POST("/funds/enrich", enrichFundsGinHandler)
	api.POST("/funds/performance", performanceFundsGinHandler)
	api.POST("/funds/import", ginAuthMiddleware(), importFundGinHandler)

	api.GET("/alerts/check", alertsCheckGinHandler)
	api.POST("/alerts/send", alertsSendGinHandler)

	api.GET("/funds/search", searchGinHandler)
	api.GET("/funds/:code/history", fundHistoryGinHandler)
	api.GET("/funds", fundsGinHandler)
	api.GET("/fund/:code", fundDetailGinHandler)
	api.GET("/search_proxy", searchGinHandler)
	watchlist := api.Group("/watchlist")
	watchlist.Use(ginAuthMiddleware())

	watchlist.GET("", getWatchlistGinHandler)
	watchlist.POST("", addWatchlistGinHandler)
	watchlist.PUT("/:fundCode", updateWatchlistThresholdGinHandler)
	watchlist.DELETE("/:fundCode", deleteWatchlistGinHandler)
	port := os.Getenv("PORT")
	host := "127.0.0.1"
	if port == "" {
		port = "8081"
	} else {
		host = "0.0.0.0"
	}
	addr := host + ":" + port
	appLogger.Info("server_started", "addr", addr)
	err := http.ListenAndServe(addr, logHTTPMiddleware(r))
	if err != nil {
		appLogger.Error("server_stopped", "error", err)
		os.Exit(1)
	}
}
