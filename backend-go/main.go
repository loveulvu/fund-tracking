package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client
var fundCollection *mongo.Collection

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

type Fund struct {
	FundCode string `json:"fund_code" bson:"fund_code"`
	FundName string `json:"fund_name" bson:"fund_name"`

	NetValue         float64 `json:"net_value" bson:"net_value"`
	DayGrowth        float64 `json:"day_growth" bson:"day_growth"`
	WeekGrowth       float64 `json:"week_growth" bson:"week_growth"`
	MonthGrowth      float64 `json:"month_growth" bson:"month_growth"`
	ThreeMonthGrowth float64 `json:"three_month_growth" bson:"three_month_growth"`
	SixMonthGrowth   float64 `json:"six_month_growth" bson:"six_month_growth"`
	YearGrowth       float64 `json:"year_growth" bson:"year_growth"`
	ThreeYearGrowth  float64 `json:"three_year_growth" bson:"three_year_growth"`

	FundType     string `json:"fund_type" bson:"fund_type"`
	FundCompany  string `json:"fund_company" bson:"fund_company"`
	FundManager  string `json:"fund_manager" bson:"fund_manager"`
	FundScale    string `json:"fund_scale" bson:"fund_scale"`
	NetValueDate string `json:"net_value_date" bson:"net_value_date"`

	UpdateTime any `json:"update_time" bson:"update_time"`

	IsSeed    bool `json:"is_seed" bson:"is_seed"`
	IsWatched bool `json:"is_watched" bson:"is_watched"`
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
func fetchFundBasicInfo(fundCode string) (Fund, error) {
	url := fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", fundCode)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
func upsertFundBasicInfo(fund Fund) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
func updateFundsHandler(w http.ResponseWriter, r *http.Request) {
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
	targetCodes, skippedCodes, err := buildUpdateFundCodes(ctx)
	if err != nil {
		http.Error(w, "Failed to build update fund codes", http.StatusInternalServerError)
		return
	}
	updated := 0
	failed := make([]string, 0)
	updatedCodes := make([]string, 0, len(targetCodes))
	failedCodes := make([]string, 0)
	for _, fundCode := range targetCodes {
		fund, err := fetchFundBasicInfo(fundCode)
		if err != nil {
			failed = append(failed, fundCode+":fetch failed:"+err.Error())
			failedCodes = append(failedCodes, fundCode)
			continue
		}
		if err := validateFetchedFund(fundCode, fund); err != nil {
			failed = append(failed, fundCode+":validation failed:"+err.Error())
			failedCodes = append(failedCodes, fundCode)
			continue
		}
		if err := upsertFundBasicInfo(fund); err != nil {
			failed = append(failed, fundCode+":upsert failed:"+err.Error())
			failedCodes = append(failedCodes, fundCode)
			continue
		}
		updated++
		updatedCodes = append(updatedCodes, fundCode)
		time.Sleep(300 * time.Millisecond)
	}
	status := "success"
	if len(failed) > 0 && updated > 0 {
		status = "partial_success"
	}
	if updated == 0 && len(failed) > 0 {
		status = "failed"
	}

	w.Header().Set("Content-Type", "application/json;charset=utf-8")

	if err := json.NewEncoder(w).Encode(updateFundsResponse{
		Status:       status,
		Updated:      updated,
		Failed:       failed,
		Total:        len(targetCodes),
		DurationMs:   time.Since(start).Milliseconds(),
		TargetCodes:  targetCodes,
		UpdatedCodes: updatedCodes,
		FailedCodes:  failedCodes,
		SkippedCodes: skippedCodes,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func requireUpdateAPIKey(r *http.Request) bool {
	expectedKey := os.Getenv("UPDATE_API_KEY")
	if expectedKey == "" {
		return true
	}
	providedKey := r.Header.Get("X-Update-Key")
	if providedKey == "" {
		providedKey = r.URL.Query().Get("key")
	}
	return providedKey == expectedKey
}
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}
	funds, err := searchFundsInMongoDB(query)
	if err != nil {
		http.Error(w, "Failed to search funds", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(funds); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
func fundsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	funds, err := loadFundsFromMongoDB()
	if err != nil {
		http.Error(w, "Failed to load funds", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(funds); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	code := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/fund/"), "/")
	if code == "" {
		http.Error(w, "Fund code is required", http.StatusBadRequest)
		return
	}
	fund, ok, err := findFundByCodeInMongoDB(code)
	if err != nil {
		http.Error(w, "Failed to find fund", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Fund not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	if err := json.NewEncoder(w).Encode(fund); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

}
func versionHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if mongoClient == nil {
		http.Error(w, "MongoDB client not initialized", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := mongoClient.Ping(ctx, nil); err != nil {
		http.Error(w, "Failed to ping MongoDB: "+err.Error(), http.StatusInternalServerError)
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
		log.Fatal(err)
	}
	defer mongoClient.Disconnect(context.Background())
	http.HandleFunc("/api/health/mongo", mongoHealthHandler)
	http.HandleFunc("/api/version", versionHandler)
	http.HandleFunc("/api/auth/register", registerHandler)
	http.HandleFunc("/api/auth/login", loginHandler)
	http.HandleFunc("/api/update", updateFundsHandler)
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
	log.Println("Server is running on http://" + addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}
