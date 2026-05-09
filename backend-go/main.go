package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
func loadFundsFromFile() ([]Fund, error) {
	data, err := os.ReadFile("data/funds.json")
	if err != nil {
		return nil, err
	}
	var funds []Fund
	if err := json.Unmarshal(data, &funds); err != nil {
		return nil, err
	}
	return funds, nil
}
func getFundCollection(ctx context.Context) (*mongo.Client, *mongo.Collection, error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, err
	}
	collection := client.Database("fund_tracking").Collection("fund_data")
	return client, collection, nil
}
func loadFundsFromMongoDB() ([]Fund, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, collection, err := getFundCollection(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)
	filter := bson.M{}
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
func findFundByCodeInMongoDB(code string) (Fund, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, collection, err := getFundCollection(ctx)
	if err != nil {
		return Fund{}, false, err
	}
	defer client.Disconnect(ctx)
	filter := bson.M{"fund_code": code}
	findOptions := options.FindOne().SetProjection(bson.M{
		"_id": 0,
	})
	var fund Fund
	err = collection.FindOne(ctx, filter, findOptions).Decode(&fund)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Fund{}, false, nil
		}
		return Fund{}, false, err
	}
	return fund, true, nil
}
func findFundByCode(code string) (Fund, bool, error) {
	funds, err := loadFundsFromFile()
	if err != nil {
		return Fund{}, false, err
	}
	for _, fund := range funds {
		if fund.FundCode == code {
			return fund, true, nil
		}
	}
	return Fund{}, false, nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := getFundCollection(ctx)
	if err != nil {
		http.Error(w, "Failed to connect MongoDB: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(ctx)
	if err := client.Ping(ctx, nil); err != nil {
		http.Error(w, "Failed to ping MongoDB:"+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "MongoDB connected",
	})
}
func main() {
	http.HandleFunc("/api/funds", fundsHandler)
	http.HandleFunc("/api/fund/", fundDetailHandler)
	http.HandleFunc("/api/health/mongo", mongoHealthHandler)
	log.Println("Server is running on http://127.0.0.1:8081")
	err := http.ListenAndServe("127.0.0.1:8081", nil)
	if err != nil {
		log.Fatal(err)
	}
}
